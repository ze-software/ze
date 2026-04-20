// Design: docs/architecture/api/json-format.md — message formatting

package format

import (
	"encoding/json"

	bgpfilter "codeberg.org/thomas-mangin/ze/internal/component/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// FormatDecodeUpdateJSON formats a FilterResult as ze-bgp JSON for the decode-update RPC.
// Produces {"update":{"attr":{...},"nlri":{...}}} without peer/message metadata.
func FormatDecodeUpdateJSON(result bgpfilter.FilterResult, addPath bool) string {
	var scratch [1024]byte
	buf := scratch[:0]
	buf = append(buf, `{"update":{`...)

	// Attributes
	if len(result.Attributes) > 0 {
		buf = append(buf, `"attr":{`...)
		buf = appendAttributesJSON(buf, result)
		buf = append(buf, `},`...)
	}

	// Collect NLRI operations by family
	familyOps := make(map[string][]familyOperation)

	// MP-BGP announced routes
	for _, mp := range result.MPReach {
		nlris, err := mp.NLRIs(addPath)
		if err != nil || len(nlris) == 0 {
			continue
		}
		nhStr := mp.NextHop().String()
		familyOps[mp.Family().String()] = append(familyOps[mp.Family().String()], familyOperation{
			Action:  "add",
			NextHop: nhStr,
			NLRIs:   nlris,
		})
	}

	// MP-BGP withdrawn routes
	for _, mp := range result.MPUnreach {
		nlris, err := mp.NLRIs(addPath)
		if err != nil || len(nlris) == 0 {
			continue
		}
		familyOps[mp.Family().String()] = append(familyOps[mp.Family().String()], familyOperation{
			Action: "del",
			NLRIs:  nlris,
		})
	}

	// Legacy IPv4 announced
	if result.IPv4Announced != nil {
		nlris, err := result.IPv4Announced.NLRIs(addPath)
		if err == nil && len(nlris) > 0 {
			familyOps["ipv4/unicast"] = append(familyOps["ipv4/unicast"], familyOperation{
				Action:  "add",
				NextHop: result.IPv4Announced.NextHop().String(),
				NLRIs:   nlris,
			})
		}
	}

	// Legacy IPv4 withdrawn
	if result.IPv4Withdrawn != nil {
		nlris, err := result.IPv4Withdrawn.NLRIs(addPath)
		if err == nil && len(nlris) > 0 {
			familyOps["ipv4/unicast"] = append(familyOps["ipv4/unicast"], familyOperation{
				Action: "del",
				NLRIs:  nlris,
			})
		}
	}

	// Format NLRI operations
	buf = append(buf, `"nlri":{`...)
	buf = appendFamilyOpsJSON(buf, familyOps)
	buf = append(buf, `}}}`...)

	return string(buf)
}

// FormatNLRIsAsJSON formats a slice of NLRIs as a JSON array.
// Uses appendNLRIJSONValue for consistent formatting of all NLRI types.
func FormatNLRIsAsJSON(nlris []nlri.NLRI) json.RawMessage {
	var scratch [512]byte
	buf := scratch[:0]
	buf = append(buf, '[')
	var fam family.Family
	if len(nlris) > 0 {
		fam = nlris[0].Family()
	}
	for i, n := range nlris {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = appendNLRIJSONValue(buf, n, fam)
	}
	buf = append(buf, ']')
	result := make([]byte, len(buf))
	copy(result, buf)
	return json.RawMessage(result)
}
