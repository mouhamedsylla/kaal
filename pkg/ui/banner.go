package ui

import (
	"fmt"
	"os"
)

const asciiLogo = `
  ██╗  ██╗ █████╗  █████╗ ██╗
  ██║ ██╔╝██╔══██╗██╔══██╗██║
  █████╔╝ ███████║███████║██║
  ██╔═██╗ ██╔══██║██╔══██║██║
  ██║  ██╗██║  ██║██║  ██║███████╗
  ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝`

// PrintBanner displays the kaal ASCII banner with tagline.
func PrintBanner(version string) {
	ver := version
	if ver == "" {
		ver = "dev"
	}

	cyan := "\x1b[36m"
	dim := "\x1b[2m"
	rst := "\x1b[0m"

	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		cyan, dim, rst = "", "", ""
	}

	fmt.Println(cyan + asciiLogo + rst)
	fmt.Println(dim + "  Dev Environment as Code — v" + ver + rst)
	fmt.Println(dim + "  Describe your infra once. Run locally. Ship anywhere." + rst)
	fmt.Println()
	fmt.Println("  kaal init    →  describe your infra in kaal.yaml")
	fmt.Println("  kaal up      →  simulate it locally")
	fmt.Println("  kaal push    →  build + push your image")
	fmt.Println("  kaal deploy  →  SSH into your VPS, pull, restart")
	fmt.Println()
}
