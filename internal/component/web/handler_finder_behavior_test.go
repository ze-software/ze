// Design: docs/architecture/web-interface.md -- URL scheme and authentication
// Related: handler_golden_test.go -- shared server, request and registered schema

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/ze-software/ze/internal/component/config"
)

// These are request contracts, not a census of registered schema children.
// In particular, a plugin adding a container must become navigable without
// requiring a new copy of the entire Finder document.
var webFinderBehaviorCases = []webHandlerCase{
	{Name: "get-fragment-detail", Method: http.MethodGet, Target: "/fragment/detail?path=bgp", HTMX: true},
	{Name: "get-fragment-detail-page", Method: http.MethodGet, Target: "/fragment/detail?path=bgp"},
	{Name: "get-show-bgp-finder", Method: http.MethodGet, Target: "/show/bgp/peer/",
		Cookies: map[string]string{"ze-ui-mode": "finder"}},
}

// VALIDATES: authenticated Finder GETs negotiate a document or OOB fragment,
// expose registered navigation and configured data, and keep links usable both
// through HTMX and as direct history URLs. Browsing needs no edit permission.
// PREVENTS: an inventory-only golden rejecting a valid plugin registration while
// missing the request/response contracts that make navigation usable.
func TestWebFinderHandlerBehavior(t *testing.T) {
	for _, c := range webFinderBehaviorCases {
		t.Run(c.Name, func(t *testing.T) {
			for _, readOnly := range []bool{false, true} {
				name := "editable"
				if readOnly {
					name = "read-only"
				}
				t.Run(name, func(t *testing.T) {
					env := newWebGoldenEnv(t, readOnly)
					rec := httptest.NewRecorder()
					env.handler.ServeHTTP(rec, webGoldenRequest(t, env, c))
					path := "bgp"
					if c.Target == "/show/bgp/peer/" {
						path = "bgp/peer"
					}
					doc := assertFinderResponse(t, rec, path, "bgp", c.HTMX)
					finder := finderElement(t, doc, "id", "finder")

					// Read the live schema rather than pinning its names or size.
					_, schema, tree, _ := webGoldenParts(t)
					children, ok := schema.Get("bgp").(childLister)
					require.True(t, ok, "the configured BGP root must be browsable")
					for _, name := range children.Children() {
						child := children.Get(name)
						if child.Kind() == config.NodeLeaf {
							continue
						}
						link := finderElement(t, finder, "href", "/show/bgp/"+name+"/")
						assertFinderLink(t, link)
					}
					selected := finderElement(t, finder, "href", "/show/bgp/")
					assert.Contains(t, strings.Fields(finderAttr(selected, "class")), "selected")

					var next *html.Node
					nextContext := "bgp" // A list index keeps the CLI in its parent.
					if path == "bgp" {
						input := finderElement(t, finderElement(t, doc, "id", "detail"), "id", fieldInputID("bgp", "router-id"))
						assert.Equal(t, "1.2.3.4", finderAttr(input, "value"))
						assert.Equal(t, "/config/set/bgp", finderAttr(input, "hx-post"))
						var values map[string]string
						require.NoError(t, json.Unmarshal([]byte(finderAttr(input, "hx-vals")), &values))
						assert.Equal(t, "router-id", values["leaf"])
						next = finderElement(t, finder, "href", "/show/bgp/peer/")
					} else {
						assert.Contains(t, strings.Fields(finderAttr(finderElement(t, finder, "href", "/show/bgp/peer/"), "class")), "selected")
						detail := finderElement(t, doc, "id", "detail")
						// Read the fixture directly: the renderer's list-key helper
						// must not also decide which missing rows this test notices.
						bgp := tree.GetContainer("bgp")
						require.NotNil(t, bgp)
						keys := bgp.ListKeys("peer")
						require.NotEmpty(t, keys, "configured peers must exercise list navigation")
						assert.ElementsMatch(t, keys, finderAttributeValues(detail, "data-row-key"),
							"the peer table must render exactly the configured entries")
						for _, key := range keys {
							row := finderElement(t, detail, "data-row-key", key)
							assert.Equal(t, "tr", row.Data)
							link := finderElement(t, row, "href", "/show/bgp/peer/"+key+"/")
							assert.Equal(t, key, finderText(link))
							assertFinderLink(t, link)
							if next == nil {
								next = link
								nextContext = "bgp/peer/" + key
							}
						}
						add := finderElement(t, detail, "id", "btn-add-entry")
						assert.Equal(t, "/config/add-form/bgp/peer/", finderAttr(add, "hx-get"))
						assert.Equal(t, "body", finderAttr(add, "hx-target"))
						assert.Equal(t, "beforeend", finderAttr(add, "hx-swap"))
					}

					// Follow the rendered action, then reload the pushed URL. No
					// parallel list of navigation destinations feeds these requests.
					assertFinderLink(t, next)
					for _, htmx := range []bool{true, false} {
						target := finderAttr(next, "href")
						if htmx {
							target = finderAttr(next, "hx-get")
						}
						follow := webHandlerCase{Method: http.MethodGet, Target: target, HTMX: htmx,
							Cookies: map[string]string{"ze-ui-mode": "finder"}}
						answer := httptest.NewRecorder()
						env.handler.ServeHTTP(answer, webGoldenRequest(t, env, follow))
						nextPath := strings.Trim(strings.TrimPrefix(finderAttr(next, "href"), "/show/"), "/")
						assertFinderResponse(t, answer, nextPath, nextContext, htmx)
					}

					if readOnly {
						// Finder browsing is not edit authorization: the server must
						// still refuse a submitted change, even if an input is visible.
						edit := webHandlerCase{Method: http.MethodPost, Target: "/config/set/bgp/",
							Form: "leaf=router-id&value=9.9.9.9", HTMX: true}
						denied := httptest.NewRecorder()
						env.handler.ServeHTTP(denied, webGoldenRequest(t, env, edit))
						assert.Equal(t, http.StatusForbidden, denied.Code)
					}
				})
			}

			t.Run("anonymous", func(t *testing.T) {
				env := newWebGoldenEnv(t, false)
				anonymous := c
				anonymous.Anonymous = true
				rec := httptest.NewRecorder()
				env.handler.ServeHTTP(rec, webGoldenRequest(t, env, anonymous))
				require.Equal(t, http.StatusUnauthorized, rec.Code)
				doc := parseFinderHTML(t, rec.Body.String())
				assert.Empty(t, finderElements(doc, "id", "finder"), "unauthenticated requests must not disclose configuration")
				form := finderElement(t, doc, "action", "/login")
				assert.Equal(t, "post", strings.ToLower(finderAttr(form, "method")))
				finderElement(t, form, "name", "username")
				finderElement(t, form, "name", "password")
				assert.Equal(t, c.Target, finderAttr(finderElement(t, form, "name", "return_to"), "value"))
			})
		})
	}
}

