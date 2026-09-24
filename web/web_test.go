package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIndex_VersionsAssets(t *testing.T) {
	page, err := Index()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(page, []byte(assetVersionPlaceholder)) {
		t.Error("placeholder was not replaced")
	}
	if !bytes.Contains(page, []byte("/static/app.css?v=")) {
		t.Error("expected versioned stylesheet link")
	}
}

func TestStaticHandler(t *testing.T) {
	tests := []struct {
		path       string
		wantStatus int
		wantCache  string
		wantType   string
	}{
		{"/static/app.css?v=abc", http.StatusOK, "immutable", "text/css"},
		{"/static/app.js", http.StatusOK, "max-age=300", "javascript"},
		{"/static/fonts/inter-latin-wght-normal.woff2", http.StatusOK, "immutable", "font/woff2"},
		{"/static/", http.StatusNotFound, "", ""},
		{"/static/fonts/", http.StatusNotFound, "", ""},
		{"/static/missing.css", http.StatusNotFound, "", ""},
	}
	h := StaticHandler()
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantCache != "" && !strings.Contains(rec.Header().Get("Cache-Control"), tt.wantCache) {
				t.Errorf("Cache-Control = %q, want %q", rec.Header().Get("Cache-Control"), tt.wantCache)
			}
			if tt.wantType != "" && !strings.Contains(rec.Header().Get("Content-Type"), tt.wantType) {
				t.Errorf("Content-Type = %q, want %q", rec.Header().Get("Content-Type"), tt.wantType)
			}
		})
	}
}
