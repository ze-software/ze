// Design: docs/architecture/chaos-web-dashboard.md -- toasts arrive only through SSE.
package web

import (
	"errors"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"

	"github.com/ze-software/ze/internal/chaos/peer"
)

// TestToastMarkupHasOneBaseClass parses every event producer's toast, retaining
// the severity modifier and useful text while rejecting repeated class tokens.
func TestToastMarkupHasOneBaseClass(t *testing.T) {
	for _, kind := range []peer.EventType{peer.EventDisconnected, peer.EventReconnecting, peer.EventError, peer.EventChaosExecuted} {
		t.Run(kind.String(), func(t *testing.T) {
			toast, emitted := toastForEvent(peer.Event{Type: kind, PeerIndex: 3, ChaosAction: "tcp-disconnect", Err: errors.New("connection refused")})
			if !emitted {
				t.Fatal("noteworthy event produced no toast")
			}
			document, err := xhtml.Parse(strings.NewReader(renderToast(toast)))
			if err != nil {
				t.Fatal(err)
			}
			var classes []string
			var walk func(*xhtml.Node)
			walk = func(node *xhtml.Node) {
				for _, attribute := range node.Attr {
					if attribute.Key == "class" {
						fields := strings.Fields(attribute.Val)
						for _, name := range fields {
							if name == "toast" {
								classes = fields
							}
						}
					}
				}
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					walk(child)
				}
			}
			walk(document)
			if len(classes) != 2 {
				t.Fatalf("toast class tokens = %q, want base and severity", classes)
			}
			if classes[0] != "toast" {
				t.Fatalf("toast lacks its base class: %q", classes)
			}
			if classes[1] != toast.CSSClass {
				t.Fatalf("toast severity = %q, want %q", classes[1], toast.CSSClass)
			}
			if !strings.Contains(renderToast(toast), "p3 "+toast.Label) {
				t.Fatal("toast lost its peer and event label")
			}
		})
	}
}

// TestInitialDashboardHasNoToastChild proves the server-side precondition of
// the stream browser test before a live SSE client can race that observation.
func TestInitialDashboardHasNoToastChild(t *testing.T) {
	dashboard := newTestDashboard(3)
	defer dashboard.broker.Close()
	var page strings.Builder
	writeLayout(&page, dashboard)
	document, err := xhtml.Parse(strings.NewReader(page.String()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		for _, attribute := range node.Attr {
			if attribute.Key != "id" {
				continue
			}
			if attribute.Val != "toast-container" {
				continue
			}
			found = true
			if node.FirstChild != nil {
				t.Error("initial server page already contains a toast child")
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	if !found {
		t.Fatal("initial server page has no toast container")
	}
}
