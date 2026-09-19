package qr

import (
	standard "github.com/yeqown/go-qrcode/writer/standard"
	"github.com/yeqown/go-qrcode/writer/standard/shapes"
)

// shapeFor returns a custom IShape for shapes not built into the library.
// Square and circle are handled by the library's own defaults/options, so
// this returns nil for those.
func shapeFor(s Shape) standard.IShape {
	switch s {
	case ShapeRounded:
		return shapes.Assemble(shapes.RoundedFinder(), roundedBlock)
	case ShapeDiamond:
		return shapes.Assemble(shapes.SquareFinder(), diamondBlock)
	default:
		return nil
	}
}

func roundedBlock(ctx *standard.DrawContext) {
	w, h := ctx.Edge()
	x, y := ctx.UpperLeft()
	fw, fh := float64(w), float64(h)

	ctx.SetColor(ctx.Color())
	ctx.DrawRoundedRectangle(x, y, fw, fh, fw*0.3)
	ctx.Fill()
}

func diamondBlock(ctx *standard.DrawContext) {
	w, h := ctx.Edge()
	x, y := ctx.UpperLeft()
	fw, fh := float64(w), float64(h)
	cx, cy := x+fw/2, y+fh/2

	ctx.SetColor(ctx.Color())
	ctx.MoveTo(cx, y)
	ctx.LineTo(x+fw, cy)
	ctx.LineTo(cx, y+fh)
	ctx.LineTo(x, cy)
	ctx.ClosePath()
	ctx.Fill()
}
