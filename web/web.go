// Package web embeds the static demo UI.
package web

import "embed"

//go:embed index.html
var files embed.FS

// Index returns the contents of the demo page.
func Index() ([]byte, error) {
	return files.ReadFile("index.html")
}
