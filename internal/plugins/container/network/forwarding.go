// Design: docs/architecture/container/network.md -- the forwarding defaults
// Related: apply.go -- the apply that claims and withdraws them
//
// Traffic between a container network and anything beyond its bridge is
// forwarded by the router: a connection from beyond the router to a
// published port, and a container's connection to a host beyond it. The
// kernel forwards a family only while its forwarding setting is on, so the
// package claims that setting, for each family a network has a prefix of, as
// a default of the sysctl plugin. The sysctl plugin writes a default unless
// the operator's sysctl config sets the key, keeps a default another source
// claims too until both withdrew, and puts the kernel's own value back when
// the last claim is withdrawn or the daemon stops.

package network

import (
	"context"
	"encoding/json"

	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
)

// The forwarding keys are the ones fib-kernel claims, so the sysctl plugin
// sees one key claimed by two sources and keeps it on until both withdrew. A
// second spelling of the same kernel setting (net.ipv4.ip_forward) would be a
// second key whose withdrawal turns forwarding off under the other claim.
//
// The sysctl plugin withdraws defaults by source, every key of a source at
// once, so each family is claimed under a source of its own: a family is
// withdrawn without touching the other, and without touching the runtime's
// own claim, which the container plugin makes under its plugin name.
const (
	forwardingIPv4Key    = "net.ipv4.conf.all.forwarding"
	forwardingIPv6Key    = "net.ipv6.conf.all.forwarding"
	forwardingOn         = "1"
	forwardingSourceIPv4 = "container-network-ipv4"
	forwardingSourceIPv6 = "container-network-ipv6"
)

// forwardingFamily is the forwarding default of one family.
type forwardingFamily struct {
	key    string
	source string
	ipv4   bool
}

// forwardingFamilies are the two families, IPv4 first. A claim state is an
// array indexed like it.
var forwardingFamilies = [...]forwardingFamily{
	{key: forwardingIPv4Key, source: forwardingSourceIPv4, ipv4: true},
	{key: forwardingIPv6Key, source: forwardingSourceIPv6, ipv4: false},
}

// forwardingClaims holds, per family of forwardingFamilies, whether the
// default is claimed or wanted.
type forwardingClaims [len(forwardingFamilies)]bool

// sysctlDefault is the payload of the sysctl plugin's default event.
type sysctlDefault struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

// sysctlSource is the payload of the sysctl plugin's clear event.
type sysctlSource struct {
	Source string `json:"source"`
}

// forwardingWanted answers the families whose forwarding the config needs:
// each family a network has a prefix of.
func forwardingWanted(cfg Config) forwardingClaims {
	var wanted forwardingClaims
	for index, family := range forwardingFamilies {
		for _, network := range cfg.Networks {
			if familyIndex(network.Prefixes, family.ipv4) >= 0 {
				wanted[index] = true
			}
		}
	}
	return wanted
}

// forward claims the forwarding default of each family wanted holds and
// claimed does not, and withdraws each claim wanted no longer holds. A family
// whose need did not change sends nothing: the sysctl plugin keeps a claim
// until it is withdrawn, and one the operator's config overrides would be
// reported again at every apply. Each change is recorded as it is made, so a
// refusal part way leaves the record true.
func (m *manager) forward(ctx context.Context, client container.EngineClient, wanted, claimed forwardingClaims) error {
	for index, family := range forwardingFamilies {
		if wanted[index] == claimed[index] {
			continue
		}
		var err error
		if wanted[index] {
			err = claimForwarding(ctx, client, family)
		} else {
			err = withdrawForwarding(ctx, client, family)
		}
		if err != nil {
			return err
		}
		m.recordForwarding(index, wanted[index])
	}
	return nil
}

// recordForwarding records whether one family's default is claimed.
func (m *manager) recordForwarding(index int, claimed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forwarding[index] = claimed
}

func claimForwarding(ctx context.Context, client container.EngineClient, family forwardingFamily) error {
	payload, err := json.Marshal(sysctlDefault{Key: family.key, Value: forwardingOn, Source: family.source})
	if err != nil {
		return err
	}
	if _, err := client.EmitEvent(ctx, sysctlevents.Namespace, sysctlevents.EventDefault, "", "", string(payload)); err != nil {
		return opererr.Wrap(err,
			"the kernel setting "+family.key+" could not be asked of the sysctl plugin",
			"a container network's traffic beyond its bridge is forwarded by the router, which the kernel does only while the setting is 1",
			"ze show doctor",
		)
	}
	return nil
}

func withdrawForwarding(ctx context.Context, client container.EngineClient, family forwardingFamily) error {
	payload, err := json.Marshal(sysctlSource{Source: family.source})
	if err != nil {
		return err
	}
	if _, err := client.EmitEvent(ctx, sysctlevents.Namespace, sysctlevents.EventClearSourceDefaults, "", "", string(payload)); err != nil {
		return opererr.Wrap(err,
			"the container plugin could not hand the kernel setting "+family.key+" back to the sysctl plugin",
			"no container network needs it any more, and the sysctl plugin restores the kernel's value once no source claims it",
			"ze show doctor",
		)
	}
	return nil
}
