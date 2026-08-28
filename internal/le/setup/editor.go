// Design: docs/architecture/core-design.md -- the language-server plugins the LSP tool needs
//
// The LSP tool does not contain a hardcoded set of languages. Each plugin tells
// the harness which server binary to run for specified file extensions. If a
// language has no installed plugin, every call returns
// "No LSP server available for file type: <ext>".
//
// On this machine, Python was in that state while Go worked. gopls-lsp was
// installed, but pyright-lsp was not. Thus, the tool answered .go questions and
// refused .py questions. Both servers were on PATH. The binary probe and the
// answering probe in servers.go did not detect the problem. The harness lacked
// the information that the server exists.
//
// THIS DELIBERATELY DOES NOT DO TWO THINGS. First, it does not install the
// plugin. When `claude plugin ...` runs from inside a session, it does not
// return. Measurements show that it hangs until killed. A setup program that
// runs it in a shell would therefore hang the run that it must finish. Instead,
// this code reports the condition and gives the command. The KVM group and all
// PATH fixes use the same method. Second, it does not create a project-local
// plugin as a workaround. A plugin must still be enabled before it takes
// effect. Thus, the workaround requires the same manual step and adds a second
// definition of a plugin that the marketplace publishes.

package setup

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// officialMarketplace is where the official language-server plugins are
// published.
const officialMarketplace = "claude-plugins-official"

// lspPlugin defines one language-server plugin and the file types it supports.
//
// Binary is the server that the plugin runs. The report lists it to distinguish
// two failures. Either the plugin is missing, or the plugin is installed but
// its named binary is missing. These failures require different corrections.
// The tool table already installs the binaries.
type lspPlugin struct {
	Plugin     string
	Binary     string
	Extensions []string
	Why        string
}

// qualified answers the name an install command takes.
func (p lspPlugin) qualified() string {
	var tb textbuf.Buffer
	return tb.Str(p.Plugin).Byte('@').Str(officialMarketplace).String()
}

// installCommand gives the command that a person must run to install it.
//
// Use the slash command, not the shell command. When `claude plugin ...` runs
// from a shell inside a session, it does not return. The reader can use the
// slash command from that session.
func (p lspPlugin) installCommand() string {
	var tb textbuf.Buffer
	return tb.Str("/plugin install ").Str(p.qualified()).String()
}

// lspPlugins lists the language-server plugins this Go repository needs.
func lspPlugins() []lspPlugin {
	return []lspPlugin{{
		Plugin:     "gopls-lsp",
		Binary:     toolGopls,
		Extensions: []string{".go"},
		Why:        "every LSP call on a .go file is refused without it",
	}}
}

// pluginRecord answers where the harness records what is installed, for every
// scope.
func (s *Setup) pluginRecord() string {
	return filepath.Join(s.home(), ".claude", "plugins", "installed_plugins.json")
}

// installedPlugins returns all installed plugins by their name@marketplace key.
//
// If the record is absent or unreadable, this function returns the empty set
// instead of an error. The harness does not always write a record before setup
// runs, so "nothing installed" is the correct interpretation. This is also the
// safe interpretation. The caller reports a plugin that it cannot see as
// PENDING, which fails the run instead of passing it.
func (s *Setup) installedPlugins() map[string]bool {
	raw, err := os.ReadFile(s.pluginRecord())
	if err != nil {
		return nil
	}
	var record struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil
	}
	have := make(map[string]bool, len(record.Plugins))
	for name := range record.Plugins {
		have[name] = true
	}
	return have
}

// missingLSPPlugins answers the language-server plugins this machine does not
// have, in the order lspPlugins declares them.
func (s *Setup) missingLSPPlugins() []lspPlugin {
	have := s.installedPlugins()
	var missing []lspPlugin
	for _, plugin := range lspPlugins() {
		if !have[plugin.qualified()] {
			missing = append(missing, plugin)
		}
	}
	return missing
}
