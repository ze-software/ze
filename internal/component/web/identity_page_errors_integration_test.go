// Design: docs/architecture/web-interface.md -- delegated workbench event targets.
//go:build integration

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
)

// TestBrowserIdentityLoadSaveHasNoPageErrors follows the original journal's
// identity load and Save with Chromium's exception collector active before
// navigation. Candidate readback and a fresh document prove Save still works.
// MUTATION: an unguarded target.closest or target.matches in a delegated
// document listener reports a native page error when its target is Document.
func TestBrowserIdentityLoadSaveHasNoPageErrors(t *testing.T) {
	browser, mgr, address := newIdentityPageBrowser(t)
	browser.command("open", address+"/show/system/identity/")
	browser.wait(`!!document.getElementById('workbench-shell')&&!!document.getElementById('field-hostname')`, "rendered identity form")
	assert.Empty(t, identityPageErrors(t, browser), "identity load must not report PAGEERROR")
	browser.evaluate(`window.__identityBeforeSave=true;true`)
	browser.command("fill", "#field-hostname", "page-error-host")
	browser.command("fill", "#field-domain", "page-error.example")
	browser.command("click", "#workbench-form-save")
	require.Eventually(t, func() bool {
		diff, err := mgr.Diff("alice")
		if err != nil {
			return false
		}
		return strings.Contains(diff, "system host page-error-host") && strings.Contains(diff, "system domain page-error.example")
	}, 5*time.Second, 20*time.Millisecond, "the real form save must reach both candidate leaves")
	assert.Empty(t, identityPageErrors(t, browser), "identity save must not report PAGEERROR")
	// The old document marker disappears before the new form finishes loading.
	// Wait for both controls before reading their values during that transition.
	browser.wait(`(function(){
		if(typeof window.__identityBeforeSave!=='undefined')return false;
		if(document.readyState!=='complete')return false;
		var hostname=document.getElementById('field-hostname');
		var domain=document.getElementById('field-domain');
		return !!hostname&&!!domain&&hostname.value==='page-error-host'&&domain.value==='page-error.example';
	})()`, "successful form reload with saved values")
	assert.Empty(t, identityPageErrors(t, browser), "post-save identity reload must not report PAGEERROR")
}

// TestBrowserDelegatedListenersAcceptNonElementTargets exercises the contract
// behind the original PAGEERROR: document listeners also receive Document and
// Text targets. Real Element clicks remain actionable after those events.
// This is an explicit event-boundary regression, not a claim that each event
// below arises during an ordinary identity save.
func TestBrowserDelegatedListenersAcceptNonElementTargets(t *testing.T) {
	browser, _, address := newIdentityPageBrowser(t)
	browser.command("open", address+"/show/system/identity/")
	browser.wait(`!!document.getElementById('theme-toggle')`, "identity action controls")
	browser.command("errors", "--clear")
	for _, target := range []string{"document", "text"} {
		t.Run(target, func(t *testing.T) {
			for _, event := range []string{"click", "change", "mouseenter", "mouseleave", "htmx:after:request", "keydown"} {
				t.Run(event, func(t *testing.T) {
					browser.command("errors", "--clear")
					script := `(function(){var target=document;if(%q==='text'){target=document.createTextNode('event boundary');document.body.appendChild(target);}var event=new Event(%q,{bubbles:true});Object.defineProperty(event,'key',{value:'Escape'});target.dispatchEvent(event);if(target!==document)target.remove();return true;})()`
					require.Equal(t, "true", browser.evaluate(fmt.Sprintf(script, target, event)))
					assert.Empty(t, identityPageErrors(t, browser), "non-Element %s target for %s must not throw", target, event)
				})
			}
		})
	}
	// The target guard must retain real delegated Element actions.
	before := browser.evaluate(`document.documentElement.getAttribute('data-theme')`)
	browser.command("click", "#theme-toggle")
	assert.NotEqual(t, before, browser.evaluate(`document.documentElement.getAttribute('data-theme')`))
	assert.Empty(t, identityPageErrors(t, browser), "a real Element action must still work without an exception")
	// SVG is an Element too: clicking a nested icon must reach its action button.
	before = browser.evaluate(`document.documentElement.getAttribute('data-theme')`)
	require.Equal(t, "true", browser.evaluate(`(function(){
		var icon=document.createElementNS('http://www.w3.org/2000/svg','svg');
		icon.id='identity-theme-svg';
		icon.setAttribute('width','12');
		icon.setAttribute('height','12');
		document.getElementById('theme-toggle').appendChild(icon);
		return true;
	})()`))
	browser.command("click", "#identity-theme-svg")
	assert.NotEqual(t, before, browser.evaluate(`document.documentElement.getAttribute('data-theme')`))
	assert.Empty(t, identityPageErrors(t, browser), "an SVG action target must remain actionable without an exception")
	browser.evaluate(`document.getElementById('identity-theme-svg').remove();true`)
	// The existing recent-events filter is the real submit-on-change consumer.
	// Selecting its All Namespaces option must perform a native GET submission.
	browser.command("open", address+"/show/events/")
	browser.wait(`!!document.querySelector('[data-action="submit-on-change"]')`, "real events namespace filter")
	browser.evaluate(`window.__eventsBeforeChange=true;true`)
	browser.command("select", `[data-action="submit-on-change"]`, "")
	browser.wait(`typeof window.__eventsBeforeChange==='undefined'&&new URL(window.location.href).searchParams.has('namespace')`, "Element change submits its real form")
	assert.Empty(t, identityPageErrors(t, browser), "Element change submission must retain its normal behavior")
}

