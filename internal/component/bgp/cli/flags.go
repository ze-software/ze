// Design: docs/architecture/core-design.md — BGP CLI commands

package cli

import "github.com/ze-software/ze/internal/core/textbuf"

// pluginFlags collects multiple --plugin flag values.
type pluginFlags []string

func (p *pluginFlags) String() string {
	return textbuf.Join(*p, ",")
}

func (p *pluginFlags) Set(value string) error {
	*p = append(*p, value)
	return nil
}
