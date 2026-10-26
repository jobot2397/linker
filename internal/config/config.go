package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the top-level linker.toml document.
type Config struct {
	// Links declares source → destination symlink mappings.
	Links []Link `toml:"links"`
}

// Link maps a file or directory in the repo to a destination path.
//
// Paths are resolved as follows:
//   - source: relative to the directory containing linker.toml (the repo root)
//   - destination: ~ expands to the user's home directory; other relative
//     paths are also resolved against the repo root
type Link struct {
	Source      string `toml:"source"`
	Destination string `toml:"destination"`
}

// Load reads and validates a linker.toml file at path.
func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks that the config has at least one well-formed link.
func (c *Config) Validate() error {
	if len(c.Links) == 0 {
		return fmt.Errorf("no [[links]] entries found")
	}
	for i, link := range c.Links {
		if strings.TrimSpace(link.Source) == "" {
			return fmt.Errorf("links[%d]: source is required", i)
		}
		if strings.TrimSpace(link.Destination) == "" {
			return fmt.Errorf("links[%d]: destination is required", i)
		}
	}
	return nil
}

// Resolve expands source and destination paths for a link relative to
// the repo root (directory containing linker.toml) and the user's home.
// Forward slashes in TOML are accepted on all platforms (filepath.FromSlash).
func (l Link) Resolve(repoRoot string) (src, dst string, err error) {
	src = filepath.Join(repoRoot, filepath.FromSlash(l.Source))

	dst, err = ExpandPath(l.Destination, repoRoot)
	if err != nil {
		return "", "", err
	}
	return src, dst, nil
}

// ExpandPath expands ~ to the home directory. Absolute paths are kept as-is.
// Other relative paths are joined with base (typically the repo root).
// Uses filepath so separators are correct on Windows, macOS, and Linux.
func ExpandPath(path, base string) (string, error) {
	path = strings.TrimSpace(path)
	// Normalize ~\ on Windows-style TOML as well as ~/
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		rest := filepath.FromSlash(strings.TrimPrefix(strings.TrimPrefix(path[1:], "/"), `\`))
		return filepath.Join(home, rest), nil
	}
	path = filepath.FromSlash(path)
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(base, path), nil
}

// Find walks up from start looking for a linker.toml file.
// Returns the absolute path to the file and the directory that contains it.
func Find(start string) (configPath, repoRoot string, err error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", "", err
	}

	for {
		candidate := filepath.Join(dir, "linker.toml")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", "", fmt.Errorf("linker.toml not found (searched from %s upward)", start)
}

// MatchFilter returns links whose source path matches filter.
// filter may be a relative source name, a path under the repo, or a basename.
func (c *Config) MatchFilter(repoRoot, filter string) []Link {
	if filter == "" {
		return c.Links
	}

	absFilter, err := filepath.Abs(filter)
	if err != nil {
		absFilter = filter
	}

	var matched []Link
	for _, link := range c.Links {
		src, _, err := link.Resolve(repoRoot)
		if err != nil {
			continue
		}
		if link.Source == filter ||
			filepath.Base(link.Source) == filter ||
			src == absFilter ||
			strings.HasSuffix(src, string(filepath.Separator)+filter) {
			matched = append(matched, link)
		}
	}
	return matched
}
