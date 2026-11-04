// Design: docs/architecture/testing/runner-architecture.md -- settled browser captures.
package webtesting

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// WBCapture receives the owned browser's CDP endpoint after the authored journey
// and render settling succeed. It MUST finish under ctx before returning; the
// runner MUST close the owned session afterwards, including capture failures.
type WBCapture func(ctx context.Context, endpoint string) error

// captureFactsJS reports observed values without making readiness decisions.
// Types and finite flags distinguish missing state from a measured zero; frame
// progress belongs to the labeled sample, not to a later command's timeout.
const captureFactsJS = `(function(phase){
  function observed(value){return {type:typeof value,finite:Number.isFinite(value),value:String(value)};}
  var frames=window.__zeCaptureFrames;
  return {
    phase:phase,
    installed:window.__zeWaitInstalled===true,
    installedType:typeof window.__zeWaitInstalled,
    inflight:observed(window.__zeInflight),
    lastChange:observed(window.__zeLastChange),
    quietMs:Number.isFinite(window.__zeLastChange)?performance.now()-window.__zeLastChange:null,
    readyState:document.readyState,
    fonts:document.fonts?document.fonts.status:null,
    visibility:document.visibilityState,
    rAFStarted:frames?frames.started:null,
    rAFFirstCompleted:frames?frames.firstCompleted:null,
    rAFCompleted:frames?frames.completed:null,
    finiteAnimations:document.getAnimations().filter(function(a){
      return (a.playState==='running'||a.pending)&&a.effect&&a.effect.getComputedTiming().endTime!==Infinity;
    }).map(function(a){
      var target=a.effect.target;
      return {playState:a.playState,pending:a.pending,endTime:String(a.effect.getComputedTiming().endTime),
        currentTime:String(a.currentTime),target:target?{tag:target.tagName,id:target.id,className:String(target.className)}:null};
    })
  };
})`

// The initial synchronous sample survives even if the first frame wait never
// returns. Both it and every later observation consume the existing retry budget.
const captureInitialFactsExpr = `(function(){
  window.__zeCaptureFrames={started:false,firstCompleted:false,completed:false};
  return ` + captureFactsJS + `('before-render-frames');
})()`

// captureReadyExpr retains the same two-frame wait and readiness predicate.
// Infinite activity indicators and persistent EventSource connections are not
// settling work. A false result carries facts; only boolean true permits capture.
const captureReadyExpr = `(async function(){
  var frames={started:false,firstCompleted:false,completed:false};
  window.__zeCaptureFrames=frames;
  var first=new Promise(requestAnimationFrame);
  frames.started=true;
  await first;
  frames.firstCompleted=true;
  await new Promise(requestAnimationFrame);
  frames.completed=true;
  var ready=window.__zeWaitInstalled===true&&Number.isFinite(window.__zeInflight)&&Number.isFinite(window.__zeLastChange)&&document.readyState==='complete'&&(!document.fonts||document.fonts.status==='loaded')&&!document.getAnimations().some(function(a){return (a.playState==='running'||a.pending)&&a.effect&&a.effect.getComputedTiming().endTime!==Infinity;})&&(` + inflightIdleExpr + `);
  return ready?true:` + captureFactsJS + `('after-render-frames');
})()`

// settleCapture retains the last completed sample independently of the last
// command error, so a deadline-killed eval cannot erase earlier observed facts.
func (b *Browser) settleCapture() error {
	first := true
	var lastFacts string
	var initialErr error
	err := b.retryCommand(func() error {
		if first {
			first = false
			lastFacts, initialErr = b.runAgentOutput("eval", captureInitialFactsExpr)
		}
		out, err := b.runAgentOutput("eval", captureReadyExpr)
		if err != nil {
			return fmt.Errorf("observe browser render before capture: %w", err)
		}
		if strings.TrimSpace(out) != "true" {
			lastFacts = strings.TrimSpace(out)
			return errors.New("browser capture is not settled: require installed request observation, then wait for rendering, fonts, finite animations and requests to finish; inspect the page if they do not finish")
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if lastFacts != "" {
		err = errors.Join(err, fmt.Errorf("last completed capture readiness observation: %s", lastFacts))
	}
	if initialErr != nil {
		err = errors.Join(err, fmt.Errorf("initial capture readiness observation failed: %w", initialErr))
	}
	return err
}

func (b *Browser) capture(capture WBCapture) error {
	if err := b.settleCapture(); err != nil {
		return err
	}
	endpoint, err := b.runAgentOutput("get", "cdp-url")
	if err != nil {
		return fmt.Errorf("read owned browser capture endpoint: %w", err)
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return errors.New("owned browser returned no capture endpoint; check agent-browser installation before recording the demo")
	}
	return capture(b.ctx, endpoint)
}
