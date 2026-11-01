// Design: docs/architecture/config/syntax.md — config variable substitution for dynamic peers
// Related: library_chains.go — compileLibraryChain, which marks a templated chain
// Related: peers.go — validateChainNames, which judges a templated member at load
// Related: ../reactor/reactor_dynamic.go — resolveDynamicPolicy, which resolves and compiles per peer

package bgpconfig

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/policy"
)

// chainVariables are the config variables a dynamic peer resolves in a chain
// member's name (reactor resolveFilterVars), each with the pattern its value
// takes: an AS number, or an address in either family.
var chainVariables = []struct{ name, value string }{
	{"$remote_as", `[0-9]+`},
	{"$local_as", `[0-9]+`},
	{"$remote_ip", `[0-9A-Fa-f.:]+`},
}

// isTemplatedMember reports whether a chain member names its filter through a
// config variable.
func isTemplatedMember(name string) bool { return strings.ContainsRune(name, '$') }

// templatedPattern answers the pattern every name the member can resolve to
// matches, or an error naming a `$` that starts no variable Ze resolves.
// The scan is bounded by the name's length.
func templatedPattern(name string) (*regexp.Regexp, error) {
	var pattern strings.Builder
	pattern.WriteString("^")
	rest := name
	for {
		at := strings.IndexByte(rest, '$')
		if at < 0 {
			pattern.WriteString(regexp.QuoteMeta(rest))
			break
		}
		pattern.WriteString(regexp.QuoteMeta(rest[:at]))
		rest = rest[at:]
		value, length := variableAt(rest)
		if length == 0 {
			return nil, fmt.Errorf("%q holds %s, which is not a variable Ze resolves; the variables are $remote_as, $local_as and $remote_ip",
				name, variableToken(rest))
		}
		pattern.WriteString(value)
		rest = rest[length:]
	}
	pattern.WriteString("$")
	return regexp.Compile(pattern.String())
}

// variableAt answers the value pattern and the length of the variable that
// text starts with, or a zero length when it starts none. A variable followed
// by a letter, digit or underscore is a longer, unknown token.
func variableAt(text string) (string, int) {
	for _, v := range chainVariables {
		if !strings.HasPrefix(text, v.name) {
			continue
		}
		if len(text) > len(v.name) {
			if isTokenByte(text[len(v.name)]) {
				continue
			}
		}
		return v.value, len(v.name)
	}
	return "", 0
}

// variableToken answers the `$`-led token text starts with, for an error.
func variableToken(text string) string {
	end := 1
	for end < len(text) && isTokenByte(text[end]) {
		end++
	}
	return text[:end]
}

func isTokenByte(c byte) bool {
	return c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9')
}

// checkTemplatedMember judges, at load, a chain member naming its filter
// through a config variable. Only a dynamic group resolves variables, from the
// peer's OPEN, so a static peer is refused. The name's variables must be ones
// Ze resolves, and some instance under the policy root must match the
// pattern: an unqualified name no instance can match is refused, as an
// unknown plain name is (policy.Registry.ValidateName). A qualified name no
// instance matches is an external plugin's filter, judged as one
// (checkExternalChainRef). Which instance a peer names, and whether the
// attachment admits it, is judged when the peer resolves the chain
// (templatedChain.CompileResolved).
func checkTemplatedMember(reg *policy.Registry, name string, dynamic bool, attachment, context string) error {
	if !dynamic {
		return fmt.Errorf("%s: %q names a filter through a config variable, which only a dynamic group resolves (from the peer's OPEN); "+
			"name the filter itself, or move the chain to a dynamic group", context, name)
	}
	pattern, err := templatedPattern(filterInstanceName(name))
	if err != nil {
		return fmt.Errorf("%s: %w", context, err)
	}
	for _, instance := range reg.Names() {
		if pattern.MatchString(instance) {
			return nil
		}
	}
	if strings.Contains(name, ":") {
		return checkExternalChainRef(name, attachment, context)
	}
	return fmt.Errorf("%s: %q names no filter: no instance under the %s root matches it once its variables are resolved; "+
		"define one, for instance policy { match %s { ... } } with the AS a peer announces in place of the variable",
		context, name, policy.RootName, name)
}

// templatedChain compiles, per dynamic peer, a template chain that names a
// member through a config variable (filterapi.TemplatedChain). It holds only
// comparable values, so two built from the same config are reflect.DeepEqual
// and a reload that changes nothing swaps nothing.
type templatedChain struct {
	reg        *policy.Registry
	bases      policychain.Bases
	attachment string
	context    string
	// members indexes the refs that named their filter through a variable.
	members []int
}

var _ filterapi.TemplatedChain = (*templatedChain)(nil)

// CompileResolved judges and compiles resolved, the template chain with the
// peer's variables replaced, as a static peer's chain is judged and compiled
// at load: each resolved member is canonicalized and bound to its stage
// (canonicalizeFilterRefs), checked (validateChainNames), and the chain is
// compiled (compileLibraryChain). A refusal answers the error and a policy
// whose library rejects every route, for the caller to install.
func (c *templatedChain) CompileResolved(resolved filterapi.Policy) (filterapi.Policy, error) {
	compiled, err := c.compile(resolved)
	if err == nil {
		return compiled, nil
	}
	closed, closeErr := policychain.Compile(c.attachment, c.reg, nil, policy.FinalReject, c.bases)
	if closeErr != nil {
		panic("BUG: bgpconfig: a chain of no member with final reject does not compile: " + closeErr.Error())
	}
	return resolved.WithLibrary(closed), err
}

func (c *templatedChain) compile(resolved filterapi.Policy) (filterapi.Policy, error) {
	refs := append([]policy.Ref(nil), resolved.Filters()...)
	var bound []bool
	if bindings := resolved.Bindings(); bindings != nil {
		bound = append([]bool(nil), bindings...)
	}
	for _, i := range c.members {
		if err := validateChainNames(c.reg, refs[i:i+1], false, c.attachment, c.context); err != nil {
			return resolved, err
		}
		canonical, binding := canonicalizeFilterRefs(refs[i:i+1], c.reg)
		refs[i] = canonical[0]
		if binding != nil {
			if bound == nil {
				bound = make([]bool, len(refs))
			}
			bound[i] = binding[0]
		}
	}
	return compileLibraryChain(resolved.WithBoundFilters(refs, bound), c.reg, c.bases, c.attachment, c.context)
}
