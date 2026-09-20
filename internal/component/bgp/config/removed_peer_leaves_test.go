// Design: internal/component/bgp/yang/ze-bgp-conf.yang -- the peer schema
//
// The goal is that two leaves nothing read are refused rather than accepted.
// The method is the loader an operator reaches, config.LoadConfig, over config
// text: a leaf the schema no longer declares has to stop the load, because a
// leaf that commits and changes nothing is the failure being repaired here
// (ai/rules/principles.md).
package bgpconfig

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// peerWithout is the config the cases below add a removed leaf to. It loads
// clean, so a refusal in a case is the added leaf and nothing else.
const peerWithout = `bgp {
    group default {
        peer edge1 {
            connection {
                remote {
                    ip 192.0.2.1
                }
                local {
                    ip 192.0.2.2
                }
            }
            session {
                asn {
                    remote 65001
                }
            }
        }
    }
}
`

// peerWithConnectionLinkLocal adds the removed connection > link-local boolean.
// The live leaf of that name is session > link-local, which carries an address
// and which applyLinkLocal reads.
const peerWithConnectionLinkLocal = `bgp {
    group default {
        peer edge1 {
            connection {
                remote {
                    ip 192.0.2.1
                }
                local {
                    ip 192.0.2.2
                }
                link-local true
            }
            session {
                asn {
                    remote 65001
                }
            }
        }
    }
}
`

// peerWithProcessContentAttribute adds the removed attach > process > content >
// attribute leaf. parseProcessBindingsFromTree takes encoding and format off
// that container and nothing else, so the leaf promised a filter no event ever
// had applied to it.
const peerWithProcessContentAttribute = `bgp {
    group default {
        peer edge1 {
            connection {
                remote {
                    ip 192.0.2.1
                }
                local {
                    ip 192.0.2.2
                }
            }
            session {
                asn {
                    remote 65001
                }
            }
            attach process watcher {
                content {
                    encoding json
                    attribute next-hop
                }
            }
        }
    }
}
`

func TestRemovedPeerLeavesAreRefusedRatherThanAccepted(t *testing.T) {
	if _, err := config.LoadConfig(peerWithout, "test.conf", nil); err != nil {
		t.Fatalf("the baseline config must load, or no case below discriminates: %v", err)
	}

	for name, tc := range map[string]struct {
		text string
		word string
	}{
		// The control. A word no module has ever declared proves the loader
		// refuses an undeclared leaf at all, so the two cases under it are
		// about the schema rather than about a loader that refuses nothing.
		"a leaf no module ever declared":         {text: strings.Replace(peerWithConnectionLinkLocal, "link-local true", "frobnicate true", 1), word: "frobnicate"},
		"connection > link-local":                {text: peerWithConnectionLinkLocal, word: "link-local"},
		"attach > process > content > attribute": {text: peerWithProcessContentAttribute, word: "attribute"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := config.LoadConfig(tc.text, "test.conf", nil)
			if err == nil {
				t.Fatalf("%s was accepted; nothing reads it, so the commit changes no behavior", name)
			}
			// The word has to be in the message, otherwise the case is passing
			// on some unrelated refusal and proves nothing about the leaf.
			if !strings.Contains(err.Error(), tc.word) {
				t.Fatalf("%s was refused with %q, which does not name the leaf", name, err)
			}
		})
	}
}
