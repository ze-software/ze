// Design: docs/features/dns-resolver.md -- show system dns
// Related: register.go -- registers the handler below

package cmd

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/dnssource"
)

// routerConsumer is the consumer name the router's own selection is shown
// under: it is the system dns-source list, published in the snapshot rather
// than by a plugin.
const routerConsumer = "system"

// systemDNSAnswer is the show system dns answer: every learned record, then
// every consumer's selections, the router's first.
type systemDNSAnswer struct {
	Records   []dnssource.Record      `json:"records"`
	Consumers []dnssource.Publication `json:"consumers"`
}

func registerShowSystemDNS() {
	pluginserver.RegisterRPCs(pluginserver.RPCRegistration{
		WireMethod: "ze-system-dns:show-system-dns",
		Handler: func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
			return showSystemDNS(system.ProcessResolvers())
		},
	})
}

// showSystemDNS answers registry's records and selections. It runs in the
// daemon, where the registry is the live one the lease producers and the
// consumers' publications fill.
func showSystemDNS(registry *system.Resolvers) (*plugin.Response, error) {
	b, err := json.Marshal(newSystemDNSAnswer(registry.Snapshot(), registry.Consumers()))
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "show system dns: encode: " + err.Error()}, nil //nolint:nilerr // operational error propagated via Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.RawJSON(b)}, nil
}

// newSystemDNSAnswer puts the router's selection first, under routerConsumer,
// then every published consumer in the registry's order.
func newSystemDNSAnswer(snapshot dnssource.Snapshot, consumers []dnssource.Publication) systemDNSAnswer {
	answer := systemDNSAnswer{
		Records:   snapshot.Records,
		Consumers: make([]dnssource.Publication, 0, len(consumers)+1),
	}
	if answer.Records == nil {
		answer.Records = []dnssource.Record{}
	}
	answer.Consumers = append(answer.Consumers, dnssource.Publication{
		Consumer:   routerConsumer,
		Selections: []dnssource.ConsumerSelection{snapshot.Router},
	})
	answer.Consumers = append(answer.Consumers, consumers...)
	return answer
}
