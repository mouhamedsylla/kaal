// Package assets embeds static files into the kaal binary.
package assets

import _ "embed"

// KaalPixelPNG is the mascot pixel art, embedded at build time.
//
//go:embed kaal_pixel.png
var KaalPixelPNG []byte
