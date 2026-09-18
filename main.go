// Command cnvw is a read-only viewer for Obsidian / JSON Canvas files.
//
// It behaves more like a two-dimensional less than like a canvas editor: it
// opens a .canvas file, preserves its spatial layout, and never writes to it.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"

	"github.com/dgcnz/cnvw/internal/app"
	"github.com/dgcnz/cnvw/internal/canvas"
)

// version is overridden at build time with -ldflags "-X main.version=...".
// A release built that way wins; otherwise buildVersion falls back to what the
// module system recorded.
var version = ""

// buildVersion reports the version to print for -version.
//
// The usual way in is `go install module@version`, which sets no ldflags, so a
// binary relying on those alone reports "dev" no matter which release it came
// from. The module version is already stamped into the build info, so read it
// from there.
func buildVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "dev"
	}
	// A build straight from a working tree records "(devel)".
	if info.Main.Version == "(devel)" {
		if rev := setting(info, "vcs.revision"); rev != "" {
			if len(rev) > 12 {
				rev = rev[:12]
			}
			if setting(info, "vcs.modified") == "true" {
				rev += "-dirty"
			}
			return rev
		}
		return "dev"
	}
	return info.Main.Version
}

func setting(info *debug.BuildInfo, key string) string {
	for _, s := range info.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}

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
		fmt.Println("cnvw", buildVersion())
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
