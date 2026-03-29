package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"math"
	"os"
	"strings"

	kaalassets "github.com/mouhamedsylla/kaal/assets"
)

const (
	bannerWidth = 48
	ansiReset   = "\x1b[0m"
)

// PrintBanner displays the kaal mascot with branding in the terminal.
// It silently skips rendering if the terminal does not support color
// (NO_COLOR env var set, or TERM=dumb).
func PrintBanner(version string) {
	if !supportsColor() {
		printTextBanner(version)
		return
	}

	img, _, err := image.Decode(bytes.NewReader(kaalassets.KaalPixelPNG))
	if err != nil {
		printTextBanner(version)
		return
	}

	lines := renderANSI(img, bannerWidth)
	side := buildSideText(version, len(lines))

	for i, line := range lines {
		if i < len(side) {
			fmt.Printf("%s  %s\n", line, side[i])
		} else {
			fmt.Println(line)
		}
	}
	fmt.Println()
}

// supportsColor returns true when the terminal can render ANSI truecolor.
func supportsColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := os.Getenv("TERM")
	if term == "dumb" || term == "" {
		return false
	}
	return true
}

// renderANSI converts img to a slice of ANSI half-block art lines.
// Each line is targetW characters wide; each terminal row covers 2 pixel rows
// using the ▀ (U+2580) upper-half block.
func renderANSI(img image.Image, targetW int) []string {
	bounds := img.Bounds()
	srcW := bounds.Max.X - bounds.Min.X
	srcH := bounds.Max.Y - bounds.Min.Y

	// Compute terminal rows preserving aspect ratio (cells are ~2× taller).
	targetH := int(math.Round(float64(srcH) / float64(srcW) * float64(targetW) * 0.5))
	pixH := targetH * 2

	// Background colour to blend transparent pixels against.
	bg := color.RGBA{R: 74, G: 74, B: 74, A: 255}

	sample := func(px, py int) color.RGBA {
		sx := bounds.Min.X + px*srcW/targetW
		sy := bounds.Min.Y + py*srcH/pixH
		r, g, b, a := img.At(sx, sy).RGBA()
		return color.RGBA{
			R: uint8(r >> 8),
			G: uint8(g >> 8),
			B: uint8(b >> 8),
			A: uint8(a >> 8),
		}
	}

	blend := func(c color.RGBA) color.RGBA {
		if c.A == 255 {
			return c
		}
		alpha := float64(c.A) / 255.0
		return color.RGBA{
			R: uint8(float64(c.R)*alpha + float64(bg.R)*(1-alpha)),
			G: uint8(float64(c.G)*alpha + float64(bg.G)*(1-alpha)),
			B: uint8(float64(c.B)*alpha + float64(bg.B)*(1-alpha)),
			A: 255,
		}
	}

	lines := make([]string, targetH)
	for row := 0; row < targetH; row++ {
		var sb strings.Builder
		for col := 0; col < targetW; col++ {
			top := blend(sample(col, row*2))
			bot := blend(sample(col, row*2+1))
			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				top.R, top.G, top.B,
				bot.R, bot.G, bot.B,
			)
		}
		sb.WriteString(ansiReset)
		lines[row] = sb.String()
	}
	return lines
}

// buildSideText returns lines of branding text to display beside the mascot.
// The text is vertically centered within totalRows.
func buildSideText(version string, totalRows int) []string {
	bold := "\x1b[1m"
	dim := "\x1b[2m"
	cyan := "\x1b[36m"
	yellow := "\x1b[33m"
	green := "\x1b[32m"
	rst := "\x1b[0m"

	ver := version
	if ver == "" {
		ver = "dev"
	}

	content := []string{
		"",
		bold + cyan + "  kaal" + rst,
		dim + "  Dev Environment as Code" + rst,
		"",
		dim + "  Describe your infra once." + rst,
		dim + "  Run it locally. Ship it anywhere." + rst,
		"",
		yellow + "  kaal init   " + rst + dim + "→ describe your infra" + rst,
		yellow + "  kaal up     " + rst + dim + "→ simulate it locally" + rst,
		yellow + "  kaal push   " + rst + dim + "→ build + push image" + rst,
		yellow + "  kaal deploy " + rst + dim + "→ SSH + docker compose" + rst,
		"",
		green + "  kaal mcp serve" + rst + dim + "  ← AI-native via MCP" + rst,
		"",
		dim + "  v" + ver + rst,
		"",
	}

	// Vertically center within totalRows
	padding := (totalRows - len(content)) / 2
	if padding < 0 {
		padding = 0
	}

	result := make([]string, totalRows)
	for i := range result {
		idx := i - padding
		if idx >= 0 && idx < len(content) {
			result[i] = content[idx]
		} else {
			result[i] = ""
		}
	}
	return result
}

// printTextBanner is the fallback when ANSI colors are not available.
func printTextBanner(version string) {
	ver := version
	if ver == "" {
		ver = "dev"
	}
	fmt.Printf("\nkaal %s — Dev Environment as Code\n", ver)
	fmt.Println("Describe your infra once. Run it locally. Ship it anywhere.\n")
}
