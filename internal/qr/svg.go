package qr

import (
	"bytes"
	"fmt"

	qrcode "github.com/yeqown/go-qrcode/v2"
)

// svgOptions configures SVG rendering. There is no SVG writer in the
// underlying QR library, so this walks the generated matrix directly and
// emits shape primitives — the only genuinely hand-rolled rendering code in
// this package.
type svgOptions struct {
	BlockWidth int
	Border     int
	FgColor    string
	BgColor    string
	Shape      Shape
}

func renderSVG(qrc *qrcode.QRCode, opts svgOptions) ([]byte, error) {
	dimension := qrc.Dimension()
	bw := opts.BlockWidth
	side := dimension*bw + 2*opts.Border

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" shape-rendering="crispEdges">`,
		side, side, side, side)
	fmt.Fprintf(&buf, `<rect x="0" y="0" width="%d" height="%d" fill="%s"/>`, side, side, opts.BgColor)

	err := qrc.Save(&svgWriter{buf: &buf, opts: opts, side: side})
	if err != nil {
		return nil, err
	}

	buf.WriteString(`</svg>`)
	return buf.Bytes(), nil
}

// svgWriter implements qrcode.Writer, translating the matrix into SVG shape
// elements as it is written.
type svgWriter struct {
	buf  *bytes.Buffer
	opts svgOptions
	side int
}

func (w *svgWriter) Close() error { return nil }

func (w *svgWriter) Write(mat qrcode.Matrix) error {
	bw := w.opts.BlockWidth
	border := w.opts.Border

	mat.Iterate(qrcode.IterDirection_ROW, func(x, y int, v qrcode.QRValue) {
		if !v.IsSet() {
			return
		}

		px, py := x*bw+border, y*bw+border
		writeModule(w.buf, w.opts.Shape, px, py, bw, w.opts.FgColor)
	})

	return nil
}

func writeModule(buf *bytes.Buffer, shape Shape, x, y, size int, fill string) {
	switch shape {
	case ShapeCircle:
		r := size / 2
		fmt.Fprintf(buf, `<circle cx="%d" cy="%d" r="%d" fill="%s"/>`, x+r, y+r, r, fill)
	case ShapeRounded:
		fmt.Fprintf(buf, `<rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="%s"/>`,
			x, y, size, size, int(float64(size)*0.3), fill)
	case ShapeDiamond:
		cx, cy := x+size/2, y+size/2
		fmt.Fprintf(buf, `<polygon points="%d,%d %d,%d %d,%d %d,%d" fill="%s"/>`,
			cx, y, x+size, cy, cx, y+size, x, cy, fill)
	default:
		fmt.Fprintf(buf, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`, x, y, size, size, fill)
	}
}
