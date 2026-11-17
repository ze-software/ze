// Design: docs/architecture/container/network.md -- the apply: assignments, bridges and tables
// Related: attach.go -- the participant that reads what the apply planned
// Related: address.go -- the assignment the apply stores
// Related: tables.go -- the tables the apply sends

package network

import (
	"context"
	"encoding/json"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// manager holds what the network package knows between calls: the engine
// client, the config last applied, the links each instance's start makes,
// and the links that exist. Safe for concurrent use: the sibling apply runs
// on the config path while containers start and end on their own workers.
type manager struct {
	mu     sync.Mutex
	client container.EngineClient
	// plans are the links each instance's start makes, by instance name, in
	// the order of its network list.
	plans map[string][]linkPlan
	// live are the links made for each instance that has not ended.
	live map[string][]linkPlan
	// bridgesSent and tablesSent say whether the last set sent was not
	// empty, so an empty set is sent once to withdraw it and never to a
	// component a config with no network may not load.
	bridgesSent bool
	tablesSent  bool
}

// linkPlan is one link of an instance: the request the created call sends.
type linkPlan struct {
	network   string
	bridge    string
	slot      uint16
	name      string
	hardware  string
	mtu       uint32
	addresses []netip.Prefix
	gateways  []netip.Addr
}

func newManager() *manager {
	return &manager{plans: map[string][]linkPlan{}, live: map[string][]linkPlan{}}
}

// receive stores the engine client the container plugin hands each run.
func (m *manager) receive(client container.EngineClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.client = client
}

// apply is the network sibling's apply. It stores each network's address
// assignments, then sends the complete set of bridges to the interface
// component and the complete set of tables to the firewall component. It is
// idempotent: every call sends complete sets. A config with no network sends
// nothing unless a previous set must be withdrawn.
func (m *manager) apply(ctx context.Context, cfg Config) error {
	m.mu.Lock()
	client := m.client
	sendBridges := len(cfg.Networks) > 0 || m.bridgesSent
	sendTables := hasTables(cfg) || m.tablesSent
	live := copyLive(m.live)
	m.mu.Unlock()

	if !sendBridges {
		if !sendTables {
			m.install(map[string][]linkPlan{}, false, false)
			return nil
		}
	}
	if client == nil {
		return refuseNoEngine("the container networks were not applied")
	}
	records, err := assignAll(ctx, client, cfg, live)
	if err != nil {
		return err
	}
	if sendBridges {
		if _, err := client.OwnedBridgesReplace(ctx, &rpc.OwnedBridgesReplaceInput{Bridges: bridgeSet(cfg)}); err != nil {
			return opererr.Wrap(err,
				"the interface component refused the container network bridges",
				"each container network is a bridge the interface component owns for the container plugin",
				"ze show doctor",
			)
		}
	}
	if sendTables {
		section, err := buildTables(cfg, records)
		if err != nil {
			return opererr.Wrap(err,
				"the container translation tables could not be written",
				"the tables travel to the firewall component as a firewall config section",
				"ze show doctor",
			)
		}
		if _, err := client.FirewallTablesReplace(ctx, &rpc.FirewallTablesReplaceInput{Section: section}); err != nil {
			return opererr.Wrap(err,
				"the firewall component refused the container translation tables",
				"the outbound translation of each network and the published ports are tables the firewall component owns for the container plugin",
				"ze show firewall ruleset",
			)
		}
	}
	m.install(planLinks(cfg, records), len(cfg.Networks) > 0, hasTables(cfg))
	return nil
}

// install records what an apply made.
func (m *manager) install(plans map[string][]linkPlan, bridgesSent, tablesSent bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plans = plans
	m.bridgesSent = bridgesSent
	m.tablesSent = tablesSent
}

// copyLive copies the live links, so the apply reads them without the lock.
func copyLive(live map[string][]linkPlan) map[string][]linkPlan {
	copied := make(map[string][]linkPlan, len(live))
	for instance, links := range live {
		copied[instance] = append([]linkPlan(nil), links...)
	}
	return copied
}

// bridgeSet answers the complete set of bridges: one per network, with its
// mtu and its gateway addresses.
func bridgeSet(cfg Config) []rpc.OwnedBridge {
	bridges := make([]rpc.OwnedBridge, 0, len(cfg.Networks))
	for _, network := range cfg.Networks {
		addresses := make([]netip.Prefix, 0, len(network.Prefixes))
		for index, prefix := range network.Prefixes {
			addresses = append(addresses, netip.PrefixFrom(network.Gateways[index], prefix.Bits()))
		}
		bridges = append(bridges, rpc.OwnedBridge{Name: network.Bridge, MTU: network.MTU, Addresses: addresses})
	}
	return bridges
}

// assignAll reads each network's record, assigns its attachments and writes
// the record back when it changed. A network that left the config keeps its
// record, which the next config holding it reads again.
func assignAll(ctx context.Context, client container.EngineClient, cfg Config, live map[string][]linkPlan) (map[string]addressRecord, error) {
	records := make(map[string]addressRecord, len(cfg.Networks))
	for _, network := range cfg.Networks {
		key := zefs.KeyContainerNetworkAddress.Key(network.Name)
		value, found, err := client.StateGet(ctx, key)
		if err != nil {
			return nil, refuseState(network.Name, "read", err)
		}
		var stored addressRecord
		if found {
			stored = decodeRecord(value)
		}
		members := membersOf(cfg, network.Name)
		record, err := assign(network, members, stored, heldBy(live, members, network.Name))
		if err != nil {
			return nil, err
		}
		records[network.Name] = record
		if sameRecord(record, stored) {
			continue
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			return nil, refuseState(network.Name, "encoded", err)
		}
		if err := client.StatePut(ctx, key, encoded); err != nil {
			return nil, refuseState(network.Name, "written", err)
		}
	}
	return records, nil
}

// membersOf answers the instances attached to network, in instance name
// order, with the addresses their operators set.
func membersOf(cfg Config, network string) []attached {
	var members []attached
	for _, instance := range cfg.Instances {
		for _, attachment := range instance.Attachments {
			if attachment.Network == network {
				members = append(members, attached{instance: instance.Name, set: attachment.Addresses})
			}
		}
	}
	return members
}

// heldBy answers the addresses on network that links of instances no longer
// attached to it still carry: those containers have not ended yet.
func heldBy(live map[string][]linkPlan, members []attached, network string) []netip.Addr {
	attachedNow := make(map[string]bool, len(members))
	for _, member := range members {
		attachedNow[member.instance] = true
	}
	var held []netip.Addr
	for instance, links := range live {
		if attachedNow[instance] {
			continue
		}
		for _, link := range links {
			if link.network != network {
				continue
			}
			for _, address := range link.addresses {
				held = append(held, address.Addr())
			}
		}
	}
	return held
}

// planLinks answers each instance's links from the config and the records.
func planLinks(cfg Config, records map[string]addressRecord) map[string][]linkPlan {
	plans := make(map[string][]linkPlan, len(cfg.Instances))
	for _, instance := range cfg.Instances {
		links := make([]linkPlan, 0, len(instance.Attachments))
		for position, attachment := range instance.Attachments {
			network, _ := cfg.network(attachment.Network) // parseSections refused an attachment to an unknown network
			given := records[network.Name].Instances[instance.Name]
			addresses := make([]netip.Prefix, 0, len(network.Prefixes))
			for index, prefix := range network.Prefixes {
				addresses = append(addresses, netip.PrefixFrom(given.Addresses[index], prefix.Bits()))
			}
			links = append(links, linkPlan{
				network:   network.Name,
				bridge:    network.Bridge,
				slot:      given.Slot,
				name:      containerLinkName(position),
				hardware:  attachment.HardwareAddress,
				mtu:       network.MTU,
				addresses: addresses,
				gateways:  append([]netip.Addr(nil), network.Gateways...),
			})
		}
		plans[instance.Name] = links
	}
	return plans
}

func refuseNoEngine(problem string) error {
	return opererr.New(
		problem+": the container plugin holds no engine connection",
		"bridges, links and tables are requests to the interface and firewall components, made through the engine",
		"ze show doctor",
	)
}

func refuseState(network, step string, err error) error {
	return opererr.Wrap(err,
		"network "+network+": the address record could not be "+step,
		"each container's address is kept in the plugin's runtime state so it is the same after a restart",
		"ze show doctor",
	)
}
