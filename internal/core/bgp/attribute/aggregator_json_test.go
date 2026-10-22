package attribute

import (
	"net/netip"
	"testing"
)

// TestAggregatorJSONFormatter checks the registered ATOMIC_AGGREGATE, AGGREGATOR
// and AS4_AGGREGATOR formatters through the registry, as the event writer reaches
// them.
//
// VALIDATES: each code has a formatter; ATOMIC_AGGREGATE writes `true`, and
// "aggregator" and "as4-aggregator" write "<asn>:<address>" with an asplain AS
// number; a value with no address answers nil so the caller writes the hex.
// PREVENTS: the attribute reaching a plugin as "attr-6" or "attr-7" hex.
func TestAggregatorJSONFormatter(t *testing.T) {
	address := netip.MustParseAddr("219.118.225.189")
	tests := []struct {
		name string
		code AttributeCode
		key  string
		attr Attribute
		want string
	}{
		{"aggregator", AttrAggregator, "aggregator", &Aggregator{ASN: 18144, Address: address}, `"18144:219.118.225.189"`},
		{"aggregator four-octet", AttrAggregator, "aggregator", &Aggregator{ASN: 4200000000, Address: address}, `"4200000000:219.118.225.189"`},
		{"as4-aggregator", AttrAS4Aggregator, "as4-aggregator", &AS4Aggregator{ASN: 4200000000, Address: address}, `"4200000000:219.118.225.189"`},
		{"atomic-aggregate", AttrAtomicAggregate, "atomic-aggregate", AtomicAggregate{}, `true`},
		{"aggregator without address", AttrAggregator, "aggregator", &Aggregator{ASN: 18144}, ""},
		{"wrong type", AttrAggregator, "aggregator", &AS4Aggregator{ASN: 1, Address: address}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := GetJSONFormatter(tt.code)
			if formatter == nil {
				t.Fatalf("no JSON formatter registered for %s", tt.code)
			}
			if formatter.Key != tt.key {
				t.Fatalf("key = %q, want %q", formatter.Key, tt.key)
			}
			got := formatter.AppendValue(nil, tt.attr)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("AppendValue = %q, want nil (hex fallback)", got)
				}
				return
			}
			if string(got) != tt.want {
				t.Fatalf("AppendValue = %s, want %s", got, tt.want)
			}
		})
	}
}
