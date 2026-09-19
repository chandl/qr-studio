// Package httpapi exposes the qr package over HTTP: GET/POST /qr.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/chandl/qrcode/internal/qr"
)

// maxGETDataLen is a practical ceiling on URL length; browsers and proxies
// commonly cap full URLs around ~2000 characters, so longer payloads should
// use POST /qr instead. This is enforced only for GET, since POST bodies
// have no such constraint.
const maxGETDataLen = 2000

// maxPOSTBodyBytes bounds the JSON request body for POST /qr.
const maxPOSTBodyBytes = 1 << 20 // 1MiB

// cacheImmutableYear is applied to GET /qr responses: output is fully
// deterministic per parameter set, so it is safe to cache forever.
const cacheImmutableYear = "public, max-age=31536000, immutable"

func NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /qr", handleGetQR)
	mux.HandleFunc("POST /qr", handlePostQR)
	mux.HandleFunc("GET /healthz", handleHealthz)
	return mux
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func handleGetQR(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	data := q.Get("data")
	if len(data) > maxGETDataLen {
		writeError(w, http.StatusBadRequest,
			"data exceeds the %d character limit for GET requests; use POST /qr for longer payloads", maxGETDataLen)
		return
	}

	opts, err := optionsFromQuery(q)
	if err != nil {
		writeGenErr(w, err)
		return
	}

	result, err := qr.Generate(opts)
	if err != nil {
		writeGenErr(w, err)
		return
	}

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Cache-Control", cacheImmutableYear)
	if result.ContrastWarned {
		w.Header().Set("X-QR-Contrast-Warning", "foreground/background colors are low contrast and may not scan reliably")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Bytes)
}

// postQRRequest mirrors the GET query params for JSON bodies.
type postQRRequest struct {
	Data    string `json:"data"`
	Format  string `json:"format"`
	Size    int    `json:"size"`
	ECL     string `json:"ecl"`
	Color   string `json:"color"`
	BgColor string `json:"bgcolor"`
	Shape   string `json:"shape"`
}

func handlePostQR(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxPOSTBodyBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read request body")
		return
	}
	if len(body) > maxPOSTBodyBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds %d bytes", maxPOSTBodyBytes)
		return
	}

	var req postQRRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: %v", err)
		return
	}

	opts := qr.Options{
		Data:    req.Data,
		Format:  qr.Format(req.Format),
		Size:    req.Size,
		ECL:     qr.ECL(req.ECL),
		Color:   req.Color,
		BgColor: req.BgColor,
		Shape:   qr.Shape(req.Shape),
	}

	result, err := qr.Generate(opts)
	if err != nil {
		writeGenErr(w, err)
		return
	}

	w.Header().Set("Content-Type", result.ContentType)
	if result.ContrastWarned {
		w.Header().Set("X-QR-Contrast-Warning", "foreground/background colors are low contrast and may not scan reliably")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Bytes)
}

func optionsFromQuery(q url.Values) (qr.Options, error) {
	get := q.Get

	opts := qr.Options{
		Data:    get("data"),
		Format:  qr.Format(get("format")),
		ECL:     qr.ECL(get("ecl")),
		Color:   get("color"),
		BgColor: get("bgcolor"),
		Shape:   qr.Shape(get("shape")),
	}

	if s := get("size"); s != "" {
		size, err := strconv.Atoi(s)
		if err != nil {
			return opts, &qr.ValidationError{Msg: "size must be an integer"}
		}
		opts.Size = size
	}

	return opts, nil
}

func writeGenErr(w http.ResponseWriter, err error) {
	var ve *qr.ValidationError
	if errors.As(err, &ve) {
		writeError(w, http.StatusBadRequest, "%s", ve.Msg)
		return
	}
	log.Printf("qr generate: %v", err)
	writeError(w, http.StatusInternalServerError, "internal error generating QR code")
}

func writeError(w http.ResponseWriter, status int, format string, args ...any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": fmt.Sprintf(format, args...),
	})
}
