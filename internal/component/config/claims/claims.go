// Design: docs/architecture/config/yang-config-design.md — config claim model
// Related: schema.go -- builds the merged config-schema tree Audit walks
// Related: allowlist.json -- recorded exceptions, one reason and one owner each
//
// Package claims answers one question about the config surface: is every config
// subtree an operator can write delivered to something that reads it?
//
// The daemon claims config per declared path. Server.reloadConfig selects the
// plugins whose Registration.WantsConfigRoots match the changed paths
// (internal/component/plugin/server/reload.go), and Hub.RouteCommand resolves a
// path to a subsystem through SchemaRegistry.FindHandler
// (internal/component/plugin/server/hub.go). A path matched by neither is
// accepted, stored, and delivered nowhere: reloadConfig logs Info "config
// reload: no affected plugins, updating config" and calls SetConfigTree.
//
// This package holds the claim semantics and the audit. It holds no inventory:
// the schema comes from the YANG loader, the claims come from the plugin
// registry and the schema registry, and the caller supplies both.
package claims

import (
	_ "embed"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// wildcardClaim is the declared root meaning "every root in this reload".
// rootHasChanges (reload.go) gives it that meaning, and expandWildcardRoots
// (reload_tx.go) resolves it to the concrete roots of the diff.
const wildcardClaim = "*"

// AllowlistPath names the allowlist source, so a failure can tell the reader
// where a new exception is recorded.
const AllowlistPath = "internal/component/config/claims/allowlist.json"

// hubHandlerSource labels a claim that arrives through the schema registry.
const hubHandlerSource = "hub-handler"

//go:embed allowlist.json
var allowlistJSON []byte

var errNilLoader = errors.New("claims: nil YANG loader")

// Node is one node of the merged config-schema tree. Several YANG modules can
// contribute to one node (five modules add children under "system"), so Modules
// is a set rather than an owner.
type Node struct {
	Path     string
	IsLeaf   bool
	Modules  []string
	Children map[string]*Node
}

// leafCount returns the number of config leaves in this subtree. A subtree with
// no leaves carries no operator data, which changes what an unclaimed finding
// means to the reader.
func (n *Node) leafCount() int {
	if n == nil {
		return 0
	}
	if n.IsLeaf {
		return 1
	}
	total := 0
	for _, c := range n.Children {
		total += c.leafCount()
	}
	return total
}

// lookup resolves a config path to its node, or nil.
func (n *Node) lookup(path string) *Node {
	cur := n
	for seg := range strings.SplitSeq(path, config.PathSep) {
		if cur == nil {
			return nil
		}
		cur = cur.Children[seg]
	}
	return cur
}

// Claim is one delivery declaration: a config path that some surface receives.
type Claim struct {
	Path   string // "bgp", "ddos/local", or "*"
	Source string // "plugin:bgp-rib", "hub-handler"
}

// Allow is one recorded exception: a config path delivered by a surface this
// package cannot enumerate, or a container that deliberately reaches nobody.
// Reason and Owner are both required. An entry without them is a finding, not
// an exception.
type Allow struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	Owner  string `json:"owner"`
}

// Kind classifies a finding.
type Kind string

const (
	// KindUnclaimed is a config subtree no claim covers.
	KindUnclaimed Kind = "unclaimed-subtree"
	// KindPhantomClaim is a claim that names no node in the config schema.
	KindPhantomClaim Kind = "phantom-claim"
	// KindAllowlistNoReason is an allowlist entry missing its path, reason, or owner.
	KindAllowlistNoReason Kind = "allowlist-missing-reason"
	// KindAllowlistStale is an allowlist entry whose path is now claimed, or
	// names no schema node.
	KindAllowlistStale Kind = "allowlist-stale"
	// KindUnclassifiable is an input the audit cannot judge. It is reported
	// rather than passed: a gate that cannot see its subject has not cleared it.
	KindUnclassifiable Kind = "unclassifiable"
)

// Finding is one reason the audit failed.
type Finding struct {
	Kind   Kind
	Path   string
	Detail string
}

