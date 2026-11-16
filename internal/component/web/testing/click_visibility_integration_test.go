// Design: docs/architecture/testing/runner-architecture.md -- visible pointer clicks.
// Related: runner.go -- clickID, clickStableExpr.
//go:build integration

package webtesting

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestBrowserClickIDScrollsPartiallyVisibleTarget proves a long form's button
// reaches the viewport before a real pointer click. The native selector click
// scrolls a wholly offscreen box, but a box that intersects the viewport can
// still have its center outside it. The fixture pins that boundary explicitly.
// MUTATION: omitting scrollintoview from clickID leaves the center outside the
// viewport and the strict native click fails without activating the button.
func TestBrowserClickIDScrollsPartiallyVisibleTarget(t *testing.T) {
	browser := newClickVisibilityBrowser(t, `<div style="height:230px"></div>
<button id="target" style="width:120px;height:80px">Target</button>
<div style="height:1000px"></div>`)
	clickVisibilityAssert(t, browser, `(()=>{const r=document.getElementById('target').getBoundingClientRect();return r.top>0&&r.top<innerHeight&&r.top+r.height/2>innerHeight&&scrollY===0})()`,
		"the target must intersect the viewport while its center remains outside")
	if err := browser.clickID("target"); err != nil {
		t.Fatalf("click partially visible target: %v", err)
	}
	clickVisibilityAssert(t, browser, `window.targetClicks===1&&window.untrustedClicks===0&&window.clickWasVisible===true&&scrollY>0`,
		"one trusted click must reach the scrolled target inside the viewport")
}

// TestBrowserClickIDKeepsClosedDisclosureHidden proves scrolling cannot open
// a hidden editing branch. The actual summary click is the positive control:
// only after that visible interaction may the target receive a trusted click.
// MUTATION: opening ancestor details or using a synthetic click makes the
// hidden attempt succeed or changes the disclosure/click counters.
func TestBrowserClickIDKeepsClosedDisclosureHidden(t *testing.T) {
	browser := newClickVisibilityBrowser(t, `<details id="branch"><summary id="summary">Edit</summary>
<button id="target">Target</button></details>`)
	clickVisibilityAssert(t, browser, `document.getElementById('branch').open===false&&window.targetClicks===0`,
		"the editing disclosure must start closed")
	restore := browser.bounded(3 * time.Second)
	err := browser.clickID("target")
	restore()
	if err == nil {
		t.Fatal("a target inside a closed disclosure was clicked")
	}
	clickVisibilityAssert(t, browser, `document.getElementById('branch').open===false&&window.targetClicks===0&&window.untrustedClicks===0`,
		"the refused click must leave the disclosure closed and target untouched")
	if err := browser.clickID("summary"); err != nil {
		t.Fatalf("open disclosure with its visible summary: %v", err)
	}
	if err := browser.clickID("target"); err != nil {
		t.Fatalf("click target after explicit disclosure opening: %v", err)
	}
	clickVisibilityAssert(t, browser, `document.getElementById('branch').open===true&&window.targetClicks===1&&window.untrustedClicks===0`,
		"the explicitly opened target must accept exactly one trusted click")
}

// TestBrowserClickIDKeepsCoveredTargetBlocked proves bringing a target into
// view does not bypass a covering element. The covering button is dismissed
// through its own real click before the original target can be activated.
// MUTATION: forced or synthetic target clicks bypass the cover and violate
// the refusal or the exact trusted-click counters.
func TestBrowserClickIDKeepsCoveredTargetBlocked(t *testing.T) {
	browser := newClickVisibilityBrowser(t, `<div style="height:230px"></div>
<button id="target" style="width:120px;height:80px">Target</button>
<div style="height:1000px"></div>
<button id="cover" style="position:fixed;inset:0;z-index:100" onclick="window.coverClicks++;this.remove()">Dismiss cover</button>`)
	clickVisibilityAssert(t, browser, `document.getElementById('cover')!==null&&window.coverClicks===0&&window.targetClicks===0`,
		"the blocking cover must be present before the attempted click")
	restore := browser.bounded(3 * time.Second)
	err := browser.clickID("target")
	restore()
	if err == nil {
		t.Fatal("a target behind a fixed cover was clicked")
	}
	if !strings.Contains(err.Error(), "covered by") {
		t.Fatalf("the native refusal must identify coverage: %v", err)
	}
	if !strings.Contains(err.Error(), "cover") {
		t.Fatalf("the native refusal must identify the cover: %v", err)
	}
	clickVisibilityAssert(t, browser, `document.getElementById('cover')!==null&&window.coverClicks===0&&window.targetClicks===0&&window.untrustedClicks===0`,
		"the refused target click must neither activate the target nor dismiss its cover")
	if err := browser.clickID("cover"); err != nil {
		t.Fatalf("dismiss covering button through a real pointer click: %v", err)
	}
	if err := browser.clickID("target"); err != nil {
		t.Fatalf("click target after explicit cover dismissal: %v", err)
	}
	clickVisibilityAssert(t, browser, `window.coverClicks===1&&window.targetClicks===1&&window.untrustedClicks===0&&window.clickWasVisible===true`,
		"cover dismissal and target activation must each occur exactly once")
}

