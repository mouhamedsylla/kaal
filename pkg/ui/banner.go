package ui

import (
	"fmt"
	"os"
)

const asciiLogo = `
    ██████╗ ██╗██╗      ██████╗ ████████╗
    ██╔══██╗██║██║     ██╔═══██╗╚══██╔══╝
    ██████╔╝██║██║     ██║   ██║   ██║
    ██╔═══╝ ██║██║     ██║   ██║   ██║
    ██║     ██║███████╗╚██████╔╝   ██║
    ╚═╝     ╚═╝╚══════╝ ╚═════╝    ╚═╝`

// PrintBanner affiche le banner pilot.
func PrintBanner(version string) {
	ver := version
	if ver == "" {
		ver = "dev"
	}

	// Couleurs ANSI
	cyan := "\x1b[36m"
	dim := "\x1b[2m"
	rst := "\x1b[0m"
	bold := "\x1b[1m"

	// Désactiver les couleurs si NO_COLOR ou TERM=dumb
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		cyan, dim, rst, bold = "", "", "", ""
	}

	fmt.Println()
	fmt.Println(cyan + asciiLogo + rst)
	fmt.Println(dim + "    Dev Environment as Code — v" + ver + rst)
	fmt.Println(dim + "    Describe your infra once. Run locally. Ship anywhere." + rst)
	fmt.Println()
	fmt.Println(bold + "  Quick start:" + rst)
	fmt.Println("  pilot init    " + dim + "→  describe your infra in pilot.yaml" + rst)
	fmt.Println("  pilot up      " + dim + "→  simulate it locally" + rst)
	fmt.Println("  pilot push    " + dim + "→  build + push your image" + rst)
	fmt.Println("  pilot deploy  " + dim + "→  SSH into your VPS, pull, restart" + rst)
	fmt.Println()
}
