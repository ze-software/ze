package bgpconfig

import (
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/sysrib"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/policy"
)

// d8ReaderType is a filter type only this test declares: it reads the four
// fields D-8 added to the BGP attachments.
const d8ReaderType = "test-d8-reader"

func init() {
	policy.MustRegisterFilterType(d8ReaderType,
		[]policy.Field{policy.FieldAtomicAggregate, policy.FieldAggregator, policy.FieldOriginatorID, policy.FieldClusterList}, nil)
}

// TestBGPAttachmentsCarryD8Fields proves the registration half of AC-23: a
// filter reading atomic-aggregate, aggregator, originator-id and
// cluster-list passes the field check at bgp import and bgp export. That
// every carried field has a BGP view reader is checked when this package
// initializes (register.go), so this binary would not start otherwise.
func TestBGPAttachmentsCarryD8Fields(t *testing.T) {
	reg, err := policy.NewRegistry(policy.Instance{Name: "READS-D8", Type: d8ReaderType})
	if err != nil {
		t.Fatal(err)
	}
	for _, attachment := range []string{"bgp import", "bgp export"} {
		if err := policy.CheckChain(attachment, reg, []string{"READS-D8"}); err != nil {
			t.Errorf("%s refused the D-8 fields: %v", attachment, err)
		}
	}
}

// TestCheckChainMetricRefusedAtBGP proves AC-24 and R-11: a match instance
// with a metric condition is refused at bgp import and bgp export, naming the
// instance, the field metric, the attachment and med as the BGP field for
// that fact, while the same instance still passes at rib import. Method: the
// instance comes from a parsed policy root through the registry the peer
// load builds, then each attachment's field check judges it.
func TestCheckChainMetricRefusedAtBGP(t *testing.T) {
	tree, err := config.ParseTreeWithYANG(`
policy {
	match LOW-METRIC {
		metric { maximum 10; }
	}
}
`, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	reg, err := policyRegistryOf(tree, schema)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	for _, attachment := range []string{"bgp import", "bgp export"} {
		err := policy.CheckChain(attachment, reg, []string{"LOW-METRIC"})
		if err == nil {
			t.Fatalf("%s accepted a match on metric", attachment)
		}
		for _, want := range []string{`"LOW-METRIC"`, "reads metric", attachment, "names the metric fact med"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: err %q does not name %q", attachment, err, want)
			}
		}
	}
	if err := policy.CheckChain("rib import", reg, []string{"LOW-METRIC"}); err != nil {
		t.Errorf("rib import refused a match on metric: %v", err)
	}
}

// settableProbePrefix names the filter types TestBGPAttachmentsSettableAreTheChainSets
// declares, one per field it probes.
const settableProbePrefix = "test-sets-"

func init() {
	for _, field := range policychain.SettableFields() {
		policy.MustRegisterFilterType(settableProbePrefix+field.String(), nil, []policy.Field{field})
	}
	policy.MustRegisterFilterType(settableProbePrefix+policy.FieldPrefix.String(), nil, []policy.Field{policy.FieldPrefix})
}

// TestBGPAttachmentsSettableAreTheChainSets proves the settable half of D-9
// at the attachment: a filter setting any field the policy chain writes
// passes the field check at bgp import and bgp export, and a filter setting
// prefix, a per-route intent the chain answers from a read and never a set,
// is refused. Method: one declared filter type per field, each judged by the
// check a peer's chain passes at load.
func TestBGPAttachmentsSettableAreTheChainSets(t *testing.T) {
	for _, field := range policychain.SettableFields() {
		name := "SETS-" + field.String()
		reg, err := policy.NewRegistry(policy.Instance{Name: name, Type: settableProbePrefix + field.String()})
		if err != nil {
			t.Fatal(err)
		}
		for _, attachment := range []string{"bgp import", "bgp export"} {
			if err := policy.CheckChain(attachment, reg, []string{name}); err != nil {
				t.Errorf("%s refused a filter setting %s, which the chain writes: %v", attachment, field, err)
			}
		}
	}
	reg, err := policy.NewRegistry(policy.Instance{Name: "SETS-prefix", Type: settableProbePrefix + policy.FieldPrefix.String()})
	if err != nil {
		t.Fatal(err)
	}
	for _, attachment := range []string{"bgp import", "bgp export"} {
		if err := policy.CheckChain(attachment, reg, []string{"SETS-prefix"}); err == nil {
			t.Errorf("%s accepted a filter setting prefix, which no chain set writes", attachment)
		}
	}
}
