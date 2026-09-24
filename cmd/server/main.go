// Command server runs the QR code generation HTTP service.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/chandl/qrcode/internal/httpapi"
	"github.com/chandl/qrcode/web"
)

func main() {
	addr := os.Getenv("QR_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := httpapi.NewMux()

	indexHTML, err := web.Index()
	if err != nil {
		log.Fatalf("could not load embedded web UI: %v", err)
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	mux.Handle("GET /static/", web.StaticHandler())

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("qrcode service listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
