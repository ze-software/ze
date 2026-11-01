// Design: docs/architecture/core-design.md -- registered stage ownership of policy references.
// Related: register.go -- the loop-detection filter type and the loop registration.

package filter

import (
	"strings"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// LoopName names loop detection everywhere it is reached: its registry
// registration, the reactor's protocol-stage ingress step, and the owner
// prefix of a bound `loop:<instance>` chain member.
const LoopName = "loop"

// BindLoopDetection resolves a configured loop-detection instance to the
// engine's loop step, answering the ref under its owner-qualified name
// (`loop:<instance>`) and whether the loop step owns it. The caller MUST obtain
// policyType from that instance's config definition, not from the reference
// prefix. A ref of another type, or qualified by another owner, comes back
// unchanged and unbound, on the fail-closed external path. Loop detection is
// engine code (an RFC 4271 Section 9.1.2 and RFC 4456 Section 8 protocol
// check), so this is the only stage ownership a chain member can hold.
// Called during config resolution, never on the UPDATE path. Safe for
// concurrent use.
func BindLoopDetection(ref policy.Ref, policyType string) (policy.Ref, bool) {
	if policyType != loopFilterType {
		return ref, false
	}
	prefix, instance, qualified := strings.Cut(ref.Name, ":")
	if !qualified {
		instance = ref.Name
		prefix = ""
	}
	if qualified {
		if prefix != LoopName && prefix != loopFilterType {
			return ref, false
		}
	}
	if prefix != LoopName {
		var name textbuf.Buffer
		ref.Name = name.Str(LoopName).Byte(':').Str(instance).String()
	}
	return ref, true
}

// OwnsChainMembers reports whether the compiled-in plugin registered under
// pluginName answers a `<pluginName>:<instance>` chain member in process:
// only loop detection does (BindLoopDetection). Config load reads it to tell
// a compiled-in plugin that serves chain members from one that serves none.
func OwnsChainMembers(pluginName string) bool {
	return pluginName == LoopName
}
