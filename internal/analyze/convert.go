// Design: docs/architecture/mrt.md — MRT format conversion

package analyze

import "os"

const convertUsage = `ze-analyze convert -- convert MRT to other formats

Reads MRT records and converts them to alternative formats for analysis
with external tools.

Usage:
  ze-analyze convert pcap <input.mrt> <output.pcap>   Convert to pcap (Wireshark)
  ze-analyze convert bmp  <input.mrt> <output.bmp>    Convert to BMP stream

Status: not yet implemented.
`

func runConvert(args []string) int {
	if len(args) == 0 {
		os.Stderr.WriteString(convertUsage) //nolint:errcheck // usage output
		return 1
	}
	os.Stderr.WriteString("convert: not yet implemented\n") //nolint:errcheck // status
	return 1
}