// TestBrowserClickIDKeepsFiniteAnimationGuard proves the scroll preparation
// retains the existing finite-animation refusal. A controlled long animation
// is finished explicitly only after the moving-target attempt is refused.
// MUTATION: removing the clickStateMoving branch clicks the moving target,
// so the refusal and untouched-target assertion fail.
func TestBrowserClickIDKeepsFiniteAnimationGuard(t *testing.T) {
	browser := newClickVisibilityBrowser(t, `<button id="target">Target</button>`)
	if _, err := browser.runAgentOutput("eval", `window.targetMotion=document.getElementById('target').animate([{transform:'translateX(0)'},{transform:'translateX(100px)'}],{duration:60000,fill:'forwards'});true`); err != nil {
		t.Fatalf("start controlled finite animation: %v", err)
	}
	clickVisibilityAssert(t, browser, `window.targetMotion.playState==='running'&&window.targetClicks===0`,
		"the finite target animation must be running before the attempted click")
	restore := browser.bounded(3 * time.Second)
	err := browser.clickID("target")
	restore()
	if err == nil {
		t.Fatal("a target with a running finite animation was clicked")
	}
	if !strings.Contains(err.Error(), errClickTargetMoving.Error()) {
		t.Fatalf("the moving-target refusal was lost: %v", err)
	}
	clickVisibilityAssert(t, browser, `window.targetMotion.playState==='running'&&window.targetClicks===0&&window.untrustedClicks===0`,
		"the refused click must leave the moving target untouched")
	if _, err := browser.runAgentOutput("eval", `window.targetMotion.finish();true`); err != nil {
		t.Fatalf("finish controlled finite animation: %v", err)
	}
	if err := browser.clickID("target"); err != nil {
		t.Fatalf("click settled target: %v", err)
	}
	clickVisibilityAssert(t, browser, `window.targetMotion.playState==='finished'&&window.targetClicks===1&&window.untrustedClicks===0`,
		"the settled target must accept exactly one trusted click")
}

// newClickVisibilityBrowser owns a real browser and server for one click
// behavior. Cleanup closes only that browser before its server and context.
func newClickVisibilityBrowser(t *testing.T, body string) *Browser {
	t.Helper()
	if _, err := exec.LookPath(agentBrowserBin); err != nil {
		t.Fatalf("visible pointer click proof requires agent-browser: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)
	page := `<!doctype html><html><head><script>
window.targetClicks=0;window.untrustedClicks=0;window.coverClicks=0;
document.addEventListener('click',function(e){if(e.target.id!=='target')return;
if(!e.isTrusted){window.untrustedClicks++;return;}window.targetClicks++;
const r=e.target.getBoundingClientRect();const x=r.x+r.width/2,y=r.y+r.height/2;
window.clickWasVisible=x>=0&&y>=0&&x<innerWidth&&y<innerHeight;});
</script></head><body style="margin:0">` + body + `</body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Cache-Control", "no-store")
		if _, err := w.Write([]byte(page)); err != nil {
			t.Errorf("serve visible click page: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	browser := newBrowser(server.URL)
	browser.ctx = ctx
	t.Cleanup(func() {
		if err := browser.Close(); err != nil {
			t.Errorf("close visible click browser: %v", err)
		}
	})
	if err := browser.setViewport(320, 240); err != nil {
		t.Fatal(err)
	}
	if err := browser.Open("/"); err != nil {
		t.Fatal(err)
	}
	return browser
}

func clickVisibilityAssert(t *testing.T, browser *Browser, expression, why string) {
	t.Helper()
	value, err := browser.runAgentOutput("eval", expression)
	if err != nil {
		t.Fatalf("%s: %v", why, err)
	}
	if strings.TrimSpace(value) != browserTrue {
		t.Fatalf("%s: got %q", why, value)
	}
}
