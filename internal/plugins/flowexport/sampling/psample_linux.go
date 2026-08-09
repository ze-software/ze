// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- psample generic netlink reader

//go:build linux

package sampling

import (
	"fmt"

	"github.com/mdlayher/genetlink"
)

const psampleFamilyName = "psample"

// PsampleReader receives sampled packet metadata from the kernel
// psample module via generic netlink multicast.
type PsampleReader struct {
	conn   *genetlink.Conn
	family genetlink.Family
}

// NewPsampleReader connects to the psample generic netlink family and
// joins the sample multicast group.
func NewPsampleReader() (*PsampleReader, error) {
	conn, err := genetlink.Dial(nil)
	if err != nil {
		return nil, fmt.Errorf("psample: dial genetlink: %w", err)
	}

	family, err := conn.GetFamily(psampleFamilyName)
	if err != nil {
		closeErr := conn.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("psample: get family %q: %w (close: %w)", psampleFamilyName, err, closeErr)
		}
		return nil, fmt.Errorf("psample: get family %q: %w", psampleFamilyName, err)
	}

	var groupID uint32
	for _, g := range family.Groups {
		if g.Name == "packets" {
			groupID = g.ID
			break
		}
	}
	if groupID == 0 {
		closeErr := conn.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("psample: multicast group %q not found (close: %w)", "packets", closeErr)
		}
		return nil, fmt.Errorf("psample: multicast group %q not found in family %q", "packets", psampleFamilyName)
	}

	if err := conn.JoinGroup(groupID); err != nil {
		closeErr := conn.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("psample: join group %d: %w (close: %w)", groupID, err, closeErr)
		}
		return nil, fmt.Errorf("psample: join group %d: %w", groupID, err)
	}

	return &PsampleReader{conn: conn, family: family}, nil
}

// Read blocks until a psample message arrives and returns the parsed
// SampledPacket. The returned Header slice is a copy owned by the caller.
func (r *PsampleReader) Read() (SampledPacket, error) {
	msgs, _, err := r.conn.Receive()
	if err != nil {
		return SampledPacket{}, fmt.Errorf("psample: receive: %w", err)
	}

	for _, msg := range msgs {
		pkt, parseErr := parsePsampleMessage(msg.Data)
		if parseErr != nil {
			continue
		}
		return pkt, nil
	}

	return SampledPacket{}, fmt.Errorf("psample: no valid sample in batch of %d messages", len(msgs))
}

// Close shuts down the genetlink connection.
func (r *PsampleReader) Close() error {
	return r.conn.Close()
}
