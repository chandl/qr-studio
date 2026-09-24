// Package web embeds the static demo UI.
package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed index.html static
var files embed.FS

// assetVersionPlaceholder is replaced in index.html with a content hash of
// the static directory, so /static/ URLs change whenever an asset does.
const assetVersionPlaceholder = "__ASSET_VERSION__"

const (
	cacheImmutableYear = "public, max-age=31536000, immutable"
	cacheShort         = "public, max-age=300"
)

// Index returns the contents of the demo page with asset URLs versioned.
func Index() ([]byte, error) {
	page, err := files.ReadFile("index.html")
	if err != nil {
		return nil, err
	}
	version, err := assetVersion()
	if err != nil {
		return nil, err
	}
	return bytes.ReplaceAll(page, []byte(assetVersionPlaceholder), []byte(version)), nil
}

// StaticHandler serves the embedded static directory under /static/.
// Versioned requests (?v=…) and fonts are cached for a year; anything else
// gets a short TTL so an unversioned URL never pins a stale asset.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		panic(err) // the directory is embedded at compile time
	}
	fileServer := http.StripPrefix("/static/", http.FileServerFS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r) // no directory listings
			return
		}
		if r.URL.Query().Get("v") != "" || strings.HasPrefix(r.URL.Path, "/static/fonts/") {
			w.Header().Set("Cache-Control", cacheImmutableYear)
		} else {
			w.Header().Set("Cache-Control", cacheShort)
		}
		fileServer.ServeHTTP(w, r)
	})
}

func assetVersion() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(files, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := files.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write([]byte(path))
		h.Write(b)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}
