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
// Each line is targetW characters wide; each terminal row covers 2 pixel rows.
// Background pixels are rendered transparent (terminal default background).
//
// Four half-block cases per cell:
//   - both bg      → space (terminal bg shows through)
//   - top fg/bot bg → ▀ fg=top,  default bg
//   - top bg/bot fg → ▄ fg=bot,  default bg
//   - both fg      → ▀ fg=top,  bg=bot
func renderANSI(img image.Image, targetW int) []string {
	bounds := img.Bounds()
	srcW := bounds.Max.X - bounds.Min.X
	srcH := bounds.Max.Y - bounds.Min.Y

	targetH := int(math.Round(float64(srcH) / float64(srcW) * float64(targetW) * 0.5))
	pixH := targetH * 2

	// Detect image background by sampling corners and border midpoints.
	bgColor := detectBgColor(img, bounds)

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

	// A pixel is "background" if it is transparent or close to the image bg.
	const threshold = 28.0
	isBg := func(c color.RGBA) bool {
		if c.A < 128 {
			return true
		}
		dr := float64(int(c.R) - int(bgColor.R))
		dg := float64(int(c.G) - int(bgColor.G))
		db := float64(int(c.B) - int(bgColor.B))
		return math.Sqrt(dr*dr+dg*dg+db*db) < threshold
	}

	fg := func(sb *strings.Builder, c color.RGBA) {
		fmt.Fprintf(sb, "\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
	}
	bg := func(sb *strings.Builder, c color.RGBA) {
		fmt.Fprintf(sb, "\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
	}

	lines := make([]string, targetH)
	for row := 0; row < targetH; row++ {
		var sb strings.Builder
		for col := 0; col < targetW; col++ {
			top := sample(col, row*2)
			bot := sample(col, row*2+1)
			topBg := isBg(top)
			botBg := isBg(bot)

			switch {
			case topBg && botBg:
				// Both transparent → plain space, terminal bg shows through.
				sb.WriteString("\x1b[0m ")

			case !topBg && botBg:
				// Top has color, bottom is transparent → upper-half block.
				fg(&sb, top)
				sb.WriteString("\x1b[49m▀")

			case topBg && !botBg:
				// Top is transparent, bottom has color → lower-half block.
				fg(&sb, bot)
				sb.WriteString("\x1b[49m▄")

			default:
				// Both have color → upper-half block, fg=top bg=bot.
				fg(&sb, top)
				bg(&sb, bot)
				sb.WriteString("▀")
			}
		}
		sb.WriteString(ansiReset)
		lines[row] = sb.String()
	}
	return lines
}

// detectBgColor samples the image corners and border midpoints to identify
// the uniform background colour.
func detectBgColor(img image.Image, b image.Rectangle) color.RGBA {
	w, h := b.Max.X-b.Min.X, b.Max.Y-b.Min.Y
	points := [][2]int{
		{b.Min.X, b.Min.Y},
		{b.Max.X - 1, b.Min.Y},
		{b.Min.X, b.Max.Y - 1},
		{b.Max.X - 1, b.Max.Y - 1},
		{b.Min.X + w/2, b.Min.Y},
		{b.Min.X + w/2, b.Max.Y - 1},
		{b.Min.X, b.Min.Y + h/2},
		{b.Max.X - 1, b.Min.Y + h/2},
	}
	var rSum, gSum, bSum int
	for _, p := range points {
		r, g, bv, _ := img.At(p[0], p[1]).RGBA()
		rSum += int(r >> 8)
		gSum += int(g >> 8)
		bSum += int(bv >> 8)
	}
	n := len(points)
	return color.RGBA{
		R: uint8(rSum / n),
		G: uint8(gSum / n),
		B: uint8(bSum / n),
		A: 255,
	}
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
