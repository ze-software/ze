package cli

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestLoadConfigAndConfigValidateAgree is A-1 and Q-3: the two callers of the
// custom-validator walk reach the same verdict on the same bytes. They share one
// list and one walk, and this test is what would notice if they stopped.
func TestLoadConfigAndConfigValidateAgree(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		refused bool
	}{
		{"isis hostname outside 7-bit ASCII", "isis {\n\tnet 49.0001.0000.0000.0001.00\n\thostname café.example\n}\n", true},
		{"isis hostname in 7-bit ASCII", "isis {\n\tnet 49.0001.0000.0000.0001.00\n\thostname router-1.example.net.\n}\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, loadErr := config.LoadConfig(tc.src, "test.conf", nil)
			result := runValidation(tc.src, "test.conf")
			if (loadErr != nil) != tc.refused {
				t.Errorf("LoadConfig refused=%v, want %v (err: %v)", loadErr != nil, tc.refused, loadErr)
			}
			if result.Valid == tc.refused {
				t.Errorf("ze config validate valid=%v, want %v", result.Valid, !tc.refused)
			}
			if (loadErr != nil) == result.Valid {
				t.Errorf("the two callers disagree: LoadConfig err=%v, validate Valid=%v", loadErr, result.Valid)
			}
		})
	}
}

// TestLoadConfigRefusesAnISISHostnameOutside7BitASCII is AC-3, the RFC 5301
// section 3 path spec-fixit-isis-hostname-ascii depends on. Its own tests prove
// `ze config validate` refuses; this proves the loader does.
//
// RFC requirement: RFC5301-3-7 positive -- "The Value field is encoded in 7-bit
// ASCII" (Section 3). The loader is the enforcement point, so this is where the
// MUST is proven for a hand-edited config.
func TestLoadConfigRefusesAnISISHostnameOutside7BitASCII(t *testing.T) {
	const src = "isis {\n\tnet 49.0001.0000.0000.0001.00\n\thostname café.example\n}\n"
	_, err := config.LoadConfig(src, "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted an IS-IS hostname carrying a non-ASCII octet")
	}
	for _, want := range []string{"config validation failed", "isis", "0xc3", "7-bit ASCII"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %q: %s", want, err.Error())
		}
	}
}
