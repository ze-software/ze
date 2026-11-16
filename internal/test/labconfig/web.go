// Design: docs/architecture/testing/runner-architecture.md -- browser daemon fixture.

package labconfig

import "github.com/ze-software/ze/internal/core/textbuf"

// webDaemonConfig is the daemon browser runner's complete baseline: one peer
// that never connects and whose import/export policies explicitly reject all
// routes. The corpus MUST compose browser loads over this producer.
const webDaemonConfig = `bgp {
	router-id 1.2.3.4
	session {
		asn {
			local 65000
		}
	}
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.2
			}
			local {
				ip 127.0.0.1
				accept false
			}
		}
		session {
			asn {
				remote 65001
			}
		}
		filter {
			import {
				final reject
			}
			export {
				final reject
			}
		}
	}
}
`

// WebDaemon renders the actual browser daemon baseline and its parsed plugin
// declarations. Both the launching runner and config corpus MUST call it.
func WebDaemon(plugins []string) []byte {
	var body textbuf.Buffer
	if len(plugins) > 0 {
		body.Str("plugin {\n")
		for _, name := range plugins {
			body.Str("\tinternal ").Str(name).Str(" {\n\t\tuse ").Str(name).Str("\n\t}\n")
		}
		body.Str("}\n")
	}
	body.Str(webDaemonConfig)
	return []byte(body.String())
}