// String renders a finding as one line for a test failure or a log.
func (f Finding) String() string {
	var tb textbuf.Buffer
	tb.Str(string(f.Kind)).Str(": ")
	if f.Path != "" {
		tb.Str(f.Path).Str(": ")
	}
	return tb.Str(f.Detail).String()
}

// Report is the audit result.
type Report struct {
	Findings    []Finding
	Allowlisted []string
	NodesWalked int
}

// Failed reports whether the audit found anything.
func (r Report) Failed() bool { return len(r.Findings) > 0 }

// Allowlist returns the recorded exceptions. The JSON is embedded, so the
// allowlist travels with the binary and the doctor check reads the same entries
// the build-time gate does.
func Allowlist() ([]Allow, error) {
	var out []Allow
	if err := json.Unmarshal(allowlistJSON, &out); err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("parse ").Str(AllowlistPath).Str(": ").Err(err).String())
	}
	return out, nil
}

// FromConfigRoots converts registry.ConfigRootsMap() into claims. The map is
// plugin name to declared roots; Stage 1 registration copies those roots into
// Registration.WantsConfigRoots (server/startup.go), which is what reloadConfig
// matches against.
func FromConfigRoots(m map[string][]string) []Claim {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)

	var tb textbuf.Buffer
	out := make([]Claim, 0, len(m))
	for _, name := range names {
		source := tb.Reset().Str("plugin:").Str(name).String()
		for _, root := range m[name] {
			out = append(out, Claim{Path: root, Source: source})
		}
	}
	return out
}

// FromHubHandlers converts schema-registry handler paths into claims. A path
// under a registered handler reaches a subsystem through Hub.RouteCommand even
// when no plugin declares it as a config root.
func FromHubHandlers(paths []string) []Claim {
	out := make([]Claim, 0, len(paths))
	for _, p := range paths {
		out = append(out, Claim{Path: p, Source: hubHandlerSource})
	}
	return out
}

// covers reports whether a claim path delivers a config path. Same rule as
// rootHasChanges (internal/component/plugin/server/reload.go): the wildcard
// takes everything, and a claim takes its own path plus everything below it.
func covers(claim, path string) bool {
	if claim == wildcardClaim {
		return true
	}
	if claim == path {
		return true
	}
	var tb textbuf.Buffer
	return strings.HasPrefix(path, tb.Str(claim).Str(config.PathSep).String())
}

// Audit compares the config schema against the claim union.
//
// It fails closed. An empty tree, an empty claim set, a claim with no path, or
// an allowlist entry it cannot judge each produce a finding: an audit that
// cannot see its subject reports that, and never returns a clean report.
func Audit(root *Node, cs []Claim, allow []Allow) Report {
	var r Report

	if root == nil || len(root.Children) == 0 {
		r.Findings = append(r.Findings, Finding{
			Kind:   KindUnclassifiable,
			Detail: "config schema tree is empty: no config-schema module resolved, so no subtree was checked",
		})
		return r
	}
	if len(cs) == 0 {
		r.Findings = append(r.Findings, Finding{
			Kind:   KindUnclassifiable,
			Detail: "no claims supplied: every config subtree would look unclaimed, which says nothing about the tree",
		})
		return r
	}

	var tb textbuf.Buffer
	claimPaths := make([]string, 0, len(cs))
	for _, c := range cs {
		if c.Path == "" {
			r.Findings = append(r.Findings, Finding{
				Kind: KindUnclassifiable,
				Detail: tb.Reset().Str("claim from ").Str(c.Source).
					Str(" has an empty path: it delivers nothing, and it hides nothing").String(),
			})
			continue
		}
		claimPaths = append(claimPaths, c.Path)
	}

	allowed := auditAllowlist(&r, root, allow, claimPaths)
	auditPhantomClaims(&r, root, cs)
	walkUnclaimed(&r, root, claimPaths, allowed)

	sort.Slice(r.Findings, func(i, j int) bool {
		if r.Findings[i].Kind != r.Findings[j].Kind {
			return r.Findings[i].Kind < r.Findings[j].Kind
		}
		return r.Findings[i].Path < r.Findings[j].Path
	})
	sort.Strings(r.Allowlisted)
	return r
}

