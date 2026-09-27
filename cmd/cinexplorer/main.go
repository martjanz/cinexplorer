// Command cinexplorer serves a local, read-only catalog of a movie library.
package main

import (
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
	"cinexplorer/internal/engine"
	"cinexplorer/internal/platform"
	"cinexplorer/internal/server"
	"cinexplorer/internal/store"
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
	srv, closeStore, err := setup(dirOverride)
	if err != nil {
		return err
	}
	defer closeStore()

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

// setup opens the catalog in the app directory and wires the server around
// an engine built from config.json. Without config.json (a first start) and
// with a writable directory, nothing is saved or scanned until the first-use
// assistant saves the settings; otherwise the scan starts right away.
// closeStore stops the engine and closes the catalog.
func setup(dirOverride string) (srv *server.Server, closeStore func() error, err error) {
	appDir, err := appdir.Resolve(dirOverride)
	if err != nil {
		return nil, nil, err
	}
	readOnly := !appdir.Writable(appDir)
	cfg, created, err := config.Load(appDir)
	if err != nil {
		return nil, nil, err
	}

	st, err := openStore(appDir, readOnly)
	if err != nil {
		return nil, nil, err
	}

	eng := &engine.Engine{AppDir: appDir, Store: st, ReadOnly: readOnly, Wikidata: wikidata.New(version)}
	pending := created && !readOnly
	eng.Start(cfg, pending)
	if readOnly {
		log.Print("el directorio de la app no es escribible: modo consulta")
	}
	if pending {
		log.Print("primer uso: elegí las carpetas y el token en el navegador")
	}
	srv = &server.Server{AppDir: appDir, Store: st, ReadOnly: readOnly, Opener: platform.Open, Revealer: platform.Reveal,
		Engine: eng}
	return srv, func() error {
		eng.Current().Stop()
		return st.Close()
	}, nil
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
