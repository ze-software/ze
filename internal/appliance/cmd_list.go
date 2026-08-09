// Design: docs/architecture/appliance/builder.md -- list appliances

package appliance

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

func init() {
	cmdList = runList
}

func runList(args []string) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance list\n")
		return exitOK
	}

	dir := getBaseDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "no appliances found (directory %s does not exist)\n", dir)
			return exitOK
		}
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", dir, err)
		return exitError
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tHOSTNAME\tARCH\tMANAGED") //nolint:errcheck // stdout

	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == sharedDirName || strings.HasPrefix(name, ".") {
			continue
		}
		cfg, loadErr := LoadConfig(ConfigPath(dir, name))
		if loadErr != nil {
			continue
		}
		managed := ""
		if cfg.Managed {
			managed = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, cfg.Identity.Hostname, cfg.Image.Arch, managed) //nolint:errcheck // tabwriter
		count++
	}
	w.Flush() //nolint:errcheck // stdout

	if count == 0 {
		fmt.Fprintf(os.Stderr, "no appliances found in %s\n", dir)
	}
	return exitOK
}
