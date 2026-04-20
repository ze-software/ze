package mcp

import (
	"errors"
	"testing"
)

func TestAuthModeFromYANGString(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    AuthMode
		wantErr bool
	}{
		{"empty returns unspecified", "", AuthUnspecified, false},
		{"none", "none", AuthNone, false},
		{"bearer", "bearer", AuthBearer, false},
		{"bearer-list", "bearer-list", AuthBearerList, false},
		{"oauth", "oauth", AuthOAuth, false},
		{"unknown value rejects", "weird", AuthUnspecified, true},
		{"case sensitive: OAuth rejects", "OAuth", AuthUnspecified, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseAuthMode(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseAuthMode(%q) err = %v, wantErr = %v", tc.input, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("ParseAuthMode(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestAuthModeString(t *testing.T) {
	cases := []struct {
		mode AuthMode
		want string
	}{
		{AuthUnspecified, "unspecified"},
		{AuthNone, "none"},
		{AuthBearer, "bearer"},
		{AuthBearerList, "bearer-list"},
		{AuthOAuth, "oauth"},
		{AuthMode(99), "unspecified"},
	}
	for _, tc := range cases {
		if got := tc.mode.String(); got != tc.want {
			t.Fatalf("AuthMode(%d).String() = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

func TestAuthModeRoundtripParseString(t *testing.T) {
	for _, s := range []string{"none", "bearer", "bearer-list", "oauth"} {
		m, err := ParseAuthMode(s)
		if err != nil {
			t.Fatalf("ParseAuthMode(%q): %v", s, err)
		}
		if m.String() != s {
			t.Fatalf("roundtrip %q: parsed to %v, String() = %q", s, m, m.String())
		}
	}
}

func TestAuthModeZeroIsUnspecified(t *testing.T) {
	var m AuthMode
	if m != AuthUnspecified {
		t.Fatalf("zero value = %v, want AuthUnspecified", m)
	}
}

func TestIdentityAnonymous(t *testing.T) {
	var id Identity
	if !id.IsAnonymous() {
		t.Fatal("zero Identity.IsAnonymous() = false, want true")
	}

	named := Identity{Name: "alice"}
	if named.IsAnonymous() {
		t.Fatal("Identity{Name: alice}.IsAnonymous() = true, want false")
	}
}

func TestIdentityHasScope(t *testing.T) {
	cases := []struct {
		name  string
		id    Identity
		scope string
		want  bool
	}{
		{"empty scopes", Identity{Name: "a"}, "mcp.read", false},
		{"scope present", Identity{Name: "a", Scopes: []string{"mcp.read", "mcp.write"}}, "mcp.read", true},
		{"scope absent", Identity{Name: "a", Scopes: []string{"mcp.read"}}, "mcp.write", false},
		{"exact match required (no prefix)", Identity{Name: "a", Scopes: []string{"mcp.read-only"}}, "mcp.read", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.id.HasScope(tc.scope); got != tc.want {
				t.Fatalf("%v.HasScope(%q) = %v, want %v", tc.id, tc.scope, got, tc.want)
			}
		})
	}
}

func TestParseAuthModeErrorMessage(t *testing.T) {
	_, err := ParseAuthMode("bogus")
	if err == nil {
		t.Fatal("expected error on unknown value")
	}
	if !errors.Is(err, ErrAuthModeInvalid) {
		t.Fatalf("err = %v, want wrapping ErrAuthModeInvalid", err)
	}
}
