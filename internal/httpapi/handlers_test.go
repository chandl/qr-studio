package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetQR_Success(t *testing.T) {
	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/qr?data=hello", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable", cc)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected non-empty body")
	}
}

func TestGetQR_MissingData(t *testing.T) {
	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/qr", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("could not decode error body: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

func TestGetQR_DataTooLong(t *testing.T) {
	mux := NewMux()
	longData := strings.Repeat("a", maxGETDataLen+1)
	req := httptest.NewRequest(http.MethodGet, "/qr?data="+longData, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "POST /qr") {
		t.Errorf("expected error to mention POST /qr, got: %s", rec.Body.String())
	}
}

func TestGetQR_InvalidParams(t *testing.T) {
	cases := []string{
		"/qr?data=x&format=bmp",
		"/qr?data=x&ecl=Z",
		"/qr?data=x&color=notacolor",
		"/qr?data=x&shape=hexagon",
		"/qr?data=x&size=abc",
	}
	mux := NewMux()
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestGetQR_ContrastWarningHeader(t *testing.T) {
	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/qr?data=x&color=%23888888&bgcolor=%23999999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-QR-Contrast-Warning") == "" {
		t.Error("expected contrast warning header for similar fg/bg colors")
	}
}

func TestPostQR_Success(t *testing.T) {
	mux := NewMux()
	body := `{"data":"hello","format":"svg","shape":"circle"}`
	req := httptest.NewRequest(http.MethodPost, "/qr", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q", ct)
	}
	// POST is not cacheable output, so no Cache-Control should be set.
	if cc := rec.Header().Get("Cache-Control"); cc != "" {
		t.Errorf("Cache-Control = %q, want empty for POST", cc)
	}
	if !strings.HasPrefix(rec.Body.String(), "<svg") {
		t.Error("expected raw SVG body")
	}
}

func TestPostQR_WithLogo(t *testing.T) {
	logoImg := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			logoImg.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, logoImg); err != nil {
		t.Fatal(err)
	}
	logoB64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	reqBody, err := json.Marshal(map[string]string{
		"data": "https://example.com",
		"logo": "data:image/png;base64," + logoB64,
	})
	if err != nil {
		t.Fatal(err)
	}

	mux := NewMux()
	req := httptest.NewRequest(http.MethodPost, "/qr", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if _, _, err := image.Decode(bytes.NewReader(rec.Body.Bytes())); err != nil {
		t.Errorf("could not decode resulting PNG: %v", err)
	}
}

func TestPostQR_CorruptLogoIsBadRequest(t *testing.T) {
	// A logo that is valid base64 but not a decodable PNG/JPEG must surface
	// as a 400 (bad input), not a 500 — this failure only shows up once the
	// image bytes reach the raster decoder deep inside qr.Generate, so it's
	// a regression test for error-type unwrapping across that boundary.
	mux := NewMux()
	reqBody, err := json.Marshal(map[string]any{
		"data": "https://example.com",
		"logo": "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not a real png")),
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/qr", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestPostQR_InvalidJSON(t *testing.T) {
	mux := NewMux()
	req := httptest.NewRequest(http.MethodPost, "/qr", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPostQR_BodyTooLarge(t *testing.T) {
	mux := NewMux()
	huge := strings.Repeat("a", maxPOSTBodyBytes+2)
	body := `{"data":"` + huge + `"}`
	req := httptest.NewRequest(http.MethodPost, "/qr", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}
