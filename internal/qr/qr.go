// Package qr generates QR code images (SVG, PNG, JPEG) from a set of
// user-supplied options. It is a thin, validating wrapper around
// github.com/yeqown/go-qrcode/v2 — matrix generation, encoding, and masking
// are all delegated to that library.
package qr

import (
	"bytes"
	"fmt"
	"image/color"
	"regexp"

	qrcode "github.com/yeqown/go-qrcode/v2"
	standard "github.com/yeqown/go-qrcode/writer/standard"
)

// Format is an output image format.
type Format string

const (
	FormatPNG  Format = "png"
	FormatJPEG Format = "jpeg"
	FormatSVG  Format = "svg"
)

// ContentType returns the HTTP Content-Type for the format.
func (f Format) ContentType() string {
	switch f {
	case FormatPNG:
		return "image/png"
	case FormatJPEG:
		return "image/jpeg"
	case FormatSVG:
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// Shape is a QR module (block) shape.
type Shape string

const (
	ShapeSquare  Shape = "square"
	ShapeRounded Shape = "rounded"
	ShapeCircle  Shape = "circle"
	ShapeDiamond Shape = "diamond"
)

// ECL is an error correction level.
type ECL string

const (
	ECLLow     ECL = "L"
	ECLMedium  ECL = "M"
	ECLQuart   ECL = "Q"
	ECLHighest ECL = "H"
)

// quietZoneModules is the standard QR code quiet zone width, in modules,
// enforced on every raster and SVG output regardless of requested size.
const quietZoneModules = 4

const (
	// MinBlockWidth/MaxBlockWidth bound the per-module pixel size derived
	// from the requested output size, to keep memory/CPU use predictable.
	minBlockWidth = 1
	maxBlockWidth = 60

	// DefaultBlockWidth is used when no size is requested.
	defaultBlockWidth = 8

	// MaxLogoBytes bounds the size of a decoded logo image.
	MaxLogoBytes = 2 << 20 // 2MiB
)

var hexColorRe = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// Options describes a single QR code render request.
type Options struct {
	Data    string
	Format  Format
	Size    int // desired total output width/height in pixels; 0 = default
	ECL     ECL
	Color   string // foreground hex color, e.g. "#000000"
	BgColor string // background hex color, e.g. "#ffffff"
	Shape   Shape
	Logo    []byte // optional decoded logo image (PNG or JPEG bytes)
}

// ValidationError is returned for bad input and should map to an HTTP 4xx.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, args...)}
}

// Result is a generated QR code image.
type Result struct {
	Bytes          []byte
	ContentType    string
	ContrastWarned bool
}

// Generate validates opts and renders the QR code.
func Generate(opts Options) (*Result, error) {
	if opts.Data == "" {
		return nil, invalid("data is required")
	}

	format := opts.Format
	if format == "" {
		format = FormatPNG
	}
	if format != FormatPNG && format != FormatJPEG && format != FormatSVG {
		return nil, invalid("format must be one of: png, jpeg, svg")
	}

	ecl := opts.ECL
	if ecl == "" {
		ecl = ECLMedium
	}
	eclOpt, err := eclToLibrary(ecl)
	if err != nil {
		return nil, err
	}

	fgHex := opts.Color
	if fgHex == "" {
		fgHex = "#000000"
	}
	bgHex := opts.BgColor
	if bgHex == "" {
		bgHex = "#ffffff"
	}
	if !hexColorRe.MatchString(fgHex) {
		return nil, invalid("color must be a hex value like #000000")
	}
	if !hexColorRe.MatchString(bgHex) {
		return nil, invalid("bgcolor must be a hex value like #ffffff")
	}

	shape := opts.Shape
	if shape == "" {
		shape = ShapeSquare
	}
	switch shape {
	case ShapeSquare, ShapeRounded, ShapeCircle, ShapeDiamond:
	default:
		return nil, invalid("shape must be one of: square, rounded, circle, diamond")
	}

	if opts.Size < 0 {
		return nil, invalid("size must be positive")
	}
	if opts.Size > 4096 {
		return nil, invalid("size must be 4096 or smaller")
	}

	if len(opts.Logo) > MaxLogoBytes {
		return nil, invalid("logo image exceeds maximum size of %d bytes", MaxLogoBytes)
	}

	qrc, err := qrcode.NewWith(opts.Data, eclOpt)
	if err != nil {
		return nil, invalid("data cannot be encoded: %v", err)
	}

	dimension := qrc.Dimension()
	blockWidth := blockWidthFor(opts.Size, dimension)
	border := blockWidth * quietZoneModules

	fgColor := parseHexColor(fgHex)
	bgColor := parseHexColor(bgHex)

	var imgBytes []byte
	switch format {
	case FormatSVG:
		imgBytes, err = renderSVG(qrc, svgOptions{
			BlockWidth: blockWidth,
			Border:     border,
			FgColor:    fgHex,
			BgColor:    bgHex,
			Shape:      shape,
			Logo:       opts.Logo,
		})
	default:
		imgBytes, err = renderRaster(qrc, rasterOptions{
			Format:     format,
			BlockWidth: blockWidth,
			Border:     border,
			FgColor:    fgColor,
			BgColor:    bgColor,
			Shape:      shape,
			Logo:       opts.Logo,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	return &Result{
		Bytes:          imgBytes,
		ContentType:    format.ContentType(),
		ContrastWarned: LowContrast(fgHex, bgHex),
	}, nil
}

func eclToLibrary(e ECL) (qrcode.EncodeOption, error) {
	switch e {
	case ECLLow:
		return qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionLow), nil
	case ECLMedium:
		return qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionMedium), nil
	case ECLQuart:
		return qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionQuart), nil
	case ECLHighest:
		return qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionHighest), nil
	default:
		return nil, invalid("ecl must be one of: L, M, Q, H")
	}
}

