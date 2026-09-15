// Design: docs/architecture/core-design.md -- sysctl offline CLI
//
// Package sysctl provides the ze sysctl subcommand for inspecting and
// setting kernel tunables without a running daemon.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/suggest"
	sysctlreg "github.com/ze-software/ze/internal/core/sysctl"
)

// The subcommand names. show and set are the canonical CLI verbs, so their
// spelling comes from the verb registry (internal/component/command) and an
// operator meets one word on every surface. The rest are nouns this tool owns.
const (
	commandList            = "list"
	commandListProfiles    = "list-profiles"
	commandDescribe        = "describe"
	commandDescribeProfile = "describe-profile"
	commandShow            = command.VerbShow
	commandSet             = command.VerbSet
	commandHelp            = "help"
)

// Run executes the sysctl subcommand. Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	subcmd := args[0]
	subArgs := args[1:]

	if subcmd == commandHelp || subcmd == "-h" || subcmd == "--help" {
		usage()
		return 0
	}

	switch subcmd {
	case commandList:
		return cmdList(subArgs)
	case commandDescribe:
		return cmdDescribe(subArgs)
	case commandListProfiles:
		return cmdListProfiles(subArgs)
	case commandDescribeProfile:
		return cmdDescribeProfile(subArgs)
	case commandShow:
		return cmdShow(subArgs)
	case commandSet:
		return cmdSet(subArgs)
	}

	fmt.Fprintf(os.Stderr, "error: unknown sysctl subcommand: %s\n", subcmd)
	if s := suggest.Command(subcmd, []string{
		commandList, commandDescribe, commandListProfiles, commandDescribeProfile,
		commandShow, commandSet, commandHelp,
	}); s != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
	}
	usage()
	return 1
}

func cmdList(_ []string) int {
	all := sysctlreg.All()
	if len(all) == 0 {
		fmt.Println("no known sysctl keys registered")
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "KEY\tTYPE\tDESCRIPTION"); err != nil { //nolint:errcheck // output
		return 1
	}
	for _, k := range all {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", k.Name, typeName(k.Type), k.Description); err != nil { //nolint:errcheck // output
			return 1
		}
	}
	if err := w.Flush(); err != nil {
		return 1
	}
	return 0
}

func cmdDescribe(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "error: sysctl describe requires a key argument\n")
		return 1
	}
	key := args[0]

	k, ok := sysctlreg.Lookup(key)
	if !ok {
		k, ok = sysctlreg.MatchTemplate(key)
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown key: %s (not in known-keys registry)\n", key)
		return 1
	}

	type detail struct {
		Key         string `json:"key"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Min         int    `json:"min,omitempty"`
		Max         int    `json:"max,omitempty"`
		Platform    string `json:"platform"`
		Template    bool   `json:"template,omitempty"`
	}
	d := detail{
		Key:         key,
		Description: k.Description,
		Type:        typeName(k.Type),
		Platform:    platformName(k.Platform),
		Template:    k.Template,
	}
	if k.Type == sysctlreg.TypeIntRange {
		d.Min = k.Min
		d.Max = k.Max
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal describe result: %v\n", err)
		return 1
	}
	fmt.Println(string(data))
	return 0
}

func cmdListProfiles(_ []string) int {
	profiles := sysctlreg.AllProfiles()
	if len(profiles) == 0 {
		fmt.Println("no sysctl profiles registered")
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "PROFILE\tKEYS\tBUILTIN\tDESCRIPTION"); err != nil { //nolint:errcheck // output
		return 1
	}
	for _, p := range profiles {
		builtin := ""
		if p.Builtin {
			builtin = "yes"
		}
		if _, err := fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", p.Name, len(p.Settings), builtin, p.Description); err != nil { //nolint:errcheck // output
			return 1
		}
	}
	if err := w.Flush(); err != nil {
		return 1
	}
	return 0
}

func cmdDescribeProfile(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "error: sysctl describe-profile requires a profile name argument\n")
		return 1
	}
	name := args[0]

	p, ok := sysctlreg.LookupProfile(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown profile: %s\n", name)
		return 1
	}

	type settingEntry struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	type profileDetail struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Builtin     bool           `json:"builtin"`
		Settings    []settingEntry `json:"settings"`
	}
	settings := make([]settingEntry, 0, len(p.Settings))
	for _, s := range p.Settings {
		settings = append(settings, settingEntry{Key: s.Key, Value: s.Value})
	}
	d := profileDetail{
		Name:        p.Name,
		Description: p.Description,
		Builtin:     p.Builtin,
		Settings:    settings,
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal profile detail: %v\n", err)
		return 1
	}
	fmt.Println(string(data))
	return 0
}

func cmdShow(_ []string) int {
	fmt.Fprintf(os.Stderr, "error: 'show sysctl' requires a running daemon\n")
	fmt.Fprintf(os.Stderr, "hint: start ze, then use 'ze cli -c \"show sysctl\"' or the SSH CLI\n")
	fmt.Fprintf(os.Stderr, "hint: use 'ze sysctl list' to see known keys without a daemon\n")
	return 1
}

func cmdSet(_ []string) int {
	fmt.Fprintf(os.Stderr, "error: 'set sysctl' requires a running daemon\n")
	fmt.Fprintf(os.Stderr, "hint: start ze, then use 'ze cli -c \"set sysctl <key> <value>\"'\n")
	return 1
}

func typeName(t sysctlreg.ValueType) string {
	switch t {
	case sysctlreg.TypeBool:
		return "bool"
	case sysctlreg.TypeInt:
		return "int"
	case sysctlreg.TypeIntRange:
		return "int-range"
	}
	return "unknown"
}

func platformName(p sysctlreg.Platform) string {
	switch p {
	case sysctlreg.PlatformAll:
		return "all"
	case sysctlreg.PlatformLinux:
		return "linux"
	case sysctlreg.PlatformDarwin:
		return "darwin"
	}
	return "unknown"
}

func usage() {
	p := helpfmt.Page{
		Command:   "ze sysctl",
		ShortHelp: "inspect and manage kernel tunables",
		Usage:     []string{"ze sysctl <command> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Offline Commands (no daemon needed)", Entries: []helpfmt.HelpEntry{
				{Name: commandList, Desc: "List all known sysctl keys with descriptions"},
				{Name: "describe <key>", Desc: "Show detail for one known key"},
				{Name: commandListProfiles, Desc: "List all registered sysctl profiles"},
				{Name: "describe-profile <name>", Desc: "Show detail for one sysctl profile"},
			}},
			{Title: "Daemon Commands (requires running ze)", Entries: []helpfmt.HelpEntry{
				{Name: "show sysctl", Desc: "Show active keys (via daemon CLI)"},
				{Name: "set sysctl <key> <value>", Desc: "Set a transient value (via daemon CLI)"},
			}},
		},
		Examples: []string{
			"ze sysctl list",
			"ze sysctl describe net.ipv4.conf.all.forwarding",
			"ze sysctl list-profiles",
			"ze sysctl describe-profile dsr",
			"ze cli -c \"show sysctl\"      # requires running daemon",
			"ze cli -c \"set sysctl net.core.somaxconn 4096\"",
		},
	}
	p.WriteErr()
}
