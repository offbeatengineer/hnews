package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/offbeatengineer/hnews/api"
)

//go:embed frontend/*
var frontendFS embed.FS

func main() {
	// Determine port
	port := os.Getenv("HNEWS_PORT")
	if port == "" {
		port = "8666"
	}

	// Strip prefix from embedded filesystem
	frontend, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem: %v", err)
	}

	// Set up router
	mux := http.NewServeMux()

	// API routes — handled by our API package
	mux.Handle("/api/", api.Handler())

	// Frontend assets
	cssFS, err := fs.Sub(frontend, "css")
	if err == nil {
		cssHandler := http.FileServer(http.FS(cssFS))
		mux.Handle("/css/", http.StripPrefix("/css/", cssHandler))
	}

	jsFS, err := fs.Sub(frontend, "js")
	if err == nil {
		jsHandler := http.FileServer(http.FS(jsFS))
		mux.Handle("/js/", http.StripPrefix("/js/", jsHandler))
	}

	// Serve index.html for all non-API, non-asset routes (SPA fallback)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") ||
			strings.HasPrefix(r.URL.Path, "/css/") ||
			strings.HasPrefix(r.URL.Path, "/js/") {
			return
		}
		index, err := frontendFS.ReadFile("frontend/index.html")
		if err != nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})

	// CORS headers middleware
	handler := corsMiddleware(mux)

	// Create server
	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	url := fmt.Sprintf("http://localhost:%s", port)
	fmt.Printf("HN is running at %s\n", url)

	// Open browser
	go func() {
		openBrowser(url)
	}()

	// Start server
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

// corsMiddleware adds CORS headers for local development
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// openBrowser opens the default browser to the given URL
func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("cmd", "/c", "start", url).Start()
	default: // darwin/macOS
		err = exec.Command("open", url).Start()
	}

	if err != nil {
		fmt.Printf("Tip: open %s in your browser\n", url)
	}
}
