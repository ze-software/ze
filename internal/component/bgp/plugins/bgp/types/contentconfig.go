// Design: docs/architecture/core-design.md — shared BGP types

package types

import (
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/filter"
)

// ContentConfig controls HOW messages are formatted (encoding + format).
// Separated from message type subscriptions (WHAT) per API design.
type ContentConfig struct {
	Encoding   string                     // "json" | "text" (default: "text")
	Format     string                     // "parsed" | "raw" | "full" (default: "parsed")
	Attributes *bgpfilter.AttributeFilter // Which attrs to include (nil = all)
	NLRI       *bgpfilter.NLRIFilter      // Which address families to include (nil = all)
}

// WithDefaults returns a ContentConfig with default values applied.
func (c ContentConfig) WithDefaults() ContentConfig {
	if c.Encoding == "" {
		c.Encoding = "text"
	}
	if c.Format == "" {
		c.Format = "parsed"
	}
	return c
}
