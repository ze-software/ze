// Design: docs/architecture/mrt.md — refuse unsupported replay encodings.
package analyze

import (
	"encoding/binary"
	"errors"

	"github.com/ze-software/ze/internal/mrt"
)

var errReplayAddPath = errors.New("MRT replay does not negotiate ADD-PATH; refusing Path-Identifier-bearing UPDATE")

// checkReplayUpdate checks the captured encoding before any bytes are emitted.
// RFC 7911 Section 5: "For a BGP speaker to be able to send multiple paths to its
// peer, that BGP speaker MUST advertise the ADD-PATH Capability with the
// Send/Receive field set to either 2 or 3, and MUST receive from its peer the
// ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the
// corresponding <AFI, SAFI>." Empty EOR has no Path Identifier to refuse.
func checkReplayUpdate(wire mrt.BGPMessage) error {
	parsed, err := mrt.ParseBGPMessage(wire)
	if err != nil {
		return err
	}
	if parsed.Update == nil {
		return nil
	}
	u := parsed.Update
	if len(u.AnnouncedPathIDs)+len(u.WithdrawnPathIDs) != 0 {
		return errReplayAddPath
	}
	for _, attr := range u.Attributes {
		if attr.Code != mrt.AttrMPReachNLRI && attr.Code != mrt.AttrMPUnreachNLRI {
			continue
		}
		if len(attr.Value) < 3 {
			return mrt.ErrShortData
		}
		addPath := u.AddPathFor(binary.BigEndian.Uint16(attr.Value[:2]), attr.Value[2])
		offset := 3
		if attr.Code == mrt.AttrMPReachNLRI {
			if len(attr.Value) < 5 {
				return mrt.ErrShortData
			}
			offset = 5 + int(attr.Value[3])
		}
		if offset > len(attr.Value) {
			return mrt.ErrShortData
		}
		if addPath && offset < len(attr.Value) {
			return errReplayAddPath
		}
	}
	_, _, err = countUpdateNLRIs(wire)
	return err
}
