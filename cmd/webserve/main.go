// Command webserve serves the browser build of the port (tools/web/build.sh)
// together with the original game files, for a private host: the game files
// are never part of the build or the repository.
//
//	/           the build: index.html, the scripts, openorion2.wasm
//	/game/NAME  a game file (range requests work, for music and videos)
//	/game.json  the list of game files: name, size, mtime (ms)
//	/net        network play: a WebSocket per browser player, relayed to
//	            the game's TCP relay (docs/net/web.md)
package main

import (
	"crypto/subtle"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/openorion2/openorion2/internal/netplay"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "address to listen on")
	root := flag.String("web", "build/web", "directory with the browser build")
	game := flag.String("game", "game", "directory with the original game files")
	auth := flag.String("auth", os.Getenv("OPENORION2_AUTH"), "user:password for HTTP basic auth (default from OPENORION2_AUTH; empty: none)")
	netPlay := flag.Bool("net", true, "serve /net: browsers create network games here (their relays listen on the TCP port the creator gives) and join any relay this server can reach")
	maxGames := flag.Int("netgames", 8, "the most network games created from browsers at once")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle("/", noCache(gzipped(*root, http.FileServer(http.Dir(*root)))))
	mux.Handle("/game/", http.StripPrefix("/game/", http.FileServer(gameFiles{http.Dir(*game)})))
	mux.HandleFunc("/game.json", func(w http.ResponseWriter, r *http.Request) {
		list, err := manifest(*game)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		json.NewEncoder(w).Encode(list)
	})
	if *netPlay {
		mux.Handle("/net", &netplay.WebRelay{MaxRelays: *maxGames, Log: log.Default()})
	}
	var h http.Handler = mux
	if *auth != "" {
		h = basicAuth(*auth, h)
	}
	log.Printf("serving %s and the game files in %s on http://%s/", *root, *game, *addr)
	log.Fatal(http.ListenAndServe(*addr, h))
}

type gameFile struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
}

// manifest lists the regular files of the game directory.
func manifest(dir string) ([]gameFile, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	list := []gameFile{}
	for _, e := range ents {
		if !e.Type().IsRegular() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			return nil, err
		}
		list = append(list, gameFile{e.Name(), fi.Size(), fi.ModTime().UnixMilli()})
	}
	return list, nil
}

// gameFiles serves files only, no directory listings or dot files.
type gameFiles struct{ fs http.FileSystem }

func (g gameFiles) Open(name string) (http.File, error) {
	if strings.Count(name, "/") > 1 || strings.HasPrefix(filepath.Base(name), ".") {
		return nil, os.ErrNotExist
	}
	f, err := g.fs.Open(name)
	if err != nil {
		return nil, err
	}
	if fi, err := f.Stat(); err != nil || fi.IsDir() {
		f.Close()
		return nil, os.ErrNotExist
	}
	return f, nil
}

// gzipped serves NAME.gz for NAME when it exists and the client takes gzip
// (the build compresses openorion2.wasm).
func gzipped(root string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wasm") && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			gz := filepath.Join(root, filepath.FromSlash(filepath.Clean("/"+r.URL.Path))) + ".gz"
			if _, err := os.Stat(gz); err == nil {
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("Content-Type", "application/wasm")
				w.Header().Add("Vary", "Accept-Encoding")
				http.ServeFile(w, r, gz)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// noCache makes browsers revalidate the build, so a rebuild is picked up.
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func basicAuth(cred string, next http.Handler) http.Handler {
	user, pass, _ := strings.Cut(cred, ":")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 || subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="openorion2"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
