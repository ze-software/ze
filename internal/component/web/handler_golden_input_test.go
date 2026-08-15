// Related: handler_golden_test.go -- the capture these inputs feed

package web

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/aaa"
	zeconfigcli "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/health"
)

// webGoldenConfig is the committed configuration every captured request reads.
// It carries one section per page family the workbench navigation offers, so a
// page renders a populated table rather than its empty state.
const webGoldenConfig = `bgp {
    router-id 1.2.3.4
    peer alpha {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
                accept false
            }
        }
        session {
            asn {
                local 65001
                remote 65002
            }
            router-id 1.2.3.4
        }
    }
    peer beta {
        connection {
            remote {
                ip 127.0.0.2
            }
            local {
                ip 127.0.0.1
                accept false
            }
        }
        session {
            asn {
                local 65001
                remote 65003
            }
            router-id 1.2.3.4
        }
    }
}
interface {
    backend netlink;
    ethernet eth0 {
        mac {
            address 00:11:22:33:44:55;
        }
        description "uplink to the exchange";
    }
    bridge br0 {
        mac {
            address 02:00:00:00:00:03;
        }
    }
}
firewall {
    backend nft;
    table golden {
        family inet;
        set blocked {
            type ipv4;
            element 10.0.0.1 {}
        }
        chain input {
            type filter;
            hook input;
            priority 0;
            policy accept;
            term block-known {
                from { source-address "@blocked"; }
                then { drop; }
            }
        }
    }
}
system {
    host golden
    domain example.net
}
`

// webUploadedConfig is what the upload case sends. It differs from the
// committed config. A handler that ignores the body answers what a handler that
// applies it answers, and one fixture then covers both. It is urlencoded
// because readUploadedConfig accepts a form field.
const webUploadedConfig = "bgp+%7B%0A++++router-id+5.6.7.8%0A%7D%0A"

// webGoldenDispatch answers the commands the pages issue with fixed output.
// The bytes a page renders then depend on the page, not on a running daemon.
func webGoldenDispatch() CommandDispatcher {
	return func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
		var out string

		switch {
		case strings.HasPrefix(cmd, "show bgp summary"):
			out = `[{"name":"alpha","peer-address":"127.0.0.1","remote-as":"65002","state":"established",` +
				`"routes-received":"100","routes-accepted":"95","routes-sent":"50"}]`
		case strings.HasPrefix(cmd, "show ospf"), strings.HasPrefix(cmd, "show isis"):
			out = `{"rows":[{"neighbor":"10.0.0.2","state":"full"}]}`
		case strings.HasPrefix(cmd, "show l2tp"):
			out = `{"sessions":[{"id":"1","login":"user1","state":"established"}]}`
		default:
			out = `{"result":"golden"}`
		}

		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
	}
}

// webGoldenHash is the operator's bcrypt hash, at the cheapest cost the library
// offers. It is built once. A hash carries a random salt and nothing renders
// it, so one hash serves every case.
var (
	webGoldenHashOnce sync.Once
	webGoldenHashText string
	webGoldenHashErr  error
)

func webGoldenHash(t *testing.T) string {
	t.Helper()

	webGoldenHashOnce.Do(func() {
		hash, err := bcrypt.GenerateFromPassword([]byte(webGoldenPassword), bcrypt.MinCost)
		webGoldenHashText, webGoldenHashErr = string(hash), err
	})

	if webGoldenHashErr != nil {
		t.Fatalf("hash the capture password: %v", webGoldenHashErr)
	}

	return webGoldenHashText
}

// aaaAuthorizer names the interface the wraps take, so the capture can pass a
// nil one for the normal server.
type aaaAuthorizer = aaa.Authorizer

// webGoldenDenyEdits refuses every command, which is what a read-only session
// meets. RequireEditAuthz answers 403 and its body is bytes an operator reads.
type webGoldenDenyEdits struct{}

func (webGoldenDenyEdits) Authorize(_, _, _ string, _ bool) bool { return false }

// webGoldenValidate is the validator the hub gives the upload handler. Its
// error text reaches the response, so the capture uses the real one.
func webGoldenValidate(content, path string) error {
	return zeconfigcli.ValidateContent(content, path)
}

// webGoldenHealthHandler serves a fixed health registry rather than the
// process-wide one. DefaultRegistry holds what the linked packages registered,
// and a build tag changes that set. A fixture taken from it would go red on a
// build that links one component more.
func webGoldenHealthHandler() http.Handler {
	registry := &health.Registry{}
	registry.Register("bgp", func() (health.Status, string) { return health.StatusHealthy, "" })
	registry.Register("web", func() (health.Status, string) { return health.StatusDegraded, "no certificate" })

	return registry.Handler()
}

// webGoldenDiffHandler and webGoldenDiffCloseHandler mirror the two closures
// startWebServer writes inline. They are two of the four renderer entry points
// the hub reaches from outside this package. A port that leaves them behind
// breaks the diff modal with every in-package test green.
func webGoldenDiffHandler(renderer *Renderer, mgr *EditorManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)

			return
		}

		diff, _ := mgr.Diff(username)
		count := mgr.ChangeCount(username)

		html, renderErr := renderer.RenderDiffModalOpen(diff, count)
		if renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if _, err := w.Write([]byte(html)); err != nil {
			return
		}
	})
}

func webGoldenDiffCloseHandler(renderer *Renderer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		html, renderErr := renderer.RenderDiffModal()
		if renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if _, err := w.Write([]byte(html)); err != nil {
			return
		}
	})
}
