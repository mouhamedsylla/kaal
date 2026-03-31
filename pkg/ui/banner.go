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

// PrintBanner displays the pilot ASCII banner with tagline.
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
	fmt.Println("  pilot init    →  describe your infra in pilot.yaml")
	fmt.Println("  pilot up      →  simulate it locally")
	fmt.Println("  pilot push    →  build + push your image")
	fmt.Println("  pilot deploy  →  SSH into your VPS, pull, restart")
	fmt.Println()
}
