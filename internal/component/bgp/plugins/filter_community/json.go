// Design: docs/architecture/api/json-format.md -- community attribute JSON rendering

package filter_community

import (
	"encoding/hex"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

func appendCommunitiesJSON(buf []byte, attr attribute.Attribute) []byte {
	c, ok := attr.(*attribute.Communities)
	if !ok {
		return nil
	}
	buf = append(buf, '[')
	for i, comm := range *c {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = append(buf, comm.String()...)
		buf = append(buf, '"')
	}
	return append(buf, ']')
}

func appendLargeCommunitiesJSON(buf []byte, attr attribute.Attribute) []byte {
	lc, ok := attr.(*attribute.LargeCommunities)
	if !ok {
		return nil
	}
	buf = append(buf, '[')
	for i, comm := range *lc {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = append(buf, comm.String()...)
		buf = append(buf, '"')
	}
	return append(buf, ']')
}

func appendExtCommunitiesJSON(buf []byte, attr attribute.Attribute) []byte {
	ec, ok := attr.(*attribute.ExtendedCommunities)
	if !ok {
		return nil
	}
	buf = append(buf, '[')
	for i, comm := range *ec {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = hex.AppendEncode(buf, comm[:])
		buf = append(buf, '"')
	}
	return append(buf, ']')
}

func appendIPv6ExtCommunitiesJSON(buf []byte, attr attribute.Attribute) []byte {
	ec, ok := attr.(*attribute.IPv6ExtendedCommunities)
	if !ok {
		return nil
	}
	buf = append(buf, '[')
	for i, comm := range *ec {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = hex.AppendEncode(buf, comm[:])
		buf = append(buf, '"')
	}
	return append(buf, ']')
}
