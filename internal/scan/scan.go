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
	"cinexplorer/internal/store"
)

var ErrBusy = errors.New("ya hay un escaneo en curso")

type Status struct {
	Running   bool      `json:"running"`
	Files     int64     `json:"files"`
	Hashed    int64     `json:"hashed"`
	Versions  int       `json:"versions"`
	LastError string    `json:"lastError"`
	Finished  time.Time `json:"finished"`
}

type Scanner struct {
	AppDir string
	Roots  []string // catalog form
	Store  *store.Store

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
	var scanned []string

	for _, root := range s.Roots {
		abs := appdir.Abs(s.AppDir, root)
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			log.Printf("raíz no disponible, se omite: %s", root)
			continue
		}
		scanned = append(scanned, root)
		err := filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
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
				log.Printf("no se puede leer %s: %v", p, err)
				return nil
			}
			rel, err := appdir.Rel(s.AppDir, p)
			if err != nil {
				return err
			}
			row := store.FileRow{Path: rel, Size: info.Size(), MTime: info.ModTime().UnixMilli(), Kind: string(kind)}
			hashed := int64(0)
			if k, ok := known[rel]; ok && k.Size == row.Size && k.MTime == row.MTime && (k.Fingerprint != "" || !kind.Fingerprinted()) {
				row.Fingerprint = k.Fingerprint
			} else if kind.Fingerprinted() {
				fp, err := fingerprint.Of(p)
				if err != nil {
					log.Printf("no se puede leer %s: %v", p, err)
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
	}

	if err := s.Store.SyncFiles(seen, scanned); err != nil {
		return err
	}
	versions := grouping.Build(entries, scanned)
	if err := s.Store.ReplaceVersions(versions); err != nil {
		return err
	}
	s.mu.Lock()
	s.status.Versions = len(versions)
	s.mu.Unlock()
	return nil
}

func (s *Scanner) add(files, hashed int64) {
	s.mu.Lock()
	s.status.Files += files
	s.status.Hashed += hashed
	s.mu.Unlock()
}
