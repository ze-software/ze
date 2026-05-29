package instanceflag

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/pflag"
)

var (
	parentDir = func() string {
		def := os.Getenv("GOKRAZY_PARENT_DIR")
		if def == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				homeDir = fmt.Sprintf("os.UserHomeDir failed: %v", err)
			}
			def = filepath.Join(homeDir, "gokrazy")
		}
		return def
	}()

	instance = func() string {
		def := os.Getenv("GOKRAZY_INSTANCE")
		if def == "" {
			def = instanceFromPWD()
		}
		if def == "" {
			def = "hello"
		}
		return def
	}()
)

func instanceFromPWD() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	wdAbs, err := filepath.Abs(wd)
	if err != nil {
		return ""
	}

	parentAbs, err := filepath.Abs(parentDir)
	if err != nil {
		return ""
	}

	if !strings.HasPrefix(wdAbs, parentAbs+"/") {
		return ""
	}

	// Process is running in an instance directory (a
	// subdirectory of the parent dir), so default the instance
	// flag to that same subdirectory.
	instance := strings.TrimPrefix(wdAbs, parentAbs+"/")
	if idx := strings.IndexRune(instance, '/'); idx > -1 {
		instance = instance[:idx]
	}
	return instance
}

func RegisterPflags(fs *pflag.FlagSet) {
	fs.StringVarP(&instance,
		"instance",
		"i",
		instance,
		`instance, identified by hostname`)

	fs.StringVar(&parentDir,
		"parent_dir",
		parentDir,
		`parent directory: contains one subdirectory per instance`)

}

func SetInstance(i string) {
	instance = i
}

func SetParentDir(p string) {
	parentDir = p
}

func Instance() string {
	return instance
}

var parentDirOnce sync.Once

func ParentDir() string {
	parentDirOnce.Do(func() {
		if !strings.Contains(parentDir, "./") &&
			!strings.Contains(parentDir, "../") &&
			!strings.Contains(parentDir, "/..") {
			return
		}
		if abs, err := filepath.Abs(parentDir); err == nil {
			parentDir = abs
		}
	})
	return parentDir
}