func blockWidthFor(size, dimension int) int {
	if size <= 0 || dimension <= 0 {
		return defaultBlockWidth
	}

	bw := size / (dimension + 2*quietZoneModules)
	if bw < minBlockWidth {
		bw = minBlockWidth
	}
	if bw > maxBlockWidth {
		bw = maxBlockWidth
	}
	return bw
}

func parseHexColor(hex string) color.RGBA {
	c := color.RGBA{A: 0xff}
	switch len(hex) {
	case 7:
		fmt.Sscanf(hex, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	case 4:
		var r, g, b uint8
		fmt.Sscanf(hex, "#%1x%1x%1x", &r, &g, &b)
		c.R, c.G, c.B = r*17, g*17, b*17
	}
	return c
}

type rasterOptions struct {
	Format     Format
	BlockWidth int
	Border     int
	FgColor    color.RGBA
	BgColor    color.RGBA
	Shape      Shape
	Logo       []byte
}

func renderRaster(qrc *qrcode.QRCode, opts rasterOptions) ([]byte, error) {
	imgOpts := []standard.ImageOption{
		standard.WithFgColor(opts.FgColor),
		standard.WithBgColor(opts.BgColor),
		standard.WithQRWidth(uint8(opts.BlockWidth)),
		standard.WithBorderWidth(opts.Border),
	}

	switch opts.Format {
	case FormatPNG:
		imgOpts = append(imgOpts, standard.WithBuiltinImageEncoder(standard.PNG_FORMAT))
	default:
		imgOpts = append(imgOpts, standard.WithBuiltinImageEncoder(standard.JPEG_FORMAT))
	}

	if s := shapeFor(opts.Shape); s != nil {
		imgOpts = append(imgOpts, standard.WithCustomShape(s))
	} else if opts.Shape == ShapeCircle {
		imgOpts = append(imgOpts, standard.WithCircleShape())
	}

	if len(opts.Logo) > 0 {
		img, err := decodeLogo(opts.Logo)
		if err != nil {
			return nil, err
		}
		imgOpts = append(imgOpts, standard.WithLogoImage(img), standard.WithLogoSafeZone())
	}

	var buf bytes.Buffer
	w := standard.NewWithWriter(nopWriteCloser{&buf}, imgOpts...)
	if err := qrc.Save(w); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type nopWriteCloser struct{ *bytes.Buffer }

func (nopWriteCloser) Close() error { return nil }
