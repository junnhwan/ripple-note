package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	listenAddr := flag.String("listen", ":80", "HTTP listen address")
	webRoot := flag.String("web", "/app/web/dist", "frontend dist directory")
	apiBase := flag.String("api", "http://api:8080", "API upstream base URL")
	flag.Parse()

	upstream, err := url.Parse(*apiBase)
	if err != nil {
		log.Fatal("parse api upstream: ", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstream)
	mux := http.NewServeMux()
	mux.Handle("/api/", proxy)
	mux.Handle("/uploads/", proxy)
	mux.Handle("/", spaHandler{root: *webRoot})

	server := &http.Server{
		Addr:              *listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("web server listening on %s, web=%s, api=%s", *listenAddr, *webRoot, upstream.String())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("web server stopped: ", err)
	}
}

type spaHandler struct {
	root string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cleanPath := path.Clean("/" + r.URL.Path)
	requested := filepath.Join(h.root, strings.TrimPrefix(cleanPath, "/"))

	if info, err := os.Stat(requested); err == nil && !info.IsDir() {
		http.ServeFile(w, r, requested)
		return
	}

	http.ServeFile(w, r, filepath.Join(h.root, "index.html"))
}
