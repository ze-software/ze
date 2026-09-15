// Design: docs/architecture/diagnostics/active-probes.md -- the Don't Fragment mode of a probe
// Related: socket.go -- OpenICMP, the one socket construction every prober calls
// Related: socket_linux.go, socket_other.go -- the platform half of the mode

package probe

import "errors"

// DFMode says whether a probe datagram carries the Don't Fragment bit, and
// when it does, whether the kernel's cached path MTU for the destination is
// honored or bypassed. The zero value is DFUnspecified, so a caller that never
// chose a mode is refused by OpenICMP rather than silently probing without DF.
type DFMode uint8

const (
	// DFUnspecified is the zero value and is never a valid mode.
	DFUnspecified DFMode = iota
	// DFOff sends the datagram with the DF bit clear. The kernel fragments a
	// datagram larger than the path, and no path-MTU information comes back.
	DFOff
	// DFHonorCache sets the DF bit and honors the kernel's cached path MTU: a
	// datagram larger than the cached value is refused at send time with
	// EMSGSIZE, and a router's Fragmentation Needed answer updates the cache.
	// This is Linux IP_PMTUDISC_DO.
	DFHonorCache
	// DFBypassCache sets the DF bit and ignores the cached path MTU, so the
	// datagram is put on the wire at its full size and the path answers for
	// itself. This is Linux IP_PMTUDISC_PROBE, the mode tracepath measures in.
	DFBypassCache
)

// String names the mode for a log line or an error message.
func (m DFMode) String() string {
	switch m {
	case DFOff:
		return "off"
	case DFHonorCache:
		return "honor-cache"
	case DFBypassCache:
		return "bypass-cache"
	default:
		return nameUnspecified
	}
}

// The CLI spelling of the mode. DFKeyword is the keyword on the ping and
// traceroute grammars, and it takes exactly one of the two value words after
// it. The bare keyword is refused: the RPC layer reads every declared leaf as
// keyword-then-value (internal/component/plugin/server/command.go), so a bare
// form could never reach a handler, and the handlers refuse it the same way so
// the offline local parsers agree with the daemon. The YANG leaves in
// ze-ping-cmd.yang and ze-traceroute-cmd.yang declare the same words.
const (
	DFKeyword          = "do-not-fragment"
	dfValueHonorCache  = "honor-cache"
	dfValueBypassCache = "bypass-cache"
)

// ErrDFValueUnknown is what DFModeOfValue answers for a word that is neither
// value, the empty word included. The message names both values so the
// operator's next attempt is spelled for them.
var ErrDFValueUnknown = errors.New("do-not-fragment takes one value: honor-cache or bypass-cache")

// DFModeOfValue maps the value word after the do-not-fragment keyword to its
// mode, and refuses any other word with ErrDFValueUnknown.
func DFModeOfValue(word string) (DFMode, error) {
	switch word {
	case dfValueHonorCache:
		return DFHonorCache, nil
	case dfValueBypassCache:
		return DFBypassCache, nil
	default:
		return DFUnspecified, ErrDFValueUnknown
	}
}

// ErrDFUnspecified is what OpenICMP answers when the caller passed the zero
// DFMode. It is a Ze defect, never an operator input: every call site names
// its mode.
var ErrDFUnspecified = errors.New("probe: do-not-fragment mode is unspecified")

// ErrDFUnsupported is what OpenICMP answers on a platform whose socket layer
// carries no IP_MTU_DISCOVER, when a mode other than DFOff is requested. A
// probe with DF off still opens there, exactly as before the mode existed.
var ErrDFUnsupported = errors.New("probe: do-not-fragment is not supported on this platform")
