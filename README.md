# linker

Cross-platform CLI (and TUI) that creates and manages symlinks declared in a `linker.toml` — built for dotfiles repos.

**Version:** 0.1.0 (see [`VERSION`](VERSION))

## Install

### Homebrew

```bash
brew tap jobot2397/linker https://github.com/jobot2397/linker
brew install linker
# or latest main:
brew install --HEAD linker
```

The formula lives at [`Formula/linker.rb`](Formula/linker.rb) and is pinned to a git tag + revision. After tagging a release, run `./scripts/update-homebrew-formula.sh` (or `mise run formula:update`) so the formula tracks that version.

### Binaries / source

Download a binary from the [releases](https://github.com/jobot2397/linker/releases) page, or build from source:

```bash
go install github.com/jobot2397/linker/cmd/linker@latest
# or
mise run build   # embeds VERSION via ldflags
```

## Quick start

1. Put a `linker.toml` at the root of your dotfiles repo (see format below).
2. From anywhere under that repo:

```bash
linker              # apply all links
linker status       # show what's linked
linker tui          # interactive enable/disable UI
linker ~/.zshrc     # or: linker zshrc / linker dotfiles/zshrc
```

`linker` walks upward from the current working directory until it finds `linker.toml`.

## `linker.toml` format

```toml
[[links]]
source = "dotfiles/zshrc"       # relative to the repo root (this file's directory)
destination = "~/.zshrc"        # ~ = home; / works on all platforms

[[links]]
source = "dotfiles/config/nvim"
destination = "~/.config/nvim"
```

| Field | Meaning |
|-------|---------|
| `source` | Path relative to the directory containing `linker.toml` |
| `destination` | Where the symlink should live. `~` / `~/…` expand to your home directory; absolute paths are kept; other relative paths are relative to the repo root |

Forward slashes are fine on Windows — paths are normalized with `filepath`.

Copy [`linker.toml.example`](linker.toml.example) to get started.

## Commands

| Command | Description |
|---------|-------------|
| `linker` / `linker apply` | Create all symlinks |
| `linker apply <source>` | Create only matching link(s) |
| `linker status` | Print on/off state for each link |
| `linker tui` | Terminal UI — toggle links, then apply |
| `linker version` | Print version and GOOS/GOARCH |

### TUI controls

| Key | Action |
|-----|--------|
| `space` / `enter` | Toggle selected link on/off |
| `a` | Apply desired states (create or remove) |
| `A` / `D` | Enable / disable all |
| `r` | Refresh actual filesystem status |
| `/` | Filter |
| `q` | Quit |

A `*` marks a link whose desired state differs from the filesystem. Apply never overwrites a real file or an unexpected symlink — those are skipped with a warning.

## Cross-platform notes

- **macOS / Linux:** `os.Symlink` works for normal users.
- **Windows:** Creating symlinks typically requires [Developer Mode](https://learn.microsoft.com/en-us/windows/apps/get-started/enable-your-device-for-development) or an elevated shell. If creation fails, linker prints that guidance and continues.
- Path comparison is case-insensitive on Windows (and via `EvalSymlinks` fallbacks on macOS).
- Existing destinations that are not the expected symlink are **never** removed or overwritten.

## Project layout

```
cmd/linker/          # main CLI entrypoint
internal/config/     # linker.toml parse, find, path expand
internal/symlink/    # create / remove / inspect (cross-platform)
internal/tui/        # charmbracelet/bubbletea UI
scripts/build-release.sh
.github/workflows/release.yml
```

## Versioning & release

Single source of truth: the [`VERSION`](VERSION) file.

```bash
./scripts/bump-version.sh 0.2.0          # bumps VERSION, main.go default, README, formula
git commit -am "chore: bump version to v0.2.0"
git tag -a v0.2.0 -m "linker v0.2.0"
git push && git push origin v0.2.0
./scripts/update-homebrew-formula.sh     # pin Formula/linker.rb to the tag revision
git commit -am "chore: update Homebrew formula for v0.2.0" && git push
```

```bash
./scripts/build-release.sh
# artifacts land in ./dist/
```

Targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`, `windows/arm64`.

Or push a `v*` tag (must match `VERSION`) — GitHub Actions builds all platforms and attaches them to the release.

## License

MIT
