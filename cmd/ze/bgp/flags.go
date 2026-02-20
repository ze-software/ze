// Design: docs/architecture/core-design.md — BGP CLI commands

package bgp

import "strings"

// pluginFlags collects multiple --plugin flag values.
type pluginFlags []string

func (p *pluginFlags) String() string {
	return strings.Join(*p, ",")
}

func (p *pluginFlags) Set(value string) error {
	*p = append(*p, value)
	return nil
}