// VALIDATES: configuration text reaches a Finder field as a value, not elements
// or attributes, for both negotiated response forms.
// PREVENTS: an operator-controlled string becoming executable browser markup.
func TestWebFinderEscapesFieldValues(t *testing.T) {
	const value = `"><img src=x onerror=alert(1)> & 'quoted'`
	renderer, _, _, _ := webGoldenParts(t)
	schema := config.NewSchema()
	schema.Define("example", config.Container(config.Field("description", config.Leaf(config.TypeString))))
	tree := config.NewTree()
	container := config.NewTree()
	container.Set("description", value)
	tree.SetContainer("example", container)
	handler := HandleFragment(renderer, schema, tree, nil, false)
	for _, htmx := range []bool{false, true} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/fragment/detail?path=example", http.NoBody)
		if htmx {
			req.Header.Set("HX-Request", "true")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		doc := assertFinderResponse(t, rec, "example", "example", htmx)
		input := finderElement(t, doc, "id", fieldInputID("example", "description"))
		assert.Equal(t, value, finderAttr(input, "value"))
		assert.Empty(t, finderElements(doc, "src", "x"))
		assert.Empty(t, finderElements(doc, "onerror", "alert(1)"))
	}
}

func assertFinderResponse(t *testing.T, rec *httptest.ResponseRecorder, path, cliContext string, htmx bool) *html.Node {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	body := rec.Body.String()
	require.True(t, markupHasClass(body, "main-split"), "navigation must contain the Finder/detail split")
	doc := parseFinderHTML(t, body)
	finderElement(t, doc, "id", "finder")
	finderElement(t, doc, "id", "detail")
	breadcrumb := finderElement(t, doc, "id", "breadcrumb")
	assert.Contains(t, finderText(breadcrumb), path[strings.LastIndex(path, "/")+1:])
	if htmx {
		assert.NotContains(t, strings.ToLower(body), "<!doctype")
		assert.Empty(t, finderElements(doc, "src", "/assets/htmx.min.js"), "a swap must not reload the document assets")
		for id, swap := range map[string]string{
			"breadcrumb": "innerHTML", "cli-prompt": "innerHTML", "cli-path-bar": "innerHTML", "cli-context-path": "true",
		} {
			assert.Equal(t, swap, finderAttr(finderElement(t, doc, "id", id), "hx-swap-oob"), id)
		}
		assert.Equal(t, cliContext, finderText(finderElement(t, doc, "id", "cli-context-path")))
	} else {
		assert.Contains(t, strings.ToLower(body), "<!doctype html>")
		finderElement(t, doc, "id", "content-area")
		finderElement(t, doc, "src", "/assets/htmx.min.js")
		assert.Empty(t, finderAttr(breadcrumb, "hx-swap-oob"))
	}
	return doc
}

func assertFinderLink(t *testing.T, link *html.Node) {
	t.Helper()
	require.Equal(t, "a", link.Data)
	href := finderAttr(link, "href")
	require.True(t, strings.HasPrefix(href, "/show/"), "Finder links must support direct navigation: %q", href)
	assert.Equal(t, href, finderAttr(link, "hx-push-url"))
	assert.Equal(t, "#content-area", finderAttr(link, "hx-target"))
	assert.Equal(t, "innerHTML", finderAttr(link, "hx-swap"))
	get, err := url.Parse(finderAttr(link, "hx-get"))
	require.NoError(t, err)
	assert.Empty(t, get.Host)
	assert.Empty(t, get.Scheme)
	assert.Equal(t, "/fragment/detail", get.Path)
	assert.Equal(t, strings.Trim(strings.TrimPrefix(href, "/show/"), "/"), get.Query().Get("path"))
}

func parseFinderHTML(t *testing.T, body string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	require.NoError(t, err)
	return doc
}

func finderElement(t *testing.T, root *html.Node, attr, value string) *html.Node {
	t.Helper()
	matches := finderElements(root, attr, value)
	require.Len(t, matches, 1, "expected one element with %s=%q", attr, value)
	return matches[0]
}

func finderElements(root *html.Node, attr, value string) []*html.Node {
	var matches []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && finderAttr(node, attr) == value {
			matches = append(matches, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return matches
}

func finderAttributeValues(root *html.Node, name string) []string {
	var values []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			for _, attr := range node.Attr {
				if attr.Key == name {
					values = append(values, attr.Val)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return values
}

func finderAttr(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func finderText(node *html.Node) string {
	var text strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.TrimSpace(text.String())
}
