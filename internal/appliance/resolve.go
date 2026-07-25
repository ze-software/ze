// Design: plan/learned/675-appliance-1-builder.md — appliance directory resolution

package appliance

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/env"
)

const (
	applianceDirKey  = "ze.appliance.dir"
	defaultSubdir    = "ze/appliances"
	configFileName   = "appliance.json"
	secretsDirName   = "secrets"
	tlsDirName       = "tls"
	databaseFileName = "database.zefs"
	sharedDirName    = "_shared"
)

var _ = env.MustRegister(env.EnvEntry{Key: applianceDirKey, Type: "string", Description: "Override appliance directory"})

func ResolveDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	if envDir := env.Get(applianceDirKey); envDir != "" {
		return envDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", defaultSubdir)
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, defaultSubdir)
}

func AppliancePath(baseDir, name string) string {
	return filepath.Join(baseDir, name)
}

func ConfigPath(baseDir, name string) string {
	return filepath.Join(baseDir, name, configFileName)
}

func SecretsDir(baseDir, name string) string {
	return filepath.Join(baseDir, name, secretsDirName)
}

func TLSDir(baseDir, name string) string {
	return filepath.Join(baseDir, name, secretsDirName, tlsDirName)
}

func DatabasePath(baseDir, name string) string {
	return filepath.Join(baseDir, name, databaseFileName)
}