// TestBrowserPageErrorCollectorObservesListenerFailure calibrates the native
// exception oracle independently of the product: dispatchEvent does not throw
// an exception from its listener back to eval, but Chromium reports PAGEERROR.
func TestBrowserPageErrorCollectorObservesListenerFailure(t *testing.T) {
	browser, _, _ := newIdentityPageBrowser(t)
	require.Equal(t, "true", browser.evaluate(`document.addEventListener('ze-page-error-probe',function(){throw new Error('ze-page-error-collector-probe');},{once:true});document.dispatchEvent(new Event('ze-page-error-probe'));true`))
	require.Eventually(t, func() bool {
		for _, pageError := range identityPageErrors(t, browser) {
			if strings.Contains(string(pageError), "ze-page-error-collector-probe") {
				return true
			}
		}
		return false
	}, 5*time.Second, 20*time.Millisecond, "the native collector must observe an exception from a real event listener")
}

// identityPageErrors reads the browser's native exception buffer, which
// survives page reloads. A missing or malformed buffer is a failed oracle.
func identityPageErrors(t *testing.T, browser *downloadBrowser) []json.RawMessage {
	t.Helper()
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Errors []json.RawMessage `json:"errors"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(browser.command("--json", "errors")), &result))
	require.True(t, result.Success, "native page-error collection must succeed")
	require.NotNil(t, result.Data.Errors, "the native collector must return its explicit errors array")
	return result.Data.Errors
}

// newIdentityPageBrowser reuses the existing native Chromium command adapter
// and serves the production Workbench/form handlers with an explicit admin
// profile. The collector is cleared on about:blank before any product loads.
// Cleanup closes the browser before its event stream, server and editor store.
func newIdentityPageBrowser(t *testing.T) (*downloadBrowser, *EditorManager, string) {
	t.Helper()
	_, err := exec.LookPath("agent-browser")
	require.NoError(t, err, "identity PAGEERROR acceptance requires agent-browser and Chromium")
	mgr, schema, tree, renderer := setupCLITest(t)
	policies := authz.NewStore()
	policies.AddProfile(testAdminProfile())
	policies.AssignProfiles("alice", []string{"admin"})
	authorizer := authz.StoreAuthorizer{Store: policies}
	sessions := NewSessionStore(nil)
	session, err := sessions.createSession("alice", authz.AuthResult{Profiles: []string{"admin"}, Authorizer: authorizer})
	require.NoError(t, err)
	broker := NewEventBroker(2)
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", renderer.AssetHandler()))
	mux.HandleFunc("GET /show/", HandleWorkbench(renderer, schema, tree, mgr, false, WithAuthorizer(authorizer)))
	mux.HandleFunc("POST /config/form/", HandleConfigFormWithAuthorizer(mgr, schema, renderer, authorizer))
	mux.HandleFunc("GET /config/changes", HandleConfigChanges(mgr, renderer, authorizer))
	mux.Handle("GET /events", broker)
	server := httptest.NewServer(authMiddleware(sessions, &recordingAuthenticator{}, noopRenderer, mux))
	t.Cleanup(func() { broker.Close(); server.Close() })
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	browser := &downloadBrowser{t: t, ctx: ctx, session: fmt.Sprintf("ze-identity-%d-%x", os.Getpid(), downloadBrowserSequence.Add(1))}
	t.Cleanup(func() {
		cancel()
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCancel()
		browser.ctx = closeCtx
		browser.command("close")
	})
	browser.command("open", "about:blank")
	browser.command("errors", "--clear")
	browser.command("cookies", "set", "ze-session", session.Token, "--url", server.URL, "--http-only", "--same-site", "Strict")
	return browser, mgr, server.URL
}
