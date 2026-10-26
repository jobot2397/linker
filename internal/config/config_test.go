package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExpandPathHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ExpandPath("~/.config/nvim", "/repo")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "nvim")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestExpandPathRelative(t *testing.T) {
	got, err := ExpandPath("local/bin", "/repo")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/repo", "local", "bin")
	if runtime.GOOS == "windows" {
		// Join with absolute base may produce different volume form; just check suffix.
		if filepath.Base(got) != "bin" {
			t.Fatalf("got %q", got)
		}
		return
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestExpandPathFromSlash(t *testing.T) {
	home, _ := os.UserHomeDir()
	got, err := ExpandPath("~/a/b", "/repo")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "a", "b")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFindWalksUp(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(root, "linker.toml")
	content := "[[links]]\nsource = \"x\"\ndestination = \"~/x\"\n"
	if err := os.WriteFile(cfg, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	path, repo, err := Find(sub)
	if err != nil {
		t.Fatal(err)
	}
	if path != cfg {
		t.Fatalf("path %q want %q", path, cfg)
	}
	if repo != root {
		t.Fatalf("repo %q want %q", repo, root)
	}
}

func TestLoadAndValidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "linker.toml")
	if err := os.WriteFile(path, []byte(`
[[links]]
source = "dotfiles/zshrc"
destination = "~/.zshrc"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Links) != 1 {
		t.Fatalf("links=%d", len(cfg.Links))
	}
}
