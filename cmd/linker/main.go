package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jobot2397/linker/internal/config"
	"github.com/jobot2397/linker/internal/symlink"
	"github.com/jobot2397/linker/internal/tui"
)

// version is set at link time via -ldflags "-X main.version=…".
var version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "linker: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			printUsage()
			return nil
		case "-v", "--version", "version":
			fmt.Printf("linker %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
			return nil
		case "tui":
			return runTUI()
		case "apply":
			filter := ""
			if len(args) > 1 {
				filter = args[1]
			}
			return runApply(filter)
		case "status":
			return runStatus()
		}
	}

	// No subcommand: if a path-like arg is given, treat it as a filter;
	// otherwise apply all links from the nearest linker.toml.
	if len(args) == 1 && !isFlag(args[0]) {
		return runApply(args[0])
	}
	if len(args) == 0 {
		return runApply("")
	}

	printUsage()
	return fmt.Errorf("unknown arguments: %v", args)
}

func isFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}

func loadConfig() (*config.Config, string, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", "", err
	}
	cfgPath, repoRoot, err := config.Find(cwd)
	if err != nil {
		return nil, "", "", err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", "", err
	}
	return cfg, repoRoot, cfgPath, nil
}

func runApply(filter string) error {
	cfg, repoRoot, cfgPath, err := loadConfig()
	if err != nil {
		return err
	}
	fmt.Printf("using %s\n", cfgPath)

	links := cfg.MatchFilter(repoRoot, filter)
	if len(links) == 0 {
		return fmt.Errorf("no links matched %q", filter)
	}

	var failed int
	for _, link := range links {
		src, dst, err := link.Resolve(repoRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", link.Source, err)
			failed++
			continue
		}
		out := symlink.Create(src, dst)
		printOutcome(out)
		if out.Result == symlink.Failed {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d link(s) failed", failed)
	}
	return nil
}

func runStatus() error {
	cfg, repoRoot, cfgPath, err := loadConfig()
	if err != nil {
		return err
	}
	fmt.Printf("using %s\n", cfgPath)

	for _, link := range cfg.Links {
		src, dst, err := link.Resolve(repoRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", link.Source, err)
			continue
		}
		st := symlink.Inspect(src, dst)
		mark := " "
		if st.Linked {
			mark = "✓"
		}
		fmt.Printf("  [%s] %s → %s  (%s)\n", mark, link.Source, displayPath(dst), st.Message)
	}
	return nil
}

func runTUI() error {
	cfg, repoRoot, cfgPath, err := loadConfig()
	if err != nil {
		return err
	}
	return tui.Run(cfg, repoRoot, cfgPath)
}

func printOutcome(out symlink.Outcome) {
	var mark string
	switch out.Result {
	case symlink.Created:
		mark = "✓"
	case symlink.AlreadyCorrect:
		mark = "·"
	case symlink.Skipped:
		mark = "!"
	case symlink.Removed:
		mark = "–"
	case symlink.Failed:
		mark = "✗"
	}
	fmt.Printf("  %s %s → %s  (%s)\n", mark, out.Source, displayPath(out.Destination), out.Message)
}

func displayPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	sep := string(filepath.Separator)
	prefix := home + sep
	if len(p) > len(prefix) && (p[:len(home)] == home) && (len(p) == len(home) || p[len(home)] == filepath.Separator) {
		return "~" + p[len(home):]
	}
	return p
}

func printUsage() {
	fmt.Printf(`linker %s — manage dotfile symlinks from linker.toml

Usage:
  linker                 Apply all links from nearest linker.toml
  linker <source>        Apply only the link matching <source>
  linker apply [source]  Same as above
  linker status          Show current link status
  linker tui             Interactive TUI to enable/disable links
  linker version         Print version
  linker help            Show this help

Config discovery:
  Walks upward from the current directory looking for linker.toml.

Windows notes:
  Creating symlinks requires Developer Mode or an elevated shell.
`, version)
}
