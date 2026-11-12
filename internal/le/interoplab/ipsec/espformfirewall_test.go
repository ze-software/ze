// Design: docs/architecture/testing/interop.md -- the ESP form fast path kept off by a firewall rule that drops ESP.
package ipsec

import (
	"strings"
	"testing"
)

// VALIDATES: espFirewallFinding accepts a doctor answer only when a
// doctor-ipsec-esp-form-firewall finding names both the kernel table and the term,
// wherever the finding sits in the answer, and refuses an answer with no such
// finding, a finding that names another site, and an answer that is not JSON.
// PREVENTS: the scenario passing on a firewall finding about another rule, or on an
// answer the checker could not read.
func TestESPFirewallFindingNamesTheTableAndTheTerm(t *testing.T) {
	named := `{"checks":[{"name":"ipsec-esp-form-fast-path","diagnostics":[` +
		`{"code":"doctor-ipsec-esp-form-firewall","severity":"warning",` +
		`"message":"The first site that could drop it: inet table ze_espdrop, chain input, hook input, handle 4 \"drop-esp\": drop."}]}]}`
	if err := espFirewallFinding(named); err != nil {
		t.Errorf("a finding naming ze_espdrop and drop-esp was refused: %v", err)
	}

	refused := []struct {
		name, answer, want string
	}{
		{"no finding", `{"checks":[{"diagnostics":[{"code":"doctor-ipsec-esp-form-fast-path","message":"ze_espdrop drop-esp"}]}]}`, "answers no doctor-ipsec-esp-form-firewall finding"},
		{"another table", `[{"code":"doctor-ipsec-esp-form-firewall","message":"inet table ze_other, chain input, handle 4 \"drop-esp\": drop"}]`, "none naming table ze_espdrop"},
		{"another term", `[{"code":"doctor-ipsec-esp-form-firewall","message":"inet table ze_espdrop, chain input, handle 4 \"other\": drop"}]`, "none naming table ze_espdrop"},
		{"message not text", `[{"code":"doctor-ipsec-esp-form-firewall","message":7}]`, "none naming table ze_espdrop"},
		{"not JSON", `show doctor: no daemon`, "is not JSON"},
	}
	for _, c := range refused {
		err := espFirewallFinding(c.answer)
		if err == nil {
			t.Errorf("%s: accepted, want refused", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: refused with %q, want it to say %q", c.name, err, c.want)
		}
	}
}

// VALIDATES: assertESPStill accepts a direction whose SA moved no bytes, and refuses a
// direction with no SA, an SA retired between the reads, and an SA that moved.
// PREVENTS: Ze's inbound SA "staying still" because there was no SA to move, which
// would let the scenario pass with no Child SA at all.
func TestAssertESPStillRefusesWhatCannotProveStillness(t *testing.T) {
	sa := saKey{source: swanIP, target: zeIP, spi: "0xc0ffee01"}
	other := saKey{source: zeIP, target: swanIP, spi: "0xc0ffee02"}
	if err := assertESPStill(map[saKey]uint64{sa: 100, other: 5}, map[saKey]uint64{sa: 100, other: 9}, zeDecrypts); err != nil {
		t.Errorf("an SA that moved no bytes was refused: %v", err)
	}
	cases := []struct {
		name          string
		before, after map[saKey]uint64
		want          string
	}{
		{"no SA", map[saKey]uint64{other: 5}, map[saKey]uint64{other: 9}, "reports no SA"},
		{"retired", map[saKey]uint64{sa: 100}, map[saKey]uint64{other: 9}, "retired SA"},
		{"moved", map[saKey]uint64{sa: 100}, map[saKey]uint64{sa: 164}, "moved from 100 to 164 bytes"},
	}
	for _, c := range cases {
		err := assertESPStill(c.before, c.after, zeDecrypts)
		if err == nil {
			t.Errorf("%s: accepted, want refused", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: refused with %q, want it to say %q", c.name, err, c.want)
		}
	}
}
