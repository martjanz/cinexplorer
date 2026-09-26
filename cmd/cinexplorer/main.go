// Command cinexplorer serves a local, read-only catalog of a movie library.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"cinexplorer/internal/appdir"
	"cinexplorer/internal/config"
	"cinexplorer/internal/identify"
	"cinexplorer/internal/images"
	"cinexplorer/internal/platform"
	"cinexplorer/internal/scan"
	"cinexplorer/internal/server"
	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
	"cinexplorer/internal/wikidata"
)

// version identifies the build to Wikidata (User-Agent).
const version = "0.3"

func main() {
	dir := flag.String("dir", os.Getenv("CINEXPLORER_DIR"), "directorio de la app (por defecto, el del ejecutable)")
	port := flag.Int("port", 0, "puerto HTTP (0 = uno libre)")
	noBrowser := flag.Bool("no-browser", false, "no abrir el navegador")
	flag.Parse()
	if err := run(*dir, *port, !*noBrowser); err != nil {
		log.Fatal(err)
	}
}

func run(dirOverride string, port int, browser bool) error {
	appDir, err := appdir.Resolve(dirOverride)
	if err != nil {
		return err
	}
	readOnly := !appdir.Writable(appDir)
	cfg, created, err := config.Load(appDir)
	if err != nil {
		return err
	}
	if created && !readOnly {
		if err := config.Save(appDir, cfg); err != nil {
			return err
		}
		log.Printf("config.json creado con raíces %v", cfg.Roots)
	}

	st, err := openStore(appDir, readOnly)
	if err != nil {
		return err
	}
	defer st.Close()

	srv := &server.Server{AppDir: appDir, Roots: cfg.Roots, Store: st, ReadOnly: readOnly, Opener: platform.Open, Revealer: platform.Reveal,
		Language: cfg.Language, Images: &images.Cache{Dir: filepath.Join(appDir, "cache"), ReadOnly: readOnly}}
	var api *tmdb.Client
	if cfg.TMDBToken != "" {
		api = tmdb.New(cfg.TMDBToken)
		srv.TMDB, srv.Images.Fetch = api, api
	} else {
		log.Print("sin tmdbToken en config.json: no se identifican películas")
	}
	if readOnly {
		log.Print("el directorio de la app no es escribible: modo consulta")
	} else {
		runner := &identify.Runner{AppDir: appDir, Store: st, Wikidata: wikidata.New(version), Images: srv.Images,
			Language: cfg.Language, Prefetch: cfg.ImagePrefetch}
		if api != nil {
			runner.TMDB = api // only when set: a nil *tmdb.Client in the interface would not read as "no token"
		}
		srv.Identifier = runner
		srv.Scanner = &scan.Scanner{AppDir: appDir, Roots: cfg.Roots, Store: st, OnDone: runner.Trigger}
		go func() {
			if err := srv.Scanner.Run(context.Background()); err != nil {
				log.Printf("escaneo: %v", err)
			}
		}()
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/"
	log.Printf("Cinexplorer en %s (Ctrl+C para salir)", url)
	if browser {
		if err := platform.Open(url); err != nil {
			log.Printf("no se pudo abrir el navegador: %v", err)
		}
	}
	return http.Serve(ln, srv.Handler())
}

// openStore opens the catalog. In read-only mode with no catalog file yet it
// falls back to an empty in-memory one.
func openStore(appDir string, readOnly bool) (*store.Store, error) {
	path := filepath.Join(appDir, "cinexplorer.db")
	if readOnly {
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			return store.OpenMemory()
		}
	}
	return store.Open(path, readOnly)
}
