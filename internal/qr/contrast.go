package qr

import "math"

// contrastWarnThreshold is the minimum WCAG relative-luminance contrast
// ratio below which a QR code's foreground/background colors are likely to
// hurt scan reliability. WCAG's own text-legibility threshold (4.5) is
// stricter than QR scanning needs, so a lower bar is used here.
const contrastWarnThreshold = 3.0

// LowContrast reports whether the given foreground/background hex colors
// have a contrast ratio low enough to risk scan reliability.
func LowContrast(fgHex, bgHex string) bool {
	fg := parseHexColor(fgHex)
	bg := parseHexColor(bgHex)
	return contrastRatio(fg.R, fg.G, fg.B, bg.R, bg.G, bg.B) < contrastWarnThreshold
}

func contrastRatio(r1, g1, b1, r2, g2, b2 uint8) float64 {
	l1 := relativeLuminance(r1, g1, b1)
	l2 := relativeLuminance(r2, g2, b2)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(r, g, b uint8) float64 {
	rl := channelLuminance(r)
	gl := channelLuminance(g)
	bl := channelLuminance(b)
	return 0.2126*rl + 0.7152*gl + 0.0722*bl
}

func channelLuminance(c uint8) float64 {
	v := float64(c) / 255.0
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}
