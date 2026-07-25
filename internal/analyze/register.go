// Design: docs/architecture/system-architecture.md -- ze-analyze root handler registration

package analyze

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/subdispatch"
	zeversion "github.com/ze-software/ze/internal/core/version"
)

func init() {
	Register("download", runDownload, subdispatch.SubMeta{Desc: "Download MRT RIB dumps and BGP4MP updates from RIPE RIS / RouteViews"})
	Register("density", runDensity, subdispatch.SubMeta{Desc: "Analyze NLRI density per UPDATE and burst distribution"})
	Register("attributes", runAttributes, subdispatch.SubMeta{Desc: "Analyze attribute repetition patterns for caching decisions"})
	Register("communities", runCommunities, subdispatch.SubMeta{Desc: "Generate per-ASN community defaults from MRT files"})
	Register("count-attrs", runCountAttrs, subdispatch.SubMeta{Desc: "Count attributes per route (distribution table)"})
	Register("aspath", runASPath, subdispatch.SubMeta{Desc: "AS_PATH suffix sharing analysis (reversed trie compression)"})
	Register("mrt-dump", runMRTDump, subdispatch.SubMeta{Desc: "Dump MRT records as BGP UPDATE hex (one per line)"})
	Register("statistics", runStatistics, subdispatch.SubMeta{Desc: "MRT file statistics: type/subtype counts, AFI breakdown, peer summary"})
	Register("filter", runFilter, subdispatch.SubMeta{Desc: "Filter MRT records by peer, prefix, type, or timestamp to a new file"})
	Register("inject", runInject, subdispatch.SubMeta{Desc: "Inject MRT RIB entries into a running Ze instance"})
	Register("replay", runReplay, subdispatch.SubMeta{Desc: "Replay MRT over a BGP session to a remote peer"})
	Register("convert", runConvert, subdispatch.SubMeta{Desc: "Convert MRT to other formats (pcap, json)"})
	Register("export", runExport, subdispatch.SubMeta{Desc: "Export MRT data to network targets (bmp)"})
	Register("record", runRecord, subdispatch.SubMeta{Desc: "Record incoming protocol streams to MRT (bmp)"})
	Register("show", runShow, subdispatch.SubMeta{Desc: "Human-readable MRT record dump (like bgpdump)"})
	Register("routes", runRoutes, subdispatch.SubMeta{Desc: "Extract prefix table as JSON (prefix, next-hop, AS path, communities)"})
	Register("serve", runServe, subdispatch.SubMeta{Desc: "Serve MRT file contents over BGP to connecting peers"})

	registry.MustRegisterRootHandler("analyze", func(_ *registry.RuntimeContext, args []string) int {
		if len(args) == 1 && (args[0] == "--version" || args[0] == "-V") {
			fmt.Println(zeversion.Short())
			return 0
		}
		return Dispatch(args)
	}, registry.Meta{
		Description: "BGP MRT analysis tools",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subcommands,
	})
}
