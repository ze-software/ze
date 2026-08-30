package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

type ipByteRow struct {
	IP    string `json:"ip"`
	Bytes uint64 `json:"bytes"`
}

func cmdJSONIPBytes(args []string) int {
	flags := flag.NewFlagSet("json-ip-bytes", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	ip := flags.String("ip", "", "IP address to total")
	arrays := flags.String("arrays", "ingress-ips,egress-ips", "comma-separated top-level arrays")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *ip == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "json-ip-bytes: --ip is required and positional arguments are refused")
		return 2
	}
	var document map[string]json.RawMessage
	if err := json.NewDecoder(os.Stdin).Decode(&document); err != nil {
		fmt.Fprintf(os.Stderr, "json-ip-bytes: decode: %v\n", err)
		return 1
	}
	var total uint64
	for name := range strings.SplitSeq(*arrays, ",") {
		var rows []ipByteRow
		if raw := document[strings.TrimSpace(name)]; raw != nil {
			if err := json.Unmarshal(raw, &rows); err != nil {
				fmt.Fprintf(os.Stderr, "json-ip-bytes: decode %s: %v\n", name, err)
				return 1
			}
		}
		for _, row := range rows {
			if row.IP == *ip {
				total += row.Bytes
			}
		}
	}
	if _, err := fmt.Fprintln(os.Stdout, total); err != nil {
		fmt.Fprintf(os.Stderr, "json-ip-bytes: write total: %v\n", err)
		return 1
	}
	return 0
}
