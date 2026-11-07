// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- keepalive and reconnect timing
// Related: pppoe_client.go -- run applies the reconnect bounds
// Related: config.go -- parsePPPoEClientEntry reads the timing leaves

package iface

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/config"
)

// The bounds of the pppoe-client keepalive and reconnect leaves. They repeat
// the YANG ranges on purpose: Validate is the second check of the pair, so a
// caller that builds a PPPoEClientTiming without the config still cannot pass
// a zero interval to a ticker or a zero delay to the redial loop.
const (
	pppoeTimingSecondsMax     = 3600
	pppoeKeepaliveFailuresMax = 255
)

// PPPoEClientTiming holds the timers of one PPPoE client that a reload
// changes without restarting the session: dead-peer detection by LCP
// Echo-Request and the redial hold-off. Its zero value is invalid; Validate
// refuses it.
type PPPoEClientTiming struct {
	// KeepaliveInterval is the time between two LCP Echo-Requests.
	KeepaliveInterval time.Duration
	// KeepaliveFailures is the count of unanswered Echo-Requests in a row
	// that ends the session: the session ends on the Nth one, not after it.
	KeepaliveFailures int
	// ReconnectDelayMin is the wait before the first redial after a failure.
	ReconnectDelayMin time.Duration
	// ReconnectDelayMax caps the doubling wait. It is never below ReconnectDelayMin.
	ReconnectDelayMax time.Duration
}

// Validate refuses a timing outside the ranges the YANG leaves declare, and a
// delay-max below delay-min.
func (t PPPoEClientTiming) Validate() error {
	if err := timingSeconds("keepalive interval", t.KeepaliveInterval); err != nil {
		return err
	}
	if t.KeepaliveFailures < 1 {
		return fmt.Errorf("keepalive failures %d is outside 1..%d\n  set keepalive failures to a value in that range",
			t.KeepaliveFailures, pppoeKeepaliveFailuresMax)
	}
	if t.KeepaliveFailures > pppoeKeepaliveFailuresMax {
		return fmt.Errorf("keepalive failures %d is outside 1..%d\n  set keepalive failures to a value in that range",
			t.KeepaliveFailures, pppoeKeepaliveFailuresMax)
	}
	if err := timingSeconds("reconnect delay-min", t.ReconnectDelayMin); err != nil {
		return err
	}
	if err := timingSeconds("reconnect delay-max", t.ReconnectDelayMax); err != nil {
		return err
	}
	if t.ReconnectDelayMax < t.ReconnectDelayMin {
		minSeconds := int(t.ReconnectDelayMin / time.Second)
		return fmt.Errorf("reconnect delay-max %d is below reconnect delay-min %d: "+
			"the redial wait starts at delay-min and can never be capped below it\n"+
			"  set reconnect delay-max to %d or more, or lower reconnect delay-min",
			int(t.ReconnectDelayMax/time.Second), minSeconds, minSeconds)
	}
	return nil
}

// timingSeconds refuses a duration outside 1..3600 seconds for the named leaf.
func timingSeconds(leaf string, d time.Duration) error {
	if d < time.Second {
		return fmt.Errorf("%s %s is outside 1..%d seconds\n  set %s to a value in that range",
			leaf, d, pppoeTimingSecondsMax, leaf)
	}
	if d > pppoeTimingSecondsMax*time.Second {
		return fmt.Errorf("%s %s is outside 1..%d seconds\n  set %s to a value in that range",
			leaf, d, pppoeTimingSecondsMax, leaf)
	}
	return nil
}

// reconnectDelay answers the wait before redial number attempt: delay-min for
// the first, doubled for each further attempt, capped at delay-max. A timing
// that Validate refuses gets no delay, because a zero delay would redial in a
// tight loop.
func (t PPPoEClientTiming) reconnectDelay(attempt int) (time.Duration, error) {
	if err := t.Validate(); err != nil {
		return 0, err
	}
	delay := t.ReconnectDelayMin
	// Bound: delay starts at one second or more and doubles toward a cap of
	// at most 3600 seconds, so the loop ends within twelve doublings whatever
	// attempt holds.
	for step := 1; step < attempt; step++ {
		if delay >= t.ReconnectDelayMax {
			break
		}
		delay *= 2
	}
	return min(delay, t.ReconnectDelayMax), nil
}

// pppoeClientSchemaPath names the pppoe-client list whose keepalive and
// reconnect containers declare the timing defaults.
const pppoeClientSchemaPath = "interface/pppoe-client"

// pppoeClientSchema holds the keepalive and reconnect containers of the YANG
// schema, so the parse applies their defaults before it reads the leaves.
// The defaults live in the schema and nowhere else: an entry that names no
// keepalive interval runs at 10 seconds because ze-iface-conf.yang says
// `default 10`, not because a Go constant repeats it.
type pppoeClientSchema struct {
	keepalive *config.ContainerNode
	reconnect *config.ContainerNode
}

