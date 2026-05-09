// Design: plan/spec-appliance-1-builder.md — QEMU boot with port conflict detection

package appliance

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
)

func init() {
	cmdRun = runRun
}

func runRun(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: ze appliance run <name>\n")
		return exitError
	}
	name := args[0]
	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	ports := map[string]int{
		"ssh":     cfg.QEMU.SSHPort,
		"web":     cfg.QEMU.WebPort,
		"gokrazy": cfg.QEMU.GokrazyPort,
	}
	for label, port := range ports {
		if port == 0 {
			continue
		}
		if err := checkPortAvailable(port); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s port %d: %v\n", label, port, err)
			return exitError
		}
	}

	fmt.Fprintf(os.Stderr, "error: QEMU launch not yet implemented (ports verified: ssh=%d, web=%d, gokrazy=%d)\n",
		cfg.QEMU.SSHPort, cfg.QEMU.WebPort, cfg.QEMU.GokrazyPort)
	return exitError
}

func checkPortAvailable(port int) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf("port already in use")
	}
	ln.Close() //nolint:errcheck // probe only
	return nil
}
