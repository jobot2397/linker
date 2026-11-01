package symlink

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// Result describes what happened for a single link operation.
type Result int

const (
	Created Result = iota
	AlreadyCorrect
	Skipped
	Removed
	Failed
)

func (r Result) String() string {
	switch r {
	case Created:
		return "created"
	case AlreadyCorrect:
		return "already linked"
	case Skipped:
		return "skipped"
	case Removed:
		return "removed"
	case Failed:
		return "failed"
	default:
		return "unknown"
	}
}

// Outcome is the result of creating or removing one symlink.
type Outcome struct {
	Source      string
	Destination string
	Result      Result
	Message     string
	Err         error
}

// Status reports whether a destination currently points at the expected source.
type Status struct {
	Source      string
	Destination string
	Linked      bool // true if dst is a symlink to src
	Exists      bool // true if dst exists at all
	IsSymlink   bool
	Target      string // current symlink target, if any
	Message     string
}

// Inspect returns the current state of destination relative to source.
func Inspect(src, dst string) Status {
	st := Status{Source: src, Destination: dst}

	info, err := os.Lstat(dst)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			st.Message = "not linked"
			return st
		}
		st.Message = err.Error()
		return st
	}

	st.Exists = true
	if info.Mode()&os.ModeSymlink == 0 {
		st.Message = "exists (not a symlink)"
		return st
	}

	st.IsSymlink = true
	target, err := os.Readlink(dst)
	if err != nil {
		st.Message = err.Error()
		return st
	}
	st.Target = target

	resolved := target
	if !filepath.IsAbs(target) {
		resolved = filepath.Join(filepath.Dir(dst), target)
	}
	resolved = filepath.Clean(resolved)
	srcClean := filepath.Clean(src)

	if samePath(resolved, srcClean) {
		st.Linked = true
		st.Message = "linked"
		return st
	}

	st.Message = fmt.Sprintf("points elsewhere → %s", target)
	return st
}

// Create makes a symlink from dst → src. Existing correct links are left alone.
// Existing files or wrong symlinks are skipped with a warning (never overwritten).
func Create(src, dst string) Outcome {
	out := Outcome{Source: src, Destination: dst}

	if _, err := os.Lstat(src); err != nil {
		out.Result = Failed
		out.Err = fmt.Errorf("source missing: %w", err)
		out.Message = out.Err.Error()
		return out
	}

	st := Inspect(src, dst)
	if st.Linked {
		out.Result = AlreadyCorrect
		out.Message = "already linked correctly"
		return out
	}
	if st.Exists {
		out.Result = Skipped
		out.Message = fmt.Sprintf("destination exists (%s); leave it alone", st.Message)
		return out
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		out.Result = Failed
		out.Err = fmt.Errorf("create parent directory: %w", err)
		out.Message = out.Err.Error()
		return out
	}

	if err := os.Symlink(src, dst); err != nil {
		out.Result = Failed
		out.Err = wrapSymlinkError(err)
		out.Message = out.Err.Error()
		return out
	}

	out.Result = Created
	out.Message = "symlink created"
	return out
}

// Remove deletes dst only if it is a symlink pointing at src.
func Remove(src, dst string) Outcome {
	out := Outcome{Source: src, Destination: dst}

	st := Inspect(src, dst)
	if !st.Exists {
		out.Result = AlreadyCorrect
		out.Message = "already absent"
		return out
	}
	if !st.Linked {
		out.Result = Skipped
		out.Message = fmt.Sprintf("not our link (%s); leave it alone", st.Message)
		return out
	}

	if err := os.Remove(dst); err != nil {
		out.Result = Failed
		out.Err = err
		out.Message = err.Error()
		return out
	}

	out.Result = Removed
	out.Message = "symlink removed"
	return out
}

// ApplyDesired creates the symlink when wantLinked is true, otherwise removes it.
func ApplyDesired(src, dst string, wantLinked bool) Outcome {
	if wantLinked {
		return Create(src, dst)
	}
	return Remove(src, dst)
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return equalFoldPath(a, b)
	}
	// On macOS, paths are often case-insensitive; compare cleaned forms first,
	// then fall back to EvalSymlinks when both exist.
	if a == b {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA == nil && errB == nil {
		if runtime.GOOS == "darwin" {
			return equalFoldPath(ra, rb)
		}
		return ra == rb
	}
	return false
}

func equalFoldPath(a, b string) bool {
	return equalFold(filepath.Clean(a), filepath.Clean(b))
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca == cb {
			continue
		}
		// ASCII fold is enough for typical Windows/macOS paths
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// wrapSymlinkError adds platform-specific guidance for common symlink failures.
func wrapSymlinkError(err error) error {
	if err == nil {
		return nil
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("create symlink: %w", err)
	}
	return fmt.Errorf(
		"create symlink: %w\n"+
			"  On Windows, creating symlinks usually requires Developer Mode\n"+
			"  (Settings → Privacy & security → For developers) or an elevated shell.\n"+
			"  See: https://learn.microsoft.com/en-us/windows/apps/get-started/enable-your-device-for-development",
		err,
	)
}
