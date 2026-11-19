// Design: docs/architecture/fleet-interface.md -- the commands the sample console answers
// Overview: source.go -- the sample data source
// Related: views.go -- the peers and interfaces the answers print
//
// The sample console answers a fixed list of commands, each from the same
// facts the device's tabs show, so the console and the tabs agree. Any other
// command is refused with the list. The real console is fleet-ui-4's, over
// the command relay.

package sample

import (
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// consoleCommandMax is how much of a refused command the refusal quotes.
const consoleCommandMax = 64

// consoleRequest is what a command answers from: the device, the snapshot,
// and the names of every command, which only help prints.
type consoleRequest struct {
	row   *deviceRow
	snap  *snapshot
	names []string
}

// consoleCommand is one command the sample console answers.
type consoleCommand struct {
	name   string
	answer func(request *consoleRequest) string
}

// commandHelp is the command that lists the others, and the fix every
// refusal of the console names.
const commandHelp = "help"

var consoleCommands = []consoleCommand{
	{commandHelp, consoleHelp},
	{"show version", consoleVersion},
	{"show health", consoleHealth},
	{"show bgp peer list", consolePeers},
	{"show interface brief", consoleInterfaces},
	{"show system cpu", consoleCPU},
}

// consoleNames answers the names of the commands, in the order of the table.
func consoleNames() []string {
	names := make([]string, 0, len(consoleCommands))
	for at := range consoleCommands {
		names = append(names, consoleCommands[at].name)
	}

	return names
}

// consoleAnswer answers one command typed on a device, or refuses a command
// the sample console does not know with the list of those it does.
func consoleAnswer(row *deviceRow, snap *snapshot, command string) (fleetui.ConsoleAnswer, error) {
	typed := strings.Join(strings.Fields(command), " ")
	request := &consoleRequest{row: row, snap: snap, names: consoleNames()}

	for at := range consoleCommands {
		if consoleCommands[at].name != typed {
			continue
		}

		output := consoleCommands[at].answer(request)
		if len(output) > fleetui.ConsoleOutputBytesMax {
			return fleetui.ConsoleAnswer{Output: output[:fleetui.ConsoleOutputBytesMax], Truncated: true}, nil
		}

		return fleetui.ConsoleAnswer{Output: output}, nil
	}

	if len(typed) > consoleCommandMax {
		typed = typed[:consoleCommandMax]
	}
	problem := textbuf.New().Str("The sample console does not answer ").Quoted(typed).Str(".").Slice()
	why := textbuf.New().Str("The prototype answers a fixed list of commands from recorded data: ").
		Join(request.names, ", ").Str(".").Slice()

	return fleetui.ConsoleAnswer{}, opererr.New(problem, why, commandHelp)
}

// The widths of the columns of the console tables.
const (
	columnName    = 12
	columnAddress = 18
	columnWord    = 13
	columnNumber  = 10
)

// consoleHelp lists the commands.
func consoleHelp(request *consoleRequest) string {
	text := textbuf.New().Str("The sample console answers these commands:\n")
	for _, name := range request.names {
		text.Str("  ").Str(name).Byte('\n')
	}

	return text.Slice()
}

// consoleVersion prints what the device is and how long it has run.
func consoleVersion(request *consoleRequest) string {
	inventory := inventoryAt(request.row, request.snap)
	uptime := request.snap.now.Sub(inventory.StartedAt).Round(time.Minute)

	return textbuf.New().
		Str("ze ").Str(inventory.SoftwareVersion).Str(" (").Str(inventory.Build).Str(")\n").
		Str("model   ").Str(inventory.Model).Byte('\n').
		Str("serial  ").Str(inventory.Serial).Byte('\n').
		Str("kernel  ").Str(inventory.Kernel).Byte('\n').
		Str("uptime  ").Str(uptime.String()).Byte('\n').
		Slice()
}

// consoleHealth prints the device's health report.
func consoleHealth(request *consoleRequest) string {
	if request.row.name == newcomerName {
		return "no health report yet: the daemon is still starting its components\n"
	}

	text := textbuf.New()
	for _, component := range healthAt(request.row) {
		text.PadRight(component.Name, columnName).PadRight(component.State.String(), columnWord).
			Str(component.Detail).Byte('\n')
	}

	return text.Slice()
}

// consolePeers prints the device's BGP peers.
func consolePeers(request *consoleRequest) string {
	text := textbuf.New().
		PadRight("peer", columnName).PadRight("address", columnAddress).PadRight("as", columnNumber).
		PadRight("state", columnWord).PadRight("received", columnNumber).Str("sent\n")

	peers := peersAt(request.row, request.snap)
	for at := range peers {
		peer := &peers[at]
		text.PadRight(peer.Name, columnName).PadRight(peer.Address.String(), columnAddress).
			PadRight(textbuf.StringUint32(peer.RemoteAS), columnNumber).PadRight(peer.State, columnWord).
			PadRight(textbuf.StringUint(peer.PrefixesReceived), columnNumber).
			Uint(peer.PrefixesSent).Byte('\n')
	}

	return text.Slice()
}

// The words of the interface table.
const (
	wordUp        = "up"
	wordDown      = "down"
	wordAdminDown = "admin-down"
	wordNoAddress = "-"
)

// consoleInterfaces prints the device's interfaces.
func consoleInterfaces(request *consoleRequest) string {
	text := textbuf.New().
		PadRight("interface", columnName).PadRight("type", columnWord).PadRight("link", columnWord).
		Str("address\n")

	interfaces := interfacesAt(request.row, request.snap)
	for at := range interfaces {
		iface := &interfaces[at]

		link := wordDown
		if iface.LinkUp {
			link = wordUp
		}
		if !iface.AdminUp {
			link = wordAdminDown
		}

		address := wordNoAddress
		if len(iface.Addresses) > 0 {
			address = iface.Addresses[0].String()
		}

		text.PadRight(iface.Name, columnName).PadRight(iface.Kind, columnWord).PadRight(link, columnWord).
			Str(address).Byte('\n')
	}

	return text.Slice()
}

// consoleCPU prints the device's processor use, from its last sample.
func consoleCPU(request *consoleRequest) string {
	latest := latestAt(request.row, request.snap)

	return textbuf.New().
		Str("cpu    ").Float(latest.CPUPercent, 1).Str(" %\n").
		Str("load   ").Float2(latest.Load).Byte('\n').
		Str("cores  ").Int(int64(roleTable[request.row.role].cores)).Byte('\n').
		Slice()
}
