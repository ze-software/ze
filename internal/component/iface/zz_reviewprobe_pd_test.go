package iface

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

func TestZZReviewProbePDEmpty(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"pd { }", "pd { length 56; }"} {
		tree, err := config.NewParser(schema).Parse("interface {\n ethernet eth0 {\n unit 0 {\n ipv6 {\n dhcpv6 {\n enabled true;\n " + body + "\n }\n }\n }\n }\n}\n")
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		raw, _ := json.Marshal(tree.ToMap())
		cfg, err := parseIfaceConfig(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		d := cfg.Ethernet[0].Units[0].IPv6.DHCPv6
		t.Logf("%s => json=%s requested=%v length=%d", body, raw, d.PDRequested, d.PDLength)
	}
}
