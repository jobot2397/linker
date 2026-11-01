package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
	cfgPathFlag, args := peelConfigFlag(args)

	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			printUsage()
			return nil
		case "-v", "--version", "version":
			fmt.Printf("linker %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
			return nil
		case "tui":
			return runTUI(cfgPathFlag)
		case "apply":
			filter := ""
			if len(args) > 1 {
				filter = args[1]
			}
			return runApply(cfgPathFlag, filter)
		case "status":
			return runStatus(cfgPathFlag)
		}
	}

	// Bare path to an existing file → treat as config and open the TUI.
	if len(args) == 1 && !isFlag(args[0]) && looksLikeConfigFile(args[0]) {
		return runTUI(args[0])
	}

	// Bare source filter still applies that one link.
	if len(args) == 1 && !isFlag(args[0]) {
		return runApply(cfgPathFlag, args[0])
	}

	// Default: interactive TUI.
	if len(args) == 0 {
		return runTUI(cfgPathFlag)
	}

	printUsage()
	return fmt.Errorf("unknown arguments: %v", args)
}

func isFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}

// peelConfigFlag extracts -c/--config <path> from args.
func peelConfigFlag(args []string) (cfgPath string, rest []string) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-c" || a == "--config":
			if i+1 >= len(args) {
				// Leave it; run() will fail later with a clearer load error.
				rest = append(rest, a)
				continue
			}
			cfgPath = args[i+1]
			i++
		case len(a) > 9 && a[:9] == "--config=":
			cfgPath = a[9:]
		case len(a) > 2 && a[:2] == "-c" && a != "-c":
			// unsupported short form glued; pass through
			rest = append(rest, a)
		default:
			rest = append(rest, a)
		}
	}
	return cfgPath, rest
}

func looksLikeConfigFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	base := filepath.Base(path)
	if base == "linker.toml" || base == "linker.toml.example" {
		return true
	}
	return filepath.Ext(path) == ".toml" || strings.HasSuffix(base, ".toml.example")
}

func loadConfig(explicit string) (*config.Config, string, string, error) {
	if explicit != "" {
		cfgPath, err := filepath.Abs(explicit)
		if err != nil {
			return nil, "", "", err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return nil, "", "", err
		}
		return cfg, filepath.Dir(cfgPath), cfgPath, nil
	}

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

func runApply(explicit, filter string) error {
	cfg, repoRoot, cfgPath, err := loadConfig(explicit)
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

func runStatus(explicit string) error {
	cfg, repoRoot, cfgPath, err := loadConfig(explicit)
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

func runTUI(explicit string) error {
	cfg, repoRoot, cfgPath, err := loadConfig(explicit)
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
  linker                      Open the TUI (finds linker.toml upward)
  linker <config.toml>        Open the TUI with an explicit config file
  linker apply [source]       Create symlinks (all, or matching source)
  linker <source>             Apply only the link matching <source>
  linker status               Show current link status
  linker tui                  Same as bare linker
  linker version              Print version
  linker help                 Show this help

Options:
  -c, --config <path>         Use this config instead of searching

Config discovery:
  By default walks upward from the current directory looking for linker.toml
  (the filename must be exactly linker.toml — not linker.toml.example).

Windows notes:
  Creating symlinks requires Developer Mode or an elevated shell.
`, version)
}
