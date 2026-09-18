// Command cnvw is a read-only viewer for Obsidian / JSON Canvas files.
//
// It behaves more like a two-dimensional less than like a canvas editor: it
// opens a .canvas file, preserves its spatial layout, and never writes to it.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/dgcnz/cnvw/internal/app"
	"github.com/dgcnz/cnvw/internal/canvas"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "cnvw:", err)
		os.Exit(1)
	}
}

func run() error {
	theme := flag.String("theme", "auto", "color theme: auto, dark or light")
	vault := flag.String("vault", "", "vault root for resolving file nodes (default: detected)")
	dump := flag.String("dump", "", "render one frame at WxH to stdout and exit")
	zoom := flag.Float64("zoom", 0, "zoom for -dump (0 fits the canvas)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println("cnvw", version)
		return nil
	}
	if flag.NArg() != 1 {
		flag.Usage()
		return fmt.Errorf("expected exactly one .canvas file")
	}

	path := flag.Arg(0)
	doc, err := canvas.Load(path)
	if err != nil {
		return err
	}

	root := *vault
	if root == "" {
		root = app.FindVault(path)
	}

	if *dump != "" {
		w, h, err := app.ParseSize(*dump)
		if err != nil {
			return err
		}
		fmt.Println(app.Dump(doc, path, root, app.DumpOpts{
			W: w, H: h, Zoom: *zoom, Theme: themeOrDark(*theme),
		}))
		return nil
	}

	m := app.New(doc, path, root, *theme)
	_, err = tea.NewProgram(m).Run()
	return err
}

// themeOrDark resolves "auto" for the non-interactive path, where there is no
// terminal to ask about its background color.
func themeOrDark(name string) string {
	if name == "auto" {
		return "dark"
	}
	return name
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: cnvw [flags] FILE.canvas\n\nflags:\n")
	flag.PrintDefaults()
}
