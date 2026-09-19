package qr

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

// DecodeLogoParam accepts a logo image reference as either a data URI
// (data:image/png;base64,....) or bare base64, and returns the decoded
// image bytes. No remote fetching is performed — accepting a URL would mean
// making outbound requests on the caller's behalf (SSRF risk) from a public
// endpoint, so logos must be supplied inline.
func DecodeLogoParam(logo string) ([]byte, error) {
	if logo == "" {
		return nil, nil
	}

	payload := logo
	if idx := strings.Index(logo, ","); strings.HasPrefix(logo, "data:") && idx != -1 {
		payload = logo[idx+1:]
	}

	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return nil, invalid("logo must be base64-encoded image data or a data: URI")
		}
	}

	if len(data) > MaxLogoBytes {
		return nil, invalid("logo image exceeds maximum size of %d bytes", MaxLogoBytes)
	}

	return data, nil
}

func decodeLogo(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, invalid("logo could not be decoded as PNG or JPEG: %v", err)
	}
	return img, nil
}
