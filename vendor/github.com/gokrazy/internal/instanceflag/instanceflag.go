package instanceflag

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gokrazy/internal/config"
	"github.com/spf13/pflag"
)

// ParentDirDefault returns the default value for the --parent_dir flag,
// i.e. $GOKRAZY_PARENT_DIR or os.UserHomeDir/gokrazy.
func ParentDirDefault() string {
	def := os.Getenv("GOKRAZY_PARENT_DIR")
	if def == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = fmt.Sprintf("os.UserHomeDir failed: %v", err)
		}
		def = filepath.Join(homeDir, "gokrazy")
	}
	return def
}

// InstanceDefault returns the default value for the -i flag,
// i.e. "hello" or the name of the gokrazy instance in $PWD, if any.
func InstanceDefault() string {
	def := os.Getenv("GOKRAZY_INSTANCE")
	if def == "" {
		def = instanceFromPWD()
	}
	if def == "" {
		def = "hello"
	}
	return def
}

func instanceFromPWD() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	wdAbs, err := filepath.Abs(wd)
	if err != nil {
		return ""
	}

	instanceName := filepath.Base(wdAbs)
	cfg, err := config.ReadFromFile(filepath.Join(wdAbs, "config.json"), instanceName)
	if err != nil {
		return ""
	}
	if cfg.Hostname != "" && len(cfg.Packages) > 0 {
		return wdAbs
	}

	return ""
}

// Flags contains command-line flag values related to the gokrazy instance.
type Flags struct {
	Name   string // --instance / -i
	Parent string // --parent_dir
}

func (f *Flags) InstanceName() string {
	return filepath.Base(f.Name)
}

// InstancePath returns the name of the directory
// containing the gokrazy config.json,
// e.g. /home/michael/gokrazy/scan2drive/config.json
func (f *Flags) InstancePath() string {
	if strings.ContainsRune(f.Name, os.PathSeparator) {
		// Relative or absolute instance path, return as-is.
		return f.Name
	}
	return filepath.Join(f.Parent, f.Name)
}

// InstanceConfigPath returns the full path to config.json.
func (f *Flags) InstanceConfigPath() string {
	return filepath.Join(f.InstancePath(), "config.json")
}

func RegisterPflags(fs *pflag.FlagSet) *Flags {
	f := &Flags{
		Parent: ParentDirDefault(),
		Name:   InstanceDefault(),
	}

	fs.StringVarP(&f.Name,
		"instance",
		"i",
		f.Name,
		`instance, identified by hostname`)

	fs.StringVar(&f.Parent,
		"parent_dir",
		f.Parent,
		`parent directory: contains one subdirectory per instance`)

	return f
}