// AuditConfigured reports the parts of an operator's config tree that reach no
// claim. It runs the unclaimed walk only.
//
// Audit judges the SCHEMA: every subtree an operator could write. This judges
// one config: what this operator did write, on a daemon built with these
// plugins. The phantom-claim and allowlist-hygiene findings do not apply, since
// a claim naming a root the operator left out is not a defect.
func AuditConfigured(root *Node, cs []Claim, allow []Allow) []Finding {
	if root == nil || len(root.Children) == 0 || len(cs) == 0 {
		return nil
	}
	claimPaths := make([]string, 0, len(cs))
	for _, c := range cs {
		if c.Path != "" {
			claimPaths = append(claimPaths, c.Path)
		}
	}
	allowed := make(map[string]bool, len(allow))
	for _, a := range allow {
		if a.Path != "" && strings.TrimSpace(a.Reason) != "" && strings.TrimSpace(a.Owner) != "" {
			allowed[a.Path] = true
		}
	}

	var r Report
	walkUnclaimed(&r, root, claimPaths, allowed)
	sort.Slice(r.Findings, func(i, j int) bool { return r.Findings[i].Path < r.Findings[j].Path })
	return r.Findings
}

// auditAllowlist validates every recorded exception and returns the paths that
// survived. An entry that is not usable never suppresses a finding: the audit
// reports the entry AND still reports the subtree it failed to cover.
func auditAllowlist(r *Report, root *Node, allow []Allow, claimPaths []string) map[string]bool {
	var tb textbuf.Buffer
	allowed := make(map[string]bool, len(allow))

	for _, a := range allow {
		switch {
		case a.Path == "":
			r.Findings = append(r.Findings, Finding{
				Kind:   KindAllowlistNoReason,
				Detail: "allowlist entry has no path",
			})
			continue
		case strings.TrimSpace(a.Reason) == "":
			r.Findings = append(r.Findings, Finding{
				Kind: KindAllowlistNoReason,
				Path: a.Path,
				Detail: tb.Reset().Str("allowlist entry has no reason: say why this subtree reaches no plugin, in ").
					Str(AllowlistPath).String(),
			})
			continue
		case strings.TrimSpace(a.Owner) == "":
			r.Findings = append(r.Findings, Finding{
				Kind: KindAllowlistNoReason,
				Path: a.Path,
				Detail: tb.Reset().Str("allowlist entry has no owner: name the file and the symbol that reads this subtree, in ").
					Str(AllowlistPath).String(),
			})
			continue
		}

		if root.lookup(a.Path) == nil {
			r.Findings = append(r.Findings, Finding{
				Kind: KindAllowlistStale,
				Path: a.Path,
				Detail: tb.Reset().Str("allowlist entry names no config schema node: the module was removed, or the path was renamed. Delete the entry from ").
					Str(AllowlistPath).String(),
			})
			continue
		}
		for _, c := range claimPaths {
			if covers(c, a.Path) {
				r.Findings = append(r.Findings, Finding{
					Kind: KindAllowlistStale,
					Path: a.Path,
					Detail: tb.Reset().Str("allowlist entry is now claimed by ").Str(c).
						Str(": delete the entry from ").Str(AllowlistPath).String(),
				})
				break
			}
		}

		allowed[a.Path] = true
		r.Allowlisted = append(r.Allowlisted, a.Path)
	}
	return allowed
}

// auditPhantomClaims reports a claim that names no config schema node. Such a
// claim never matches in rootHasChanges, so the plugin that declared it is
// never selected and never receives the config it asked for.
func auditPhantomClaims(r *Report, root *Node, cs []Claim) {
	var tb textbuf.Buffer
	seen := make(map[string]bool, len(cs))

	for _, c := range cs {
		if c.Path == "" || c.Path == wildcardClaim {
			continue
		}
		key := tb.Reset().Str(c.Path).Byte(0).Str(c.Source).String()
		if seen[key] {
			continue
		}
		seen[key] = true
		if root.lookup(c.Path) != nil {
			continue
		}
		r.Findings = append(r.Findings, Finding{
			Kind: KindPhantomClaim,
			Path: c.Path,
			Detail: tb.Reset().Str(c.Source).
				Str(" claims a config path that no YANG config module defines: ").
				Str(nearestNodeHint(root, c.Path)).String(),
		})
	}
}

