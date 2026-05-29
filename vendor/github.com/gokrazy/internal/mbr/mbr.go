// Package mbr provides a configured version of Sebastian Plotz’s minimal
// stage1-only Linux bootloader, to be written to a Master Boot Record.
package mbr

import (
	"bytes"
	"encoding/binary"

	_ "embed"
)

//go:generate make

//go:embed bootloader.img
var mbr []byte

// write to byte offset 433
type bootloaderParams struct {
	CurrentLBA uint32 // e.g. 8218
	CmdlineLBA uint32 // e.g. 8218
}

func Configure(vmlinuzLba, cmdlineLba uint32, partuuid uint32) [446]byte {
	buf := bytes.NewBuffer(make([]byte, 0, 446))
	// buf.Write never fails
	buf.Write(mbr[:432])
	params := bootloaderParams{
		CurrentLBA: vmlinuzLba,
		CmdlineLBA: cmdlineLba,
	}
	binary.Write(buf, binary.LittleEndian, &params)
	if pad := 440 - buf.Len(); pad > 0 {
		buf.Write(bytes.Repeat([]byte{0}, pad))
	}
	// disk signature (for PARTUUID= root device Linux kernel parameter)
	binary.Write(buf, binary.LittleEndian, partuuid)
	binary.Write(buf, binary.LittleEndian, uint16(0x0000))
	var b [446]byte
	copy(b[:], buf.Bytes())
	return b
}
