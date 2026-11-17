// Design: docs/architecture/container/network.md -- the port-blocked doctor check
// Related: verify.go -- the parse that reads the operator's forward chains
// Related: apply.go -- the apply that records what the check judges
// Related: tables.go -- the translation terms whose traffic the policy must accept
// Related: register.go -- the registration of the check and of its code

package network

import (
	"maps"
	"net/netip"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/configorder"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The doctor check of a published port the operator's firewall policy blocks,
// and its one code.
const (
	checkPortBlocked = "container-port-blocked"
	codePortBlocked  = "doctor-container-port-blocked"
	// orderPortBlocked follows the runtime's and the store's container checks
	// (791 to 797).
	orderPortBlocked = 798
	// severityPortBlocked: the container runs and the port is published, and
	// a client beyond the router cannot reach it.
	severityPortBlocked = "warning"
)

// The words of the firewall schema the check reads and its fix line writes
// (internal/component/firewall/yang/ze-firewall-conf.yang). The accept action
// of a then block is the word chainPolicyAccept names.
const (
	leafFamily             = "family"
	listChain              = "chain"
	listTerm               = "term"
	leafHook               = "hook"
	leafPolicy             = "policy"
	leafOutputInterface    = "output-interface"
	leafDestinationAddress = "destination-address"
	leafDestinationPort    = "destination-port"
	familyInet             = "inet"
	hookForward            = "forward"
	policyDrop             = "drop"
	interfaceWildcard      = "*"
	// fixTermPrefix starts the name of the term the fix line adds:
	// container_INSTANCE_PORT. No instance or port name holds an underscore,
	// so two such names never collide.
	fixTermPrefix = "container_"
)

// ForwardChain is one base chain of the operator's firewall config at the
// forward hook whose policy is drop: a chain that drops every forwarded
// packet none of its terms accepts. A connection from beyond the router to a
// published port is forwarded after its translation, so it crosses each such
// chain. It holds no func and no channel, as Config requires.
type ForwardChain struct {
	Table string
	Chain string
	// IPv4 and IPv6 say which families the chain's table carries. A table of
	// another family (arp, bridge, netdev) carries neither: a routed
	// connection does not cross it.
	IPv4 bool
	IPv6 bool
	// Accepts are the chain's accepting terms that name an output interface
	// or a destination prefix, in the chain's order.
	Accepts []ForwardAccept
}

// ForwardAccept is what the check reads of one accepting term: the two
// matches the judgment is made on, and nothing else of the term.
type ForwardAccept struct {
	// OutputInterface is the term's output-interface match as the operator
	// wrote it, a trailing asterisk included. It is empty when the term has
	// no such match: that emptiness is the guard namesBridge reads.
	OutputInterface string
	// Destination is the term's destination-address prefix. It is the invalid
	// zero prefix when the term has no such match, or names a set: that
	// invalidity is the guard namesAddress reads.
	Destination netip.Prefix
}

// parseForwardChains reads the operator's firewall config for the chains the
// port-blocked check judges, sorted by table then chain so two parses of one
// config compare equal. It judges nothing else of the firewall config: the
// firewall component verifies it. A term list whose order was not delivered
// is refused, as the firewall component refuses it.
func parseForwardChains(firewall map[string]any) ([]ForwardChain, error) {
	tables, held := firewall[sectionTableKey].(map[string]any)
	if !held {
		return nil, nil
	}
	var chains []ForwardChain
	for _, table := range slices.Sorted(maps.Keys(tables)) {
		fields, isObject := tables[table].(map[string]any)
		if !isObject {
			continue
		}
		found, err := tableForwardChains(table, fields)
		if err != nil {
			return nil, err
		}
		chains = append(chains, found...)
	}
	return chains, nil
}

// tableForwardChains answers the dropping forward chains of one table.
func tableForwardChains(table string, fields map[string]any) ([]ForwardChain, error) {
	held, hasChains := fields[listChain].(map[string]any)
	if !hasChains {
		return nil, nil
	}
	ipv4, ipv6 := tableFamilies(fields[leafFamily])
	var chains []ForwardChain
	for _, name := range slices.Sorted(maps.Keys(held)) {
		chain, isObject := held[name].(map[string]any)
		if !isObject {
			continue
		}
		if !dropsAtForward(chain) {
			continue
		}
		accepts, err := forwardAccepts(chain)
		if err != nil {
			return nil, opererr.Wrap(err,
				"the terms of chain "+name+" of firewall table "+table+" cannot be read in their order",
				"a published container port is judged against the accepting terms of each forward chain that drops",
				"ze show config "+firewallRoot,
			)
		}
		chains = append(chains, ForwardChain{Table: table, Chain: name, IPv4: ipv4, IPv6: ipv6, Accepts: accepts})
	}
	return chains, nil
}

// tableFamilies answers which of the two routed families a table of the
// family leaf carries. The set of family words is the firewall schema's, open
// to this package: a word it does not list carries neither family here.
func tableFamilies(family any) (ipv4, ipv6 bool) {
	switch family {
	case familyInet:
		return true, true
	case familyIPv4:
		return true, false
	case familyIPv6:
		return false, true
	}
	return false, false
}

// dropsAtForward answers whether chain is a base chain at the forward hook
// whose policy is drop.
func dropsAtForward(chain map[string]any) bool {
	if chain[leafHook] != hookForward {
		return false
	}
	return chain[leafPolicy] == policyDrop
}

// forwardAccepts answers the accepting terms of chain that name an output
// interface or a destination prefix.
func forwardAccepts(chain map[string]any) ([]ForwardAccept, error) {
	terms, err := configorder.Entries(chain, listTerm, leafName)
	if err != nil {
		return nil, err
	}
	var accepts []ForwardAccept
	for _, term := range terms {
		then, hasThen := term.Map[leafThen].(map[string]any)
		if !hasThen {
			continue
		}
		if _, accepting := then[chainPolicyAccept]; !accepting {
			continue
		}
		from, hasFrom := term.Map[leafFrom].(map[string]any)
		if !hasFrom {
			continue
		}
		accept, named := acceptOf(from)
		if named {
			accepts = append(accepts, accept)
		}
	}
	return accepts, nil
}

// acceptOf reads the two matches of a from block, and answers whether it
// holds one the check can judge. A destination that names a set (@NAME) is
// not judged: the check reads no set.
func acceptOf(from map[string]any) (ForwardAccept, bool) {
	var accept ForwardAccept
	named := false
	if device, isWord := from[leafOutputInterface].(string); isWord && device != "" {
		accept.OutputInterface = device
		named = true
	}
	if written, isWord := from[leafDestinationAddress].(string); isWord {
		if prefix, err := netip.ParsePrefix(written); err == nil {
			accept.Destination = prefix
			named = true
		}
	}
	return accept, named
}

// namesBridge answers whether the term's output-interface match names bridge:
// the same name, or a prefix of it before a trailing asterisk.
func (a ForwardAccept) namesBridge(bridge string) bool {
	if a.OutputInterface == "" {
		return false
	}
	prefix, wildcard := strings.CutSuffix(a.OutputInterface, interfaceWildcard)
	if wildcard {
		return strings.HasPrefix(bridge, prefix)
	}
	return a.OutputInterface == bridge
}

// namesAddress answers whether the term's destination prefix holds address.
func (a ForwardAccept) namesAddress(address netip.Addr) bool {
	if !a.Destination.IsValid() {
		return false
	}
	return a.Destination.Contains(address)
}

// carries answers whether the chain's table carries the family of address.
func (c ForwardChain) carries(address netip.Addr) bool {
	if address.Is4() {
		return c.IPv4
	}
	return c.IPv6
}

// blocks answers whether the chain drops a forwarded connection to one of
// the port's targets: the chain's table carries the target's family and no
// accepting term names the port's bridge as output interface or the target
// as destination. It judges those two matches only, so it reports a port a
// term accepts by another match, and misses a drop term ahead of the
// accepting one.
func (c ForwardChain) blocks(port publishedPort) bool {
	for _, target := range port.targets {
		if !c.carries(target) {
			continue
		}
		if !c.accepts(port.bridge, target) {
			return true
		}
	}
	return false
}

// accepts answers whether a term of the chain names bridge or target.
func (c ForwardChain) accepts(bridge string, target netip.Addr) bool {
	for _, accept := range c.Accepts {
		if accept.namesBridge(bridge) {
			return true
		}
		if accept.namesAddress(target) {
			return true
		}
	}
	return false
}

// publishedPort is one port entry of an enabled instance, as the apply
// translated it: what the port-blocked check judges.
type publishedPort struct {
	instance string
	port     string
	protocol string
	// containerPorts is the destination port a forwarded packet carries:
	// the translation rewrote the listen port before the forward hook.
	containerPorts portRange
	bridge         string
	// targets are the instance's addresses on its first network the port is
	// translated to, one per family the port is published in.
	targets []netip.Addr
}

// portPolicy is what one apply recorded for the port-blocked check: the
// ports it translated and the chains of the operator's policy they cross.
// Its zero value holds no port, which reports nothing.
type portPolicy struct {
	ports  []publishedPort
	chains []ForwardChain
}

// publishedPorts answers the ports the tables of cfg translate, with the
// address each is translated to: the ports of every enabled instance, in
// instance and port name order. A disabled instance publishes nothing
// (publishedTerms), so it has no port here.
func publishedPorts(cfg Config, records map[string]addressRecord) []publishedPort {
	var ports []publishedPort
	for _, instance := range cfg.Instances {
		if !instance.publishes() {
			continue
		}
		network, _ := cfg.network(instance.Attachments[0].Network) // parseSections refused an attachment to an unknown network
		given := records[network.Name].Instances[instance.Name]
		for _, port := range instance.Ports {
			ports = append(ports, publishedPort{
				instance:       instance.Name,
				port:           port.Name,
				protocol:       port.Protocol,
				containerPorts: port.ContainerPorts,
				bridge:         network.Bridge,
				targets:        portTargets(network, port, given.Addresses),
			})
		}
	}
	return ports
}

// portTargets answers the instance's address in each family the port is
// published in. assigned holds one address per prefix of network, in the
// same order; buildTables refused a config whose assignment is shorter.
func portTargets(network Network, port Port, assigned []netip.Addr) []netip.Addr {
	var targets []netip.Addr
	for index, prefix := range network.Prefixes {
		if index >= len(assigned) {
			break
		}
		if port.publishedIn(prefix.Addr().Is4()) {
			targets = append(targets, assigned[index])
		}
	}
	return targets
}

// publishedIn answers whether the port is published in the asked family: in
// every family when it has no listen address, else in the family of each.
func (p Port) publishedIn(ipv4 bool) bool {
	if len(p.Listen) == 0 {
		return true
	}
	for _, address := range p.Listen {
		if address.Is4() == ipv4 {
			return true
		}
	}
	return false
}

// findings answers one finding per published port and chain that blocks it,
// in port then chain order.
func (p portPolicy) findings() []rpc.DoctorCheckDiagnostic {
	var findings []rpc.DoctorCheckDiagnostic
	for _, port := range p.ports {
		for _, chain := range p.chains {
			if !chain.blocks(port) {
				continue
			}
			findings = append(findings, rpc.DoctorCheckDiagnostic{
				Code:     codePortBlocked,
				Severity: severityPortBlocked,
				Message:  blockedMessage(port, chain),
				Fix:      acceptingTerm(port, chain),
			})
		}
	}
	return findings
}

// blockedMessage says which port is blocked, by which chain, and what the
// policy must hold.
func blockedMessage(port publishedPort, chain ForwardChain) string {
	addresses := make([]string, 0, len(port.targets))
	for _, target := range port.targets {
		if chain.carries(target) {
			addresses = append(addresses, target.String())
		}
	}
	return "container " + port.instance + " port " + port.port + " is published and the firewall policy blocks it: chain " +
		chain.Chain + " of firewall table " + chain.Table + " is at the forward hook with policy drop, and none of its accepting terms names output-interface " +
		port.bridge + " or destination-address " + strings.Join(addresses, " or ") +
		", so a connection from beyond the router is translated and then dropped. Paste the lines below into ze config edit: they add a term that accepts " +
		port.protocol + " to port " + port.containerPorts.String() + " leaving by " + port.bridge +
		". The replies need a term of their own, by connection-state established,related or by input-interface " + port.bridge +
		". The editor adds a new term after the chain's existing terms, so it accepts only what no earlier term drops"
}

// acceptingTerm answers the lines that add, to chain, the term that accepts
// the port's forwarded traffic, and commit it. The term matches the protocol,
// the container port (the destination after translation) and the bridge as
// output interface, so one term serves both families.
func acceptingTerm(port publishedPort, chain ForwardChain) string {
	term := "set " + firewallRoot + " " + sectionTableKey + " " + chain.Table + " " + listChain + " " + chain.Chain +
		" " + listTerm + " " + fixTermPrefix + port.instance + termSeparator + port.port + " "
	return term + leafFrom + " " + leafProtocol + " " + port.protocol + "\n" +
		term + leafFrom + " " + leafDestinationPort + " " + port.containerPorts.String() + "\n" +
		term + leafFrom + " " + leafOutputInterface + " " + port.bridge + "\n" +
		term + leafThen + " " + chainPolicyAccept + "\n" +
		"commit now"
}

// answerPortBlocked answers show doctor's request for the port-blocked
// check from what the last apply recorded. It only reads and makes no engine
// call. It judges the committed firewall config, which is what the firewall
// component applied at the same commit.
func (m *manager) answerPortBlocked() ([]rpc.DoctorCheckDiagnostic, error) {
	m.mu.Lock()
	policy := m.policy
	m.mu.Unlock()
	return policy.findings(), nil
}

// diagnosticCodes is the explanation of each code this package owns. It is
// registered from register.go, so a build without the plugin holds none.
var diagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codePortBlocked,
		Title: "The firewall policy blocks a published container port",
		Description: "A container publishes a port, and a base chain of the firewall configuration at the forward hook has policy drop and no accepting term that names the container network's bridge as output interface or the container's address as destination. " +
			"A connection from beyond the router to a published port is translated, then forwarded to the container, so it crosses every chain at the forward hook, and a drop in any of them is final: publishing a port never opens the firewall policy. " +
			"The check judges those two matches only, so it also reports a port that a term accepts by another match, and it does not see a drop term ahead of the accepting one. " +
			"It reads the forward hook only: a chain at the output or input hook that blocks a connection the router itself opens is not reported. " +
			"The finding names the container, the port entry, the chain and the table, and its fix lines add the accepting term. The replies need a term too, by connection state established,related or by input interface the bridge.",
		Examples: []string{"ze show doctor", "ze show diagnostic " + codePortBlocked},
	},
}
