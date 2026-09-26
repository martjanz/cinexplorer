// Package scan walks the library roots and refreshes the catalog. It only
// reads the library; the only writes go to the catalog database.
package scan

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/fingerprint"
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/mediafile"
	"cinexplorer/internal/probe"
	"cinexplorer/internal/store"
)

var ErrBusy = errors.New("ya hay un escaneo en curso")

type Status struct {
	Running   bool      `json:"running"`
	Files     int64     `json:"files"`
	Hashed    int64     `json:"hashed"`
	Versions  int       `json:"versions"`
	ToProbe   int       `json:"toProbe"` // files whose headers this run reads
	Probed    int       `json:"probed"`
	LastError string    `json:"lastError"`
	Finished  time.Time `json:"finished"`
}

type Scanner struct {
	AppDir string
	Roots  []string // catalog form
	Store  *store.Store
	// Probe reads a file's technical data; nil means probe.Probe.
	Probe func(ctx context.Context, path string) (probe.Info, error)

	mu     sync.Mutex
	status Status
}

func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Run performs one incremental scan. A call while another is running returns ErrBusy.
func (s *Scanner) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		return ErrBusy
	}
	s.status = Status{Running: true}
	s.mu.Unlock()

	err := s.run(ctx)

	s.mu.Lock()
	s.status.Running = false
	s.status.Finished = time.Now()
	if err != nil {
		s.status.LastError = err.Error()
	}
	s.mu.Unlock()
	return err
}

func (s *Scanner) run(ctx context.Context) error {
	known, err := s.Store.FileIndex()
	if err != nil {
		return err
	}
	var seen []store.FileRow
	var entries []grouping.Entry
	// attempted holds every root that was walked at all (used to tell grouping
	// whether a directory is a library root, e.g. for loose files). clean holds
	// only the roots that walked without any per-node I/O error and is what we
	// pass to SyncFiles, so a partially-failed root never causes files under its
	// unreadable parts to be wrongly marked missing.
	var attempted []string
	var clean []string

	for _, root := range s.Roots {
		abs := appdir.Abs(s.AppDir, root)
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			log.Printf("raíz no disponible, se omite: %s", root)
			continue
		}
		attempted = append(attempted, root)
		rootHadError := false
		err := filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				rootHadError = true
				log.Printf("no se puede leer %s: %v", p, err)
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.IsDir() {
				if p != abs && strings.HasPrefix(d.Name(), ".") {
					return fs.SkipDir
				}
				return nil
			}
			kind := mediafile.Classify(d.Name())
			if !kind.Stored() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				rootHadError = true
				log.Printf("no se puede leer %s: %v", p, err)
				return nil
			}
			rel, err := appdir.Rel(s.AppDir, p)
			if err != nil {
				return err
			}
			row := store.FileRow{Path: rel, Size: info.Size(), MTime: info.ModTime().UnixMilli(), Kind: string(kind)}
			known_, wasKnown := known[rel]
			hashed := int64(0)
			if wasKnown && known_.Size == row.Size && known_.MTime == row.MTime && (known_.Fingerprint != "" || !kind.Fingerprinted()) {
				row.Fingerprint = known_.Fingerprint
			} else if kind.Fingerprinted() {
				fp, err := fingerprint.Of(p)
				if err != nil {
					// The file is present but unreadable/corrupt right now
					// (locked, flaky drive, truncated mid-write, etc). Don't
					// let a transient read failure make it look deleted:
					// keep its previous known state if we have one, or
					// register it with no fingerprint so a later scan can
					// retry, but either way do not drop it from this scan's
					// results.
					log.Printf("no se puede calcular fingerprint de %s, se conserva el estado anterior: %v", p, err)
					if wasKnown {
						row = known_
					} else {
						row.Fingerprint = ""
					}
					seen = append(seen, row)
					entries = append(entries, grouping.Entry{Path: rel, Size: row.Size, Kind: kind})
					s.add(1, 0)
					return nil
				}
				row.Fingerprint = fp
				hashed = 1
			}
			seen = append(seen, row)
			entries = append(entries, grouping.Entry{Path: rel, Size: row.Size, Kind: kind})
			s.add(1, hashed)
			return nil
		})
		if err != nil {
			return err
		}
		// A root that vanished mid-walk means the disk was unplugged: keep the
		// previous catalog instead of marking everything as missing.
		if _, err := os.Stat(abs); err != nil {
			return fmt.Errorf("la raíz %s desapareció durante el escaneo", root)
		}
		if rootHadError {
			// Some part of this root could not be read (flaky drive, stale
			// network mount, permission hiccup). We keep whatever we did
			// manage to see, but we must not let SyncFiles treat this root as
			// fully scanned: that would mark every known file under the
			// unreadable part as missing.
			log.Printf("raíz escaneada parcialmente por errores de lectura, no se marcarán archivos ausentes bajo ella: %s", root)
			continue
		}
		clean = append(clean, root)
	}

	if err := s.Store.SyncFiles(seen, clean); err != nil {
		return err
	}
	versions := grouping.Build(entries, attempted)
	if err := s.Store.ReplaceVersions(versions); err != nil {
		return err
	}
	s.mu.Lock()
	s.status.Versions = len(versions)
	s.mu.Unlock()
	// The catalog is already updated here: an error from the probe phase only
	// means some files keep their previous (or no) technical data.
	return s.probeAll(ctx, attempted)
}

