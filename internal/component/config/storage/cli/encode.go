// Design: docs/architecture/storage-backends.md -- shared netcapstring encoding.
package cli

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"strconv"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

type netcapPart uint8

const (
	netcapUnspecified netcapPart = iota
	netcapFull
	netcapHeader
	netcapCRC
)

// netcapCapacityMax leaves room for the frame header within one blob answer.
const netcapCapacityMax = command.BlobRemoteOctetsMax - 128

func dataEncode(args command.ValidatedArgs) command.LocalAnswer {
	if err := args.Context().Err(); err != nil {
		return netcapRefusal("the encoding request ended", err)
	}
	encoded, found := args.Value("content")
	if !found {
		return netcapRefusal("no input content was given", nil)
	}
	data, err := command.DecodeContent(encoded)
	if err != nil {
		return netcapRefusal("the input content could not be decoded", err)
	}
	part := netcapFull
	if value, found := args.Value("part"); found {
		switch value {
		case "full":
			part = netcapFull
		case "header":
			part = netcapHeader
		case "crc":
			part = netcapCRC
		default:
			return netcapRefusal("part must be full, header or crc", nil)
		}
	}
	capacity := len(data) + len(data)/10
	if value, found := args.Value("capacity"); found {
		capacity, err = strconv.Atoi(value)
		if err != nil {
			return netcapRefusal("capacity must be a non-negative byte count", err)
		}
		if capacity < 0 {
			return netcapRefusal("capacity must be a non-negative byte count", nil)
		}
	}
	if capacity > netcapCapacityMax {
		return netcapRefusal("capacity exceeds the space one remote answer carries", nil)
	}
	answer, err := encodeNetcap(data, capacity, part)
	if err != nil {
		return netcapRefusal("the input could not be encoded", err)
	}
	return command.LocalAnswer{Payload: command.Blob(answer)}
}

func netcapRefusal(problem string, cause error) command.LocalAnswer {
	return command.LocalRefusal(1, command.LocalErrorLine(opererr.Wrap(cause,
		"show data encode: "+problem,
		"content is padded base64 input; capacity must hold all decoded bytes and the answer must fit one response",
		"show data encode content <base64> part full")))
}

// encodeNetcap is shared by the stdin/string adapter and every command
// surface. CRC and header answers retain the local command's final newline.
func encodeNetcap(data []byte, capacity int, part netcapPart) ([]byte, error) {
	if capacity < len(data) {
		return nil, opererr.New("encoding capacity is less than the input length",
			"the frame must have room for every input byte", "show data encode content <base64> part full")
	}
	switch part {
	case netcapCRC:
		var checksum [4]byte
		binary.BigEndian.PutUint32(checksum[:], crc32.Checksum(data, zefs.CRC32cTable))
		return []byte(hex.EncodeToString(checksum[:]) + "\n"), nil
	case netcapHeader, netcapFull:
		encoded, err := zefs.EncodeNetcapstring(data, capacity)
		if err != nil {
			return nil, err
		}
		if part == netcapHeader {
			end := bytes.IndexByte(encoded, '\n')
			if end < 0 {
				return nil, opererr.New("the encoded frame has no header terminator",
					"the encoder did not produce a complete ZeFS frame", "ze doctor")
			}
			return encoded[:end+1], nil
		}
		return encoded, nil
	case netcapUnspecified:
		return nil, opererr.New("no encoded frame part was selected",
			"a frame, its header or its checksum must be selected", "show data encode content <base64> part full")
	default:
		panic("BUG: unknown internally produced netcapstring part")
	}
}
