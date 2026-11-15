// Design: docs/research/l2tpv2-ze-integration.md -- shared schema defaults at the transport boundary
// Related: internal/component/l2tp/ppp/yang/ze-ppp-lcp.yang -- the sole default authority

package pppconfig

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	gyang "github.com/openconfig/goyang/pkg/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	_ "github.com/ze-software/ze/internal/component/l2tp/ppp/yang"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/suggest"
)

var lcpDefaults = sync.OnceValues(loadLCPDefaults)

// ParseLCP reads one transport's lcp container. Missing leaves take their
// defaults from the shared YANG grouping, including when the block is absent.
// It never changes the provider's map. The result is safe to copy to a session.
func ParseLCP(transport string, raw any) (ppp.LCPConfig, error) {
	values, ok := raw.(map[string]any)
	if raw != nil {
		if !ok {
			return ppp.LCPConfig{}, opererr.New(transport+" lcp must be a configuration block",
				"PPP negotiation settings cannot be read from a scalar value",
				"delete "+transport+" lcp")
		}
	}
	defaults, err := lcpDefaults()
	if err != nil {
		return ppp.LCPConfig{}, lcpSchemaError(err)
	}
	names := [...]string{"restart-timer", "max-terminate", "max-configure", "max-failure"}
	// Four declared names and the first unknown bound this walk. Providers
	// bypassing the YANG parser must not turn a misspelling into a default.
	for name := range values {
		if !slices.Contains(names[:], name) {
			return ppp.LCPConfig{}, lcpUnknownSetting(transport, name, names, defaults)
		}
	}
	for i, name := range names {
		value, present := values[name]
		if !present {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return ppp.LCPConfig{}, lcpLeafError(transport, name, "must be an integer", defaults[i])
		}
		n, err := strconv.Atoi(text)
		if err != nil {
			return ppp.LCPConfig{}, lcpLeafError(transport, name, fmt.Sprintf("%q is not an integer", text), defaults[i])
		}
		minimum, maximum := lcpLeafBounds(i)
		if n < minimum {
			return ppp.LCPConfig{}, lcpLeafError(transport, name, fmt.Sprintf("%d is outside %d..%d", n, minimum, maximum), defaults[i])
		}
		if n > maximum {
			return ppp.LCPConfig{}, lcpLeafError(transport, name, fmt.Sprintf("%d is outside %d..%d", n, minimum, maximum), defaults[i])
		}
		defaults[i] = n
	}
	c, err := ppp.NewLCPConfig(time.Duration(defaults[0])*time.Millisecond, defaults[1], defaults[2], defaults[3])
	if err != nil {
		return ppp.LCPConfig{}, lcpSchemaError(err)
	}
	return c, nil
}

// lcpUnknownSetting reuses the command spelling policy and the schema defaults
// for a complete repair; an unrelated name receives the full valid-key list.
func lcpUnknownSetting(transport, name string, known [4]string, defaults [4]int) error {
	closest := suggest.Command(name, known[:])
	for i, candidate := range known {
		if candidate == closest {
			return opererr.New(fmt.Sprintf("%s lcp setting %q is unknown", transport, name),
				fmt.Sprintf("Use %s to configure this LCP setting", closest),
				fmt.Sprintf("set %s lcp %s %d", transport, closest, defaults[i]))
		}
	}
	return opererr.New(fmt.Sprintf("%s lcp setting %q is unknown", transport, name),
		"Valid LCP settings are "+strings.Join(known[:], ", "),
		"delete "+transport+" lcp")
}

// lcpLeafBounds validates integer input before converting milliseconds into a
// duration. The timer's position is the same one the default array uses.
func lcpLeafBounds(index int) (minimum, maximum int) {
	if index == 0 {
		return 100, 60000
	}
	return 1, 255
}

// lcpLeafError has the transport and schema default that internal validation
// lacks, so the repair is the complete command for this operator's root.
func lcpLeafError(transport, name, problem string, repair int) error {
	return opererr.New(fmt.Sprintf("%s lcp %s %s", transport, name, problem),
		"PPP cannot admit a session with an invalid negotiation setting",
		fmt.Sprintf("set %s lcp %s %d", transport, name, repair))
}

// lcpSchemaError distinguishes a missing or invalid build-time schema from
// operator input. No config command can install the missing module.
func lcpSchemaError(cause error) error {
	return opererr.Wrap(cause, "PPP LCP configuration schema is unavailable",
		"This Ze build cannot supply valid negotiation defaults for new sessions",
		"Install a Ze build containing the ze-ppp-lcp YANG module, then restart Ze")
}

// loadLCPDefaults reads the grouping itself, so neither transport duplicates
// a default and a missing module/default refuses startup instead of becoming 0.
func loadLCPDefaults() ([4]int, error) {
	var defaults [4]int
	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return defaults, err
	}
	if err := loader.LoadRegistered(); err != nil {
		return defaults, err
	}
	resolved, err := loader.Resolve()
	if err != nil {
		return defaults, err
	}
	module := resolved.GetModule("ze-ppp-lcp")
	if module == nil {
		return defaults, fmt.Errorf("shared PPP LCP schema is not registered")
	}
	for _, grouping := range module.Grouping {
		if grouping.Name != "lcp" {
			continue
		}
		container := gyang.ToEntry(grouping).Dir["lcp"]
		if container == nil {
			break
		}
		for i, name := range [...]string{"restart-timer", "max-terminate", "max-configure", "max-failure"} {
			leaf := container.Dir[name]
			if leaf == nil {
				return defaults, fmt.Errorf("shared PPP LCP schema has no %s leaf", name)
			}
			if len(leaf.Default) != 1 {
				return defaults, fmt.Errorf("shared PPP LCP schema has no single %s default", name)
			}
			n, err := strconv.Atoi(leaf.Default[0])
			if err != nil {
				return defaults, fmt.Errorf("shared PPP LCP %s default: %w", name, err)
			}
			minimum, maximum := lcpLeafBounds(i)
			if n < minimum {
				return defaults, fmt.Errorf("shared PPP LCP %s default %d is outside %d..%d", name, n, minimum, maximum)
			}
			if n > maximum {
				return defaults, fmt.Errorf("shared PPP LCP %s default %d is outside %d..%d", name, n, minimum, maximum)
			}
			defaults[i] = n
		}
		return defaults, nil
	}
	return defaults, fmt.Errorf("shared PPP LCP schema has no lcp grouping")
}
