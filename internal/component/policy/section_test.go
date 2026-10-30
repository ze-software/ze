// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package policy

import (
	"errors"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestSectionsParserReadsBothRoots proves the parser a filter type uses when
// it reads a root beside the policy root. Method: deliveries with both roots,
// one root, none, an unrelated root and invalid JSON, checked for what parse
// receives and for the present flag the reload reads as "unchanged".
// VALIDATES: SectionsParser hands parse the unwrapped policy and read roots,
// reports present for either root, and refuses unreadable JSON.
// PREVENTS: a bgp-only reload being read as "no change" while the filter's
// export-chain check depends on the bgp root.
func TestSectionsParserReadsBothRoots(t *testing.T) {
	errInvalid := errors.New("invalid")
	type got struct{ policy, read map[string]any }
	parse := SectionsParser("test", errInvalid, "bgp", func(policyCfg, readCfg map[string]any) (got, error) {
		return got{policy: policyCfg, read: readCfg}, nil
	})

	cfg, present, err := parse([]sdk.ConfigSection{
		{Root: RootName, Data: `{"policy":{"prefix-list":{"P":{}}}}`},
		{Root: "bgp", Data: `{"bgp":{"peer":{}}}`},
	})
	if err != nil || !present {
		t.Fatalf("both roots: present %v, err %v", present, err)
	}
	if _, ok := cfg.policy["prefix-list"]; !ok {
		t.Fatalf("policy root not unwrapped: %v", cfg.policy)
	}
	if _, ok := cfg.read["peer"]; !ok {
		t.Fatalf("read root not unwrapped: %v", cfg.read)
	}

	cfg, present, err = parse([]sdk.ConfigSection{{Root: "bgp", Data: `{"bgp":{"peer":{}}}`}})
	if err != nil || !present {
		t.Fatalf("read root only: present %v, err %v", present, err)
	}
	if cfg.policy != nil {
		t.Fatalf("absent policy root: got %v, want nil", cfg.policy)
	}

	if _, present, err = parse([]sdk.ConfigSection{{Root: "isis", Data: `{"isis":{}}`}}); err != nil || present {
		t.Fatalf("unrelated root: present %v, err %v, want absent", present, err)
	}

	if _, _, err = parse([]sdk.ConfigSection{{Root: RootName, Data: "{not json"}}); !errors.Is(err, errInvalid) {
		t.Fatalf("invalid JSON: err %v, want errInvalid", err)
	}
}