// probeBatch is how many probe results are committed at once, so an
// interrupted run keeps most of its work.
const probeBatch = 50

// probeAll reads the headers of the files that are new or changed since they
// were last probed, under the roots that were available in this scan. Read
// failures are left for the next scan; format errors are stored so the file
// is not read again until it changes. A root that disappears stops the phase.
func (s *Scanner) probeAll(ctx context.Context, roots []string) error {
	pending, err := s.Store.PendingProbes()
	if err != nil {
		return err
	}
	var targets []store.ProbeTarget
	for _, t := range pending {
		if rootOf(t.Path, roots) != "" {
			targets = append(targets, t)
		}
	}
	s.mu.Lock()
	s.status.ToProbe = len(targets)
	s.mu.Unlock()
	read := s.Probe
	if read == nil {
		read = probe.Probe
	}

	var batch []store.ProbeResult
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := s.Store.SaveProbes(batch)
		batch = batch[:0]
		return err
	}
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return errors.Join(err, flush())
		}
		info, err := read(ctx, appdir.Abs(s.AppDir, t.Path))
		if err != nil && ctx.Err() == nil {
			// The failure may only mean the drive went away: then nothing is
			// known about this file, and the scan stops like the walk does.
			root := rootOf(t.Path, roots)
			if _, statErr := os.Stat(appdir.Abs(s.AppDir, root)); statErr != nil {
				return errors.Join(fmt.Errorf("la raíz %s desapareció durante el análisis", root), flush())
			}
		}
		r := store.ProbeResult{FileID: t.FileID, Size: t.Size, MTime: t.MTime, Info: info}
		switch {
		case err == nil:
			batch = append(batch, r)
		case ctx.Err() != nil:
			return errors.Join(ctx.Err(), flush())
		case errors.Is(err, probe.ErrIO):
			log.Printf("no se puede leer %s, se reintentará: %v", t.Path, err)
		default:
			r.Err = err.Error()
			batch = append(batch, r)
		}
		s.mu.Lock()
		s.status.Probed++
		s.mu.Unlock()
		if len(batch) >= probeBatch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}

// rootOf returns the root (catalog form) that contains p, or "".
func rootOf(p string, roots []string) string {
	for _, r := range roots {
		if strings.HasPrefix(p, strings.TrimSuffix(r, "/")+"/") {
			return r
		}
	}
	return ""
}

func (s *Scanner) add(files, hashed int64) {
	s.mu.Lock()
	s.status.Files += files
	s.status.Hashed += hashed
	s.mu.Unlock()
}
