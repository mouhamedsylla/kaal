// tools/img2term converts a PNG image to ANSI half-block terminal art.
//
// Usage:
//
//	go run tools/img2term/main.go assets/kaal_pixel.png [width]
//
// Each terminal row renders 2 pixel rows using the ▀ (U+2580) half-block:
//   - foreground color = top pixel
//   - background color = bottom pixel
//
// Output is raw ANSI escape sequences printed to stdout.
package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"math"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: img2term <image.png> [width]")
		os.Exit(1)
	}

	targetW := 52
	if len(os.Args) >= 3 {
		w, err := strconv.Atoi(os.Args[2])
		if err != nil || w <= 0 {
			fmt.Fprintln(os.Stderr, "invalid width")
			os.Exit(1)
		}
		targetW = w
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	bounds := src.Bounds()
	srcW := bounds.Max.X - bounds.Min.X
	srcH := bounds.Max.Y - bounds.Min.Y

	// Maintain aspect ratio; each cell is ~2x tall so divide height by 2.
	targetH := int(math.Round(float64(srcH) / float64(srcW) * float64(targetW) * 0.5))
	// targetH is in terminal rows; actual pixels = targetH * 2
	pixH := targetH * 2

	// Nearest-neighbour sample at (x, y) in target pixel space
	sample := func(px, py int) color.RGBA {
		sx := bounds.Min.X + px*srcW/targetW
		sy := bounds.Min.Y + py*srcH/pixH
		r, g, b, a := src.At(sx, sy).RGBA()
		// 16-bit → 8-bit
		return color.RGBA{
			R: uint8(r >> 8),
			G: uint8(g >> 8),
			B: uint8(b >> 8),
			A: uint8(a >> 8),
		}
	}

	// Background colour to blend transparent pixels into (dark grey matching the image bg)
	bg := color.RGBA{R: 74, G: 74, B: 74, A: 255}

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

	reset := "\x1b[0m"

	for row := 0; row < targetH; row++ {
		for col := 0; col < targetW; col++ {
			top := blend(sample(col, row*2))
			bot := blend(sample(col, row*2+1))

			// ESC[38;2;R;G;Bm  → foreground (top half)
			// ESC[48;2;R;G;Bm  → background (bottom half)
			fmt.Printf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				top.R, top.G, top.B,
				bot.R, bot.G, bot.B,
			)
		}
		fmt.Print(reset + "\n")
	}
}
