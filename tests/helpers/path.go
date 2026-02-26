package helpers

import (
	"path/filepath"
	"runtime"
)

// RepoPath builds an absolute path under the repository root.
func RepoPath(parts ...string) string {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	all := append([]string{root}, parts...)
	return filepath.Join(all...)
}