// loadPPPoEClientSchema resolves both containers once for one parse. A parse
// that cannot read the schema cannot know the defaults it owes, so every
// failure is reported.
func loadPPPoEClientSchema() (pppoeClientSchema, error) {
	schema, err := config.YANGSchema()
	if err != nil {
		return pppoeClientSchema{}, fmt.Errorf("pppoe-client: load schema: %w", err)
	}
	keepalive, err := pppoeClientContainer(schema, "keepalive")
	if err != nil {
		return pppoeClientSchema{}, err
	}
	reconnect, err := pppoeClientContainer(schema, "reconnect")
	if err != nil {
		return pppoeClientSchema{}, err
	}
	return pppoeClientSchema{keepalive: keepalive, reconnect: reconnect}, nil
}

// pppoeClientContainer resolves one container under the pppoe-client list.
// The list carries ze:os "linux", so a schema built for another OS prunes it
// and Lookup cannot reach it. The defaults are the module's, not the host's,
// so it reads them through LookupDeclared, which also reaches a pruned node:
// the parse answers the same defaults on every host. The config parser still
// refuses a pppoe-client block on a host other than Linux.
func pppoeClientContainer(schema *config.Schema, name string) (*config.ContainerNode, error) {
	path := pppoeClientSchemaPath + "/" + name
	node, err := schema.LookupDeclared(path)
	if err != nil {
		return nil, fmt.Errorf("pppoe-client: resolve %s: %w", path, err)
	}
	container, ok := node.(*config.ContainerNode)
	if !ok {
		return nil, fmt.Errorf("pppoe-client: %s is %T, want a container", path, node)
	}
	return container, nil
}

var errPPPoEClientSchemaUnresolved = errors.New("pppoe-client: the keepalive and reconnect schema was never resolved")

// containerWithDefaults answers the named container of entry with its YANG
// defaults applied. An absent container is created, because the defaults are
// owed whether or not the operator wrote the block.
func containerWithDefaults(entry map[string]any, name string, node *config.ContainerNode) (map[string]any, error) {
	if node == nil {
		return nil, errPPPoEClientSchemaUnresolved
	}
	raw, present := entry[name]
	container, ok := raw.(map[string]any)
	if present && !ok && raw != nil {
		return nil, fmt.Errorf("%s must be a block, got %T\n  write %s { ... }", name, raw, name)
	}
	if container == nil {
		container = map[string]any{}
		entry[name] = container
	}
	config.ApplyDefaults(container, node)
	return container, nil
}

// timingLeaf reads one numeric timing leaf that the schema defaults filled.
// An absent value means the schema declares no default, which is a Ze defect
// rather than an operator choice, so it is refused rather than read as zero.
func timingLeaf(container map[string]any, block, leaf string, limit int) (int, error) {
	raw, ok := container[leaf].(string)
	if !ok {
		return 0, fmt.Errorf("%s %s has no value and its YANG default is missing", block, leaf)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s %s %q is not a number\n  set %s %s to a value in 1..%d", block, leaf, raw, block, leaf, limit)
	}
	if value < 1 {
		return 0, fmt.Errorf("%s %s %d is outside 1..%d\n  set %s %s to a value in that range", block, leaf, value, limit, block, leaf)
	}
	if value > limit {
		return 0, fmt.Errorf("%s %s %d is outside 1..%d\n  set %s %s to a value in that range", block, leaf, value, limit, block, leaf)
	}
	return value, nil
}

// parsePPPoEClientTiming reads the keepalive and reconnect blocks of one
// pppoe-client entry, YANG defaults applied, and validates the result.
func parsePPPoEClientTiming(entry map[string]any, pppoeSchema pppoeClientSchema) (PPPoEClientTiming, error) {
	keepalive, err := containerWithDefaults(entry, "keepalive", pppoeSchema.keepalive)
	if err != nil {
		return PPPoEClientTiming{}, err
	}
	reconnect, err := containerWithDefaults(entry, "reconnect", pppoeSchema.reconnect)
	if err != nil {
		return PPPoEClientTiming{}, err
	}
	interval, err := timingLeaf(keepalive, "keepalive", "interval", pppoeTimingSecondsMax)
	if err != nil {
		return PPPoEClientTiming{}, err
	}
	failures, err := timingLeaf(keepalive, "keepalive", "failures", pppoeKeepaliveFailuresMax)
	if err != nil {
		return PPPoEClientTiming{}, err
	}
	delayMin, err := timingLeaf(reconnect, "reconnect", "delay-min", pppoeTimingSecondsMax)
	if err != nil {
		return PPPoEClientTiming{}, err
	}
	delayMax, err := timingLeaf(reconnect, "reconnect", "delay-max", pppoeTimingSecondsMax)
	if err != nil {
		return PPPoEClientTiming{}, err
	}
	timing := PPPoEClientTiming{
		KeepaliveInterval: time.Duration(interval) * time.Second,
		KeepaliveFailures: failures,
		ReconnectDelayMin: time.Duration(delayMin) * time.Second,
		ReconnectDelayMax: time.Duration(delayMax) * time.Second,
	}
	if err := timing.Validate(); err != nil {
		return PPPoEClientTiming{}, err
	}
	return timing, nil
}
