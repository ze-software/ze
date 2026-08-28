package fixture

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	registerFixture("ui/cli-data-config-dir", runUIFixtureCLIDataConfigDir)
}

func runUIFixtureCLIDataConfigDir() error {
	initDir, err := os.MkdirTemp("", "ze-data-config-dir-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(initDir)

	env := uiCliDataConfigDirSetFixtureEnv(os.Environ(), "ZE_CONFIG_DIR", initDir)

	// ze init writes the store through the env-honoring resolver.
	initCmd := exec.Command("ze", "init")
	initCmd.Env = env
	initCmd.Stdin = strings.NewReader("admin\nsecret123\n127.0.0.1\n2222\n")
	initCmd.Stdout = io.Discard
	initCmd.Stderr = os.Stderr
	if err := initCmd.Run(); err != nil {
		return err
	}

	databasePath := filepath.Join(initDir, "database.zefs")
	info, statErr := os.Stat(databasePath)
	if statErr != nil || !info.Mode().IsRegular() {
		fmt.Fprintf(os.Stderr, "FAIL: ze init did not create %s\n", databasePath)
		return errors.New("fixture failed")
	}

	// ze data must resolve that same store rather than one derived from its own location.
	checkCmd := exec.Command("ze", "data", "check")
	checkCmd.Env = env
	checkCmd.Stdin = os.Stdin
	checkCmd.Stdout = os.Stdout
	checkCmd.Stderr = os.Stderr
	return checkCmd.Run()
}

func uiCliDataConfigDirSetFixtureEnv(env []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}
