package bgpconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const externalDirectionConfig = `
bgp {
 router-id 192.0.2.2;
 session { asn { local 65000; } }
 peer upstream {
  connection { remote { ip 192.0.2.1; } local { ip 192.0.2.2; } }
  session { asn { remote 65010; } family { ipv4/unicast { mode enable; prefix { maximum 100; } } } }
  filter { %DIRECTION% { filters [ ext:65010 ]; final accept; } }
 }
}
`

func loadExternalDirection(direction string) error {
	tree, err := config.ParseTreeWithYANG(strings.ReplaceAll(externalDirectionConfig, "%DIRECTION%", direction), nil)
	if err != nil {
		return err
	}
	_, err = PeersFromConfigTree(tree)
	return err
}

// TestExternalDirectionAtConfigLoadAndStage1 drives real config lowering and
// the registered Stage 1 judge, both before declaration and on a reload after
// declaration. Exact and patterned filters preserve their declared direction.
func TestExternalDirectionAtConfigLoadAndStage1(t *testing.T) {
	for _, pattern := range []bool{false, true} {
		for _, direction := range []rpc.FilterDirection{rpc.FilterImport, rpc.FilterExport, rpc.FilterBoth, rpc.FilterDirectionUnspecified} {
			for _, attachment := range []string{"import", "export"} {
				for _, runtime := range []bool{false, true} {
					name := direction.String() + "/" + attachment
					if pattern {
						name += "/pattern"
					}
					if runtime {
						name += "/reload"
					}
					t.Run(name, func(t *testing.T) {
						clearPending(t)
						filter := rpc.FilterDecl{Name: "65010", Direction: direction, Reads: []string{"prefix"}}
						if pattern {
							filter.Name, filter.NamePattern = "", "[0-9]+"
						}
						filters := []rpc.FilterDecl{filter}
						if runtime {
							declareRuntime(t, "ext", filters)
						}
						err := loadExternalDirection(attachment)
						if !runtime {
							if err != nil {
								t.Fatalf("pre-declaration load: %v", err)
							}
							err = registry.CheckFilterDeclarations("ext", filters)
						}
						wrong := direction == rpc.FilterImport && attachment == "export"
						wrong = wrong || direction == rpc.FilterExport && attachment == "import"
						if !wrong {
							if err != nil {
								t.Fatalf("compatible chain refused: %v", err)
							}
							return
						}
						requireRefusal(t, err, "ext:65010", "bgp "+attachment, "direction", direction.String())
						if !runtime && !errors.Is(err, registry.ErrLoadedChainRefused) {
							t.Fatalf("startup refusal is not marked as a loaded-chain failure: %v", err)
						}
					})
				}
			}
		}
	}
}
