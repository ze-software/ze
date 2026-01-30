package config

// ConfigType represents the type of configuration detected.
type ConfigType string

// Config type constants.
const (
	ConfigTypeBGP     ConfigType = "bgp"
	ConfigTypeHub     ConfigType = "hub"
	ConfigTypeUnknown ConfigType = "unknown"
)

// ProbeConfigType scans config content for top-level blocks without full parsing.
// Returns ConfigTypeBGP for bgp {} block, ConfigTypeHub for plugin {}, ConfigTypeUnknown otherwise.
// BGP takes precedence if both blocks are present.
func ProbeConfigType(content string) ConfigType {
	tok := NewTokenizer(content)

	hasBGP := false
	hasPlugin := false
	depth := 0

	for {
		t := tok.Next()
		if t.Type == TokenEOF {
			break
		}

		switch t.Type { //nolint:exhaustive // Only care about braces and words
		case TokenLBrace:
			depth++
		case TokenRBrace:
			depth--
		case TokenWord:
			if depth == 0 {
				next := tok.Peek()
				if next.Type == TokenLBrace {
					switch t.Value {
					case "bgp":
						hasBGP = true
					case "plugin":
						hasPlugin = true
					}
				}
			}
		}
	}

	if hasBGP {
		return ConfigTypeBGP
	}
	if hasPlugin {
		return ConfigTypeHub
	}
	return ConfigTypeUnknown
}
