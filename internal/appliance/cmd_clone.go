// Design: docs/architecture/appliance/builder.md -- clone config (not secrets)

package appliance

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	cmdClone = runClone
}

func runClone(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: ze appliance clone <src> <dst>\n")
		return exitError
	}
	src, dst := args[0], args[1]
	dir := getBaseDir()

	srcCfg, err := LoadConfig(ConfigPath(dir, src))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	dstDir := AppliancePath(dir, dst)
	if _, err := os.Stat(dstDir); err == nil {
		fmt.Fprintf(os.Stderr, "error: appliance %q already exists\n", dst)
		return exitError
	}

	dstCfg := *srcCfg
	dstCfg.Identity.Name = dst
	dstCfg.Identity.Hostname = dst

	if err := os.MkdirAll(dstDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "error: create directory: %v\n", err)
		return exitError
	}

	if err := saveConfig(ConfigPath(dir, dst), &dstCfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	var tb textbuf.Buffer
	srcZeConf := tb.Str(AppliancePath(dir, src)).Str("/ze.conf").String()
	if data, readErr := os.ReadFile(srcZeConf); readErr == nil { //nolint:gosec // appliance file
		dstZeConf := tb.Reset().Str(AppliancePath(dir, dst)).Str("/ze.conf").String()
		if writeErr := os.WriteFile(dstZeConf, data, 0o644); writeErr != nil { //nolint:gosec // config file
			fmt.Fprintf(os.Stderr, "error: copy ze.conf: %v\n", writeErr)
			return exitError
		}
	}

	fmt.Printf("cloned %q -> %q (config only, run 'ze appliance init %s' to generate secrets)\n", src, dst, dst)
	return exitOK
}