// walkUnclaimed descends the config schema and reports the highest node no
// claim covers. It descends through a node that is not covered but has a claim
// below it, so a container that exists only to carry claimed children is not
// itself a finding, while a leaf beside those children still is.
func walkUnclaimed(r *Report, root *Node, claimPaths []string, allowed map[string]bool) {
	var walk func(n *Node)
	walk = func(n *Node) {
		for _, name := range childNames(n) {
			c := n.Children[name]
			r.NodesWalked++

			if allowed[c.Path] {
				continue
			}
			if anyCovers(claimPaths, c.Path) {
				continue
			}
			if hasClaimBelow(claimPaths, c.Path) {
				walk(c)
				continue
			}

			// A leafless container is still reported. Presence is data: a
			// container whose only content is its own existence enables a
			// feature, and one that reaches nobody enables nothing. Skipping
			// it on a zero leaf count would let the emptiest config surface be
			// the one nothing checks.
			leaves := c.leafCount()
			r.Findings = append(r.Findings, Finding{
				Kind:   KindUnclaimed,
				Path:   c.Path,
				Detail: describeUnclaimed(c, leaves, claimPaths),
			})
		}
	}
	walk(root)
}

func describeUnclaimed(n *Node, leaves int, claimPaths []string) string {
	var tb textbuf.Buffer
	return tb.Str("no plugin config root and no hub handler covers this subtree (").
		Int(int64(leaves)).Str(" config leaves, from ").Join(n.Modules, ", ").
		Str("). Config written here is stored and delivered to nobody. Add the path to the owning plugin's ConfigRoots, or record the consumer in ").
		Str(AllowlistPath).
		Str(". Nearest existing claim: ").Str(nearestClaim(claimPaths, n.Path)).String()
}

func anyCovers(claimPaths []string, path string) bool {
	for _, c := range claimPaths {
		if covers(c, path) {
			return true
		}
	}
	return false
}

func hasClaimBelow(claimPaths []string, path string) bool {
	var tb textbuf.Buffer
	prefix := tb.Str(path).Str(config.PathSep).String()
	for _, c := range claimPaths {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

// nearestClaim names the claim sharing the longest path-segment prefix with
// path. AC-1 asks for it because the usual cause of an unclaimed subtree is a
// claim one segment away.
func nearestClaim(claimPaths []string, path string) string {
	best, bestShared := "", 0
	want := strings.Split(path, config.PathSep)
	for _, c := range claimPaths {
		got := strings.Split(c, config.PathSep)
		shared := 0
		for shared < len(want) && shared < len(got) && want[shared] == got[shared] {
			shared++
		}
		if shared > bestShared || (shared == bestShared && best != "" && c < best) {
			best, bestShared = c, shared
		}
	}
	if bestShared == 0 {
		return "none (no claim shares a path segment)"
	}
	return best
}

// nearestNodeHint names the deepest real schema node on a phantom claim's path,
// which is where the typo starts.
func nearestNodeHint(root *Node, path string) string {
	cur := root
	depth := 0
	segs := strings.Split(path, config.PathSep)
	for _, seg := range segs {
		next := cur.Children[seg]
		if next == nil {
			break
		}
		cur = next
		depth++
	}

	var tb textbuf.Buffer
	if depth == 0 {
		return tb.Str("no segment of it exists; the top-level config roots are ").
			Join(childNames(root), ", ").String()
	}
	return tb.Str("the path exists as far as ").Join(segs[:depth], config.PathSep).
		Str(", whose children are ").Join(childNames(cur), ", ").String()
}

// childNames returns a node's child names, sorted, so every walk and every
// message lists them in one order.
func childNames(n *Node) []string {
	names := make([]string, 0, len(n.Children))
	for name := range n.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
