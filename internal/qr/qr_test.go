package qr

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"testing"
)

func TestGenerate_AllFormatsAndShapes(t *testing.T) {
	formats := []Format{FormatPNG, FormatJPEG, FormatSVG}
	shapes := []Shape{ShapeSquare, ShapeRounded, ShapeCircle, ShapeDiamond}

	for _, f := range formats {
		for _, s := range shapes {
			t.Run(string(f)+"_"+string(s), func(t *testing.T) {
				res, err := Generate(Options{
					Data:   "https://example.com",
					Format: f,
					Shape:  s,
				})
				if err != nil {
					t.Fatalf("Generate() error = %v", err)
				}
				if len(res.Bytes) == 0 {
					t.Fatal("expected non-empty output")
				}
				if res.ContentType != f.ContentType() {
					t.Errorf("ContentType = %q, want %q", res.ContentType, f.ContentType())
				}

				switch f {
				case FormatSVG:
					if !strings.HasPrefix(string(res.Bytes), "<svg") {
						t.Error("expected SVG output to start with <svg")
					}
				default:
					if _, _, err := image.Decode(bytes.NewReader(res.Bytes)); err != nil {
						t.Errorf("could not decode raster image: %v", err)
					}
				}
			})
		}
	}
}

func TestGenerate_ECLLevels(t *testing.T) {
	for _, ecl := range []ECL{ECLLow, ECLMedium, ECLQuart, ECLHighest} {
		if _, err := Generate(Options{Data: "hello", ECL: ecl}); err != nil {
			t.Errorf("ecl=%s: %v", ecl, err)
		}
	}
}

func TestGenerate_Size(t *testing.T) {
	res, err := Generate(Options{Data: "https://example.com", Format: FormatPNG, Size: 512})
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(res.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	// Integer division on block width means the result approximates, not
	// exactly matches, the requested size.
	if b.Dx() < 400 || b.Dx() > 600 {
		t.Errorf("width = %d, want roughly 512", b.Dx())
	}
	if b.Dx() != b.Dy() {
		t.Errorf("expected square image, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestGenerate_Validation(t *testing.T) {
	cases := []struct {
		name string
		opts Options
	}{
		{"empty data", Options{Data: ""}},
		{"bad format", Options{Data: "x", Format: "bmp"}},
		{"bad ecl", Options{Data: "x", ECL: "Z"}},
		{"bad fg color", Options{Data: "x", Color: "notacolor"}},
		{"bad bg color", Options{Data: "x", BgColor: "notacolor"}},
		{"bad shape", Options{Data: "x", Shape: "hexagon"}},
		{"negative size", Options{Data: "x", Size: -1}},
		{"oversized size", Options{Data: "x", Size: 5000}},
		{"data exceeds capacity", Options{Data: strings.Repeat("a", 5000), ECL: ECLHighest}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Generate(tc.opts)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var ve *ValidationError
			if !isValidationError(err, &ve) {
				t.Errorf("expected ValidationError, got %T: %v", err, err)
			}
		})
	}
}

func isValidationError(err error, target **ValidationError) bool {
	ve, ok := err.(*ValidationError)
	if ok {
		*target = ve
	}
	return ok
}

func TestLowContrast(t *testing.T) {
	if LowContrast("#000000", "#ffffff") {
		t.Error("black on white should not warn")
	}
	if !LowContrast("#888888", "#999999") {
		t.Error("similar grays should warn")
	}
}
