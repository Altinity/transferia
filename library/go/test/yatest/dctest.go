//go:build !arcadia
// +build !arcadia

package yatest

import (
	"os"
	"path/filepath"
)

func doInit() {
	isRunningUnderGoTest = true
	context.Initialized = true
	context.Runtime.SourceRoot = detectSourceRoot()
}

func detectSourceRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}
