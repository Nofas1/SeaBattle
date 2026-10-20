package main

import (
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

//go:embed index.html css/style.css dist\bundle.js
var staticFiles embed.FS

func main() {
	apiURL := envOrDefault("SEABATTLE_API_URL", "http://127.0.0.1:28080")
	target, err := url.Parse(apiURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		log.Fatal("SEABATTLE_API_URL must be an absolute URL")
	}

	address := envOrDefault("FRONTEND_ADDR", "127.0.0.1:3000")
	staticFS, err := fs.Sub(staticFiles, ".")
	if err != nil {
		log.Fatal(err)
	}

	apiProxy := httputil.NewSingleHostReverseProxy(target)
	apiProxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		log.Printf("API proxy request failed: %v", err)
		http.Error(w, "API proxy unavailable", http.StatusBadGateway)
	}

	mux := http.NewServeMux()
	mux.Handle("POST /login", apiProxy)
	mux.Handle("POST /register", apiProxy)
	mux.Handle("GET /verify", apiProxy)
	mux.Handle("POST /bot", apiProxy)
	mux.Handle("GET /", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, staticFS, "index.html")
	})

	log.Printf("SeaBattle frontend listening on http://%s (API: %s)", address, strings.TrimRight(apiURL, "/"))
	if err := http.ListenAndServe(address, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
