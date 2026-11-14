// Design: docs/architecture/api/commands.md — data (ZeFS) command ownership
//
// Register the `data` root command and the `show data *` local-data commands
// with the importable command registry. The caller's own process answers a
// local-data command, and the daemon serves the same handler under its YANG
// path. The daemon's own store maintenance (`delete data key`, `update data
// key`, `update data config`, `request data backup`, `restore` and `repair`) is
// a set of builtin RPCs, registered in register_data_rpc.go. Store-key
// management lives with internal/component/config/storage, not under cmd/ze.
package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// modeOffline marks local store access. Read-only inspection is permitted
// alongside a daemon; a local mutation requires exclusive store ownership. A
// running daemon mutates its store only through its own handle, by the
// builtin commands register_data_rpc.go registers.
const modeOffline = "offline"

func init() {
	registry.MustRegisterRootHandler("data", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		ShortHelp: "Store keys and blob artifact management",
		Mode:      modeOffline,
		Section:   registry.SectionConfiguration,
		Subs:      "backup, restore, write, import, rm, list, cat, registered, check, repair, encode",
	})
	// list and registered answer with DATA, so their answers reach the pipe
	// layer. They printed a table and returned an exit code, while YANG
	// declared a wire method for each that no daemon handler implements.
	command.MustRegisterLocalData("show data list", dataList, registry.Meta{
		ShortHelp: "List every matching store key recursively, optionally narrowed by prefix.",
		Mode:      modeOffline,
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show data registered", dataRegistered, registry.Meta{
		ShortHelp: "The key patterns the code declares, and what each one holds.",
		Mode:      modeOffline,
	}, command.RenderLocalAnswer)

	// `show data cat` answers the BYTES of one stored file, which may be YAML,
	// JSON, a certificate or a binary blob. Those bytes are the answer, so it
	// answers a command.Blob, prints them unchanged, and refuses the row
	// operators by name.
	//
	// A key may hold a secret (meta/web/key, the HTTPS private key; the
	// bcrypt hashes under meta/auth and meta/ssh; a config file's secrets), so
	// on the daemon the answer is authorized as more than a read: the built-in
	// read-only profile, which allows every `show`, refuses it before the
	// store is opened.
	command.MustRegisterLocalData("show data cat", dataCat, registry.Meta{NotReadOnly: true}, command.RenderLocalAnswer)

	// `show data check` answers the integrity verdict `ze data check` prints,
	// from the same rows, and exits 1 when any row is an error. It reads the
	// store through a reader handle, so it is a read, and the daemon serves it
	// under the same path with the daemon's writers held for the walk.
	command.MustRegisterLocalData("show data check", dataCheck, registry.Meta{
		ShortHelp: "Check every entry and the config history of the store.",
		Mode:      modeOffline,
	}, command.RenderLocalAnswer)

	command.RegisterShape([]string{"show data list", "show data registered"}, command.ShapeTab)
	// The verdict is ONE document, its rows inside it, as the config
	// validation verdict is, so the row operators are refused over it by name.
	command.RegisterShape([]string{"show data check"}, command.ShapeDoc)
	command.RegisterColumns([]string{"show data check"},
		command.ColumnOrder{"path", "entries", "corrupt-entries", "vanished-entries", "history-checked", "findings"},
		command.ColumnOrder{findingSeverity, findingCheck, findingKind, keywordKey, findingDetail})
	command.RegisterShape([]string{"show data cat"}, command.ShapeBlob)
	command.RegisterColumns([]string{"show data list"}, command.ColumnOrder{keywordKey})
	command.RegisterColumns([]string{"show data registered"},
		command.ColumnOrder{"pattern", "description"})
}
