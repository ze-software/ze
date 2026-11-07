package iface

import (
	"slices"
	"testing"
)

// TestDeclaredDynamicNames checks the pure parser static calls on its delivered
// candidate interface section: it answers the pppoe-client keys, nothing for an
// interface kind iface creates or matches at once, and nothing for a section
// that carries no interface container (a deleted root arrives as "{}").
// Method: one table of section bodies, each with the names it must answer.
func TestDeclaredDynamicNames(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []string
	}{
		{
			name: "pppoe-client keys are declared",
			data: `{"interface":{"ethernet":{"eth0":{}},"pppoe-client":{"pppoe0":{"source-interface":"eth0"},"pppoe1":{"source-interface":"eth0"}}}}`,
			want: []string{"pppoe0", "pppoe1"},
		},
		{
			name: "other kinds declare nothing",
			data: `{"interface":{"ethernet":{"eth0":{}},"dummy":{"dum0":{}},"bridge":{"br0":{}},"veth":{"ve0":{}},"wireguard":{"wg0":{}}}}`,
			want: []string{},
		},
		{
			name: "a deleted root declares nothing",
			data: `{}`,
			want: []string{},
		},
		{
			name: "an empty interface container declares nothing",
			data: `{"interface":{}}`,
			want: []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DeclaredDynamicNames(tc.data)
			if err != nil {
				t.Fatalf("DeclaredDynamicNames: %v", err)
			}
			if names := got.Names(); !slices.Equal(names, tc.want) {
				t.Errorf("Names() = %v, want %v", names, tc.want)
			}
			for _, name := range tc.want {
				if !got.Has(name) {
					t.Errorf("Has(%q) = false, want true", name)
				}
			}
			if got.Has("eth0") {
				t.Error("Has(eth0) = true: an ethernet entry is not dynamic")
			}
		})
	}

	// The zero value is the answer for a delivery with no interface section.
	var none DeclaredNames
	if none.Has("pppoe0") || len(none.Names()) != 0 {
		t.Error("the zero DeclaredNames must declare no name")
	}
}

// TestDeclaredDynamicNamesRefusesMalformed checks that a body the set cannot be
// answered from is refused rather than read as declaring nothing.
func TestDeclaredDynamicNamesRefusesMalformed(t *testing.T) {
	for _, data := range []string{
		`not json`,
		`{"interface":"eth0"}`,
		`{"interface":{"pppoe-client":["pppoe0"]}}`,
	} {
		if _, err := DeclaredDynamicNames(data); err == nil {
			t.Errorf("DeclaredDynamicNames(%s) answered no error, want a refusal", data)
		}
	}
}
