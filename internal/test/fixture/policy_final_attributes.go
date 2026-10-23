// Design: docs/functional-tests.md -- consumer-visible modifications must retain unrelated attributes.
// Related: policy_final.go -- accepted ingress epochs call this after the processing barrier.
package fixture

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func policyFinalImportedAttributes(ctx context.Context, plugin *sdk.Plugin, prefix string, med byte) error {
	var stored struct {
		Peers map[string][]struct {
			Key        string `json:"key"`
			Attributes string `json:"attr-hex"`
			NextHop    string `json:"nhop-hex"`
		} `json:"adj-rib-in"`
	}
	status, err := Dispatch(ctx, plugin, "show bgp adj-rib-in", &stored)
	if err != nil {
		return err
	}
	if status != statusDone {
		return fmt.Errorf("show bgp adj-rib-in: status=%s", status)
	}
	for _, route := range stored.Peers["127.0.0.1"] {
		if route.Key != "ipv4/unicast:"+prefix {
			continue
		}
		if !strings.EqualFold(route.NextHop, "C0000201") {
			return fmt.Errorf("%s next hop changed: %s", prefix, route.NextHop)
		}
		attributes, err := hex.DecodeString(route.Attributes)
		if err != nil {
			return fmt.Errorf("%s stored attributes: %w", prefix, err)
		}
		wanted := map[attribute.AttributeCode][]byte{
			attribute.AttrOrigin:    {0},
			attribute.AttrASPath:    {2, 1, 0, 0, 253, 233},
			attribute.AttrMED:       {0, 0, 0, med},
			attribute.AttrCommunity: {253, 233, 0, 99},
		}
		iter := attribute.NewAttrIterator(attributes)
		for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
			if expected, exists := wanted[code]; exists {
				if !bytes.Equal(value, expected) {
					return fmt.Errorf("%s attribute %d: got %x, want %x", prefix, code, value, expected)
				}
				delete(wanted, code)
			}
		}
		if iter.Remaining() != 0 {
			return fmt.Errorf("%s has truncated stored attributes", prefix)
		}
		if len(wanted) != 0 {
			return fmt.Errorf("%s lost attributes: %v", prefix, wanted)
		}
		return nil
	}
	return fmt.Errorf("%s missing from source Adj-RIB-In", prefix)
}
