// Design: docs/architecture/exabgp-bridge.md -- the shutdown line
// Related: bridge_script_version.go -- Event.Line writes every other line a script reads

package bridge

import (
	"os"
	"strconv"
)

// ShutdownLine is the line ExaBGP writes every process as the daemon stops,
// before it terminates them (Processes.terminate, src/exabgp/reactor/api/processes.py):
// JSON `{"exabgp": "...", "notification": "shutdown", "type": "notification"}`
// with the usual time, host and process ids (Response.JSON.shutdown), text
// `shutdown <pid> <ppid>` (Response.Text.shutdown). It names no neighbor, so a
// script reads it whatever its api block grants.
//
// The process ids are the bridge's own, the process the script is a child of,
// as ExaBGP's are the daemon's.
func ShutdownLine(format Format) ([]byte, error) {
	switch format {
	case FormatV4Text:
		line := append([]byte("shutdown "), strconv.Itoa(os.Getpid())...)
		line = append(line, ' ')
		line = strconv.AppendInt(line, int64(os.Getppid()), 10)
		return append(line, '\n'), nil
	case FormatV4JSON, FormatV6JSON:
		result, _ := Event{Kind: msgTypeNotification}.exabgpEnvelope(format)
		result[msgTypeNotification] = "shutdown"
		encoded, err := MarshalExabgpJSON(result)
		if err != nil {
			return nil, err
		}
		return append(encoded, '\n'), nil
	case FormatUnspecified:
		panic("BUG: bridge format unspecified")
	default:
		panic("BUG: invalid bridge format")
	}
}
