package irr

import "testing"

// TestASReferenceReadsEveryNotation proves a firewall AS reference is read by
// asn.Parse, the one AS number reader: AS<n> names the same AS number in each
// RFC 5396 spelling, and a name that is no AS number is an AS-SET or a refusal.
// Method: newRefState and isASSetName over hand-built references.
// VALIDATES: the firewall consumer names the AS number the owner's
// `update irr asn` takes for the same text.
// PREVENTS: a private decimal-only parser refusing AS1.10 that the owner
// accepts, or reading AS0 as an AS number.
func TestASReferenceReadsEveryNotation(t *testing.T) {
	for name, want := range map[string]uint32{"AS65546": 65546, "AS1.10": 65546, "as13335": 13335, "AS0.100": 100} {
		st := newRefState(irrRef{Name: name})
		if st.keyErr != "" || st.asn != want {
			t.Errorf("%s: asn=%d keyErr=%q, want asn %d", name, st.asn, st.keyErr, want)
		}
		if isASSetName(name) {
			t.Errorf("%s read as an AS-SET name", name)
		}
	}
	for _, name := range []string{"AS0", "AS4294967296", "AS1.65536"} {
		if st := newRefState(irrRef{Name: name}); st.keyErr == "" {
			t.Errorf("%s accepted as AS number %d", name, st.asn)
		}
	}
	for _, name := range []string{"AS-FOO", "AS", "AS1x"} {
		if !isASSetName(name) {
			t.Errorf("%s not read as an AS-SET name", name)
		}
	}
}
