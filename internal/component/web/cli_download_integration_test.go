// Design: docs/architecture/web-interface.md -- caller-side result downloads.
//go:build integration

package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
)

// TestBrowserCLIDownloadFiniteFormattedResult follows the actual authenticated
// page, terminal handler and embedded JavaScript. The browser observer records
// the Blob handed to a real download anchor; it does not implement exporting.
// MUTATION: exporting the visible tail rather than the complete result loses
// the first sentinel and changes this independently calculated byte digest.
func TestBrowserCLIDownloadFiniteFormattedResult(t *testing.T) {
	payload := "FIRST-é界😀\n" + strings.Repeat("é界😀\n", 90_000) + "LAST-é界😀"
	want, err := json.Marshal(map[string]string{"answer": payload})
	require.NoError(t, err)
	require.Greater(t, len(want), 512*1024, "the proof must exceed visible stream scrollback")
	require.Less(t, len(want), 4*1024*1024)
	var calls atomic.Int32
	browser := newDownloadBrowser(t, func(_ context.Context, caller plugin.CallerIdentity, input string) (*plugin.Response, error) {
		calls.Add(1)
		require.Equal(t, "alice", caller.Username)
		require.Equal(t, "show download probe", input)
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"answer": payload}), nil
	})
	browser.submit("run show download probe | json compact")
	browser.wait(`document.getElementById('cli-output').textContent.includes('LAST-é界😀')`, "complete formatted result")
	browser.download()
	proof := browser.proof()
	digest := sha256.Sum256(want)
	require.Equal(t, len(want), proof.Size, "download must contain every UTF-8 byte")
	require.Equal(t, hex.EncodeToString(digest[:]), proof.SHA256, "download bytes differ from the requested compact JSON")
	require.NotEmpty(t, proof.Filename)
	require.Equal(t, int32(1), calls.Load(), "downloading must not dispatch the command again")
}

// TestBrowserCLIDownloadRefusalInvalidatesPreviousResult prevents a denied
// request from leaving an earlier successful result offered as its answer.
func TestBrowserCLIDownloadRefusalInvalidatesPreviousResult(t *testing.T) {
	var calls atomic.Int32
	browser := newDownloadBrowser(t, func(_ context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
		calls.Add(1)
		if input == "show download denied" {
			return &plugin.Response{Status: plugin.StatusError, Error: plugin.UnauthorizedMessage}, nil
		}
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"answer": "allowed-read-sentinel"}), nil
	})
	browser.submit("run show download allowed | json compact")
	browser.wait(`document.getElementById('cli-output').textContent.includes('allowed-read-sentinel')`, "allowed read")
	browser.download()
	require.Equal(t, 1, browser.proof().Clicks)
	browser.submit("run show download denied")
	browser.wait(`document.getElementById('cli-output').textContent.includes(`+strconv.Quote(plugin.UnauthorizedMessage)+`)`, "real dispatcher refusal")
	browser.wait(`document.querySelector('[data-cli-local-operator="save"]').disabled`, "refused result cannot be downloaded")
	require.Equal(t, 1, browser.proof().Clicks)
	require.Equal(t, int32(2), calls.Load())
}

// TestBrowserCLIDownloadReplacementReleasesObjectURL checks the caller resource
// lifetime at the native API, independently of how the export state is stored.
func TestBrowserCLIDownloadReplacementReleasesObjectURL(t *testing.T) {
	browser := newDownloadBrowser(t, func(_ context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"answer": input}), nil
	})
	browser.submit("run show download first | json compact")
	browser.wait(`document.getElementById('cli-output').textContent.includes('show download first')`, "first result")
	browser.download()
	browser.submit("run show download second | json compact")
	browser.wait(`document.getElementById('cli-output').textContent.includes('show download second')`, "replacement result")
	browser.wait(`window.__downloadProof.revoked.has(window.__downloadProof.clicks[0].url)`, "previous object URL release")
	browser.download()
	proof := browser.proof()
	want := sha256.Sum256([]byte(`{"answer":"show download second"}`))
	require.Equal(t, hex.EncodeToString(want[:]), proof.SHA256)
	require.Equal(t, 2, proof.Clicks)
	browser.evaluate(`window.dispatchEvent(new PageTransitionEvent('pagehide'));true`)
	browser.wait(`window.__downloadProof.clicks.every(function(c){return window.__downloadProof.revoked.has(c.url);})`, "page exit object URL release")
}

// TestBrowserCLIDownloadUTF8ByteLimitRefusesWholeResult prevents a character
// count from admitting an oversized Unicode export, or a silent partial file.
func TestBrowserCLIDownloadUTF8ByteLimitRefusesWholeResult(t *testing.T) {
	payload := strings.Repeat("界", 1_400_000)
	browser := newDownloadBrowser(t, func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"answer": payload}), nil
	})
	browser.submit("run show download oversized | json compact")
	browser.wait(`document.getElementById('cli-output').textContent.includes('界界界')`, "oversized result display")
	browser.wait(`!!document.querySelector('[data-cli-local-operator="save"]')&&document.querySelector('[data-cli-local-operator="save"]').disabled`, "oversized download refusal")
	browser.wait(`document.getElementById('cli-download-status').textContent.includes('4 MiB')`, "explicit byte-limit explanation")
	require.Equal(t, "true", browser.evaluate(`window.__downloadProof.clicks.length===0&&window.__downloadProof.blobs.size===0`))
}

// TestBrowserCLIDownloadLiteralDiagnosticText proves that rendered words are
// data: only the producer outcome decides whether the result is downloadable.
func TestBrowserCLIDownloadLiteralDiagnosticText(t *testing.T) {
	const text = "error: this document describes a previous event\né界😀\n"
	wire, err := json.Marshal(command.Blob(text))
	require.NoError(t, err)
	browser := newDownloadBrowser(t, func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(wire)), nil
	})
	browser.submit("run show download literal | text")
	browser.wait(`document.getElementById('cli-output').textContent.startsWith('error: this document')`, "successful diagnostic text")
	browser.download()
	digest := sha256.Sum256([]byte(text))
	require.Equal(t, hex.EncodeToString(digest[:]), browser.proof().SHA256)
}

// TestBrowserCLIDownloadPendingReplacementCancelsWithoutOldArtifact observes
// the held request directly, so no delay guesses whether cancellation started.
func TestBrowserCLIDownloadPendingReplacementCancelsWithoutOldArtifact(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	browser := newDownloadBrowser(t, func(ctx context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
		if input == "show download held" {
			close(started)
			<-ctx.Done()
			close(stopped)
			return nil, ctx.Err()
		}
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"answer": "initial-result"}), nil
	})
	browser.submit("run show download initial | json compact")
	browser.wait(`document.getElementById('cli-output').textContent.includes('initial-result')`, "initial result")
	browser.download()
	browser.submit("run show download held")
	select {
	case <-started:
	case <-browser.ctx.Done():
		t.Fatal("replacement request never reached its held producer")
	}
	browser.wait(`document.querySelector('[data-cli-local-operator="save"]').disabled&&window.__downloadProof.revoked.has(window.__downloadProof.clicks[0].url)`, "old artifact invalidated while replacement is pending")
	browser.evaluate(`window.dispatchEvent(new PageTransitionEvent('pagehide'));true`)
	select {
	case <-stopped:
	case <-browser.ctx.Done():
		t.Fatal("page exit did not cancel the actual pending request")
	}
	require.Equal(t, 1, browser.proof().Clicks, "cancellation must not offer another artifact")
}

// TestBrowserCLIDownloadRawCompleteBytes requires the authoritative framed
// terminal outcome and every original producer byte, including split UTF-8,
// invalid UTF-8 and bytes removed from the bounded visible tail.
func TestBrowserCLIDownloadRawCompleteBytes(t *testing.T) {
	want := bytes.Repeat([]byte("é界😀\xff\x00"), 4*1024*1024/11+1)[:4*1024*1024]
	copy(want[9:], "RAW-FIRST")
	copy(want[len(want)-len("RAW-LAST"):], "RAW-LAST")
	var calls atomic.Int32
	browser := newDownloadBrowser(t, nil, downloadStreamFixture{raw: func(_ context.Context, caller plugin.CallerIdentity, input string, output io.Writer) (bool, error) {
		calls.Add(1)
		require.Equal(t, "alice", caller.Username)
		require.Equal(t, "monitor download bytes", input)
		offset := 0
		for _, size := range []int{1, 2, 3, len(want) - 6} {
			_, err := output.Write(want[offset : offset+size])
			if err != nil {
				return true, err
			}
			offset += size
		}
		return true, nil
	}})
	browser.submit("run monitor download bytes")
	browser.wait(`document.getElementById('cli-output').textContent.endsWith('RAW-LAST')&&document.getElementById('cli-feedback').textContent==='Finished'`, "actual complete raw producer result")
	require.Equal(t, "true", browser.evaluate(`!document.querySelector('[data-cli-local-operator="save"]').disabled`), "authoritatively completed raw output must offer Download")
	require.Equal(t, "true", browser.evaluate(`document.getElementById('cli-output').textContent.startsWith('[Older output removed]')&&!document.getElementById('cli-output').textContent.includes('RAW-FIRST')`))
	browser.download()
	proof := browser.proof()
	digest := sha256.Sum256(want)
	require.Equal(t, len(want), proof.Size)
	require.Equal(t, hex.EncodeToString(digest[:]), proof.SHA256, "the Blob must preserve original binary bytes, independently of display decoding")
	require.Equal(t, int32(1), calls.Load())
}

// TestBrowserCLIDownloadRawProducerFailure covers a terminal failure and an
// ignored budget refusal. Neither may turn partial data into a complete file.
func TestBrowserCLIDownloadRawProducerFailure(t *testing.T) {
	for _, limit := range []bool{false, true} {
		t.Run(strconv.FormatBool(limit), func(t *testing.T) {
			browser := newDownloadBrowser(t, nil, downloadStreamFixture{raw: func(_ context.Context, _ plugin.CallerIdentity, _ string, output io.Writer) (bool, error) {
				if limit {
					if _, err := output.Write(bytes.Repeat([]byte("x"), 4*1024*1024)); err != nil {
						return true, err
					}
					_, err := output.Write([]byte("excess"))
					require.Error(t, err)
					return true, nil
				}
				if _, err := io.WriteString(output, "partial-raw-answer"); err != nil {
					return true, err
				}
				return true, errors.New("retained failure reason")
			}})
			browser.submit("run monitor download failure")
			browser.wait(`document.getElementById('cli-feedback').classList.contains('cli-feedback--error')`, "authoritative raw terminal error")
			require.Equal(t, "true", browser.evaluate(`document.querySelector('[data-cli-local-operator="save"]').disabled&&window.__downloadProof.blobs.size===0&&window.__downloadProof.clicks.length===0`))
			if limit {
				require.Equal(t, "true", browser.evaluate(`document.getElementById('cli-feedback').textContent.includes('4 MiB')`))
			}
		})
	}
}

// TestBrowserCLIDownloadRawStopInvalidatesPartialResult rendezvous with the
// actual streaming producer before Stop, then require its request to cancel.
func TestBrowserCLIDownloadRawStopInvalidatesPartialResult(t *testing.T) {
	stopped := make(chan struct{})
	browser := newDownloadBrowser(t, nil, downloadStreamFixture{raw: func(ctx context.Context, _ plugin.CallerIdentity, _ string, output io.Writer) (bool, error) {
		defer close(stopped)
		if _, err := io.WriteString(output, "held-raw-answer"); err != nil {
			return true, err
		}
		<-ctx.Done()
		return true, nil
	}})
	browser.submit("run monitor download held")
	browser.wait(`document.getElementById('cli-output').textContent.includes('held-raw-answer')`, "held raw bytes reach the actual browser")
	require.Equal(t, "true", browser.evaluate(`document.querySelector('[data-cli-local-operator="save"]').disabled`))
	browser.command("click", "#cli-stop")
	select {
	case <-stopped:
	case <-browser.ctx.Done():
		t.Fatal("Stop did not cancel the actual raw producer")
	}
	browser.wait(`document.getElementById('cli-feedback').textContent==='Stopped'`, "visible cancellation")
	require.Equal(t, "true", browser.evaluate(`document.querySelector('[data-cli-local-operator="save"]').disabled&&window.__downloadProof.blobs.size===0`))
}

// TestBrowserCLIDownloadRawInvalidFrames never accepts EOF without a terminal,
// corrupt encoding, excessive frames or producer data following completion.
func TestBrowserCLIDownloadRawInvalidFrames(t *testing.T) {
	chunk := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("a"), 32*1024))
	overBudget := strings.Repeat("{\"data\":\""+chunk+"\",\"done\":false}\n", 129) + "{\"done\":true}\n"
	for name, tc := range map[string]struct{ body, problem string }{
		"missing-terminal": {"{\"data\":\"YW5zd2Vy\",\"done\":false}\n", "before its final frame"},
		"bad-base64":       {"{\"data\":\"!!!\",\"done\":false}\n{\"done\":true}\n", "invalid encoded output"},
		"after-terminal":   {"{\"done\":true}\n{\"data\":\"YW5zd2Vy\",\"done\":false}\n", "after its final frame"},
		"oversized-frame":  {"{\"data\":\"" + strings.Repeat("YQ==", 20_000) + "\",\"done\":false}\n", "larger than 64 KiB"},
		"over-budget":      {overBudget, "4 MiB limit"},
	} {
		t.Run(name, func(t *testing.T) {
			browser := newDownloadBrowser(t, nil, downloadStreamFixture{response: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", cliOutputMediaType)
				if _, err := io.WriteString(w, tc.body); err != nil {
					require.ErrorIs(t, r.Context().Err(), context.Canceled, "malformed wire write failed before actual request cancellation: %v", err)
				}
			})})
			browser.submit("run monitor download malformed")
			browser.wait(`document.getElementById('cli-feedback').classList.contains('cli-feedback--error')`, "malformed stream refusal")
			require.Equal(t, "true", browser.evaluate(`document.getElementById('cli-feedback').textContent.includes(`+strconv.Quote(tc.problem)+`)`), "the requested malformed wire must cause its specific refusal")
			require.Equal(t, "true", browser.evaluate(`document.querySelector('[data-cli-local-operator="save"]').disabled&&window.__downloadProof.blobs.size===0`))
		})
	}
}

// TestBrowserCLIDownloadRichCurrentFrameSnapshot proves a real registered view
// exports only the current frame with an explicit snapshot label.
func TestBrowserCLIDownloadRichCurrentFrameSnapshot(t *testing.T) {
	views := NewCLIViewTransport(cliViewTestAdmission, cliViewTestBinding)
	browser := newDownloadBrowser(t, nil, downloadStreamFixture{views: views})
	browser.submit("run monitor bgp")
	browser.wait(`document.getElementById('cli-output').textContent.includes('10.0.0.10')`, "registered rich monitoring frame")
	require.Equal(t, "true", browser.evaluate(`document.getElementById('cli-download-status').textContent.toLowerCase().includes('snapshot')&&!document.getElementById('cli-download-status').textContent.includes('Complete result')`))
	browser.download()
	var frame struct {
		Text string `json:"text"`
	}
	// The live dashboard can refresh between separate browser commands. Compare
	// the Blob with the frame observed synchronously at native anchor activation.
	require.NoError(t, json.Unmarshal([]byte(browser.evaluate(`({text:window.__downloadProof.clicks[window.__downloadProof.clicks.length-1].text})`)), &frame))
	require.Contains(t, frame.Text, "10.0.0.10")
	digest := sha256.Sum256([]byte(frame.Text))
	require.Equal(t, hex.EncodeToString(digest[:]), browser.proof().SHA256)
	require.Equal(t, "ze-monitor-snapshot.txt", browser.proof().Filename)
	browser.command("click", "#cli-view-stop")
	browser.wait(`document.getElementById('cli-feedback').textContent==='Stopped'`, "real rich Stop completion")
	browser.evaluate(`window.dispatchEvent(new PageTransitionEvent('pagehide'));true`)
	browser.wait(`window.__downloadProof.clicks.every(function(c){return window.__downloadProof.revoked.has(c.url);})`, "snapshot URL release")
}

type downloadBrowser struct {
	t       *testing.T
	ctx     context.Context
	session string
}

// A process-local sequence keeps repeated fixtures distinct without putting
// unbounded test names into agent-browser's length-limited UNIX socket path.
var downloadBrowserSequence atomic.Uint64

type downloadStreamFixture struct {
	raw      plugin.CommandStreamDispatcher
	views    *CLIViewTransport
	response http.Handler
}

func newDownloadBrowser(t *testing.T, dispatch CommandDispatcher, stream ...downloadStreamFixture) *downloadBrowser {
	t.Helper()
	_, err := exec.LookPath("agent-browser")
	require.NoError(t, err, "real result-download acceptance requires agent-browser")
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	store := NewSessionStore(nil)
	session, err := store.createSession("alice", authz.AuthResult{Authorizer: fakeAuthorizer{allowEdit: true}})
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", renderer.AssetHandler()))
	mux.HandleFunc("GET /cli", HandleCLIPageHTTP(renderer, false, mgr))
	terminal := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, dispatch, fakeAuthorizer{allowEdit: true}, nil)
	streamHandler := HandleCLITerminalStream(mgr, terminal, nil)
	if len(stream) > 0 {
		streamHandler = HandleCLITerminalStream(mgr, terminal, stream[0].raw)
		if stream[0].views != nil {
			streamHandler = stream[0].views.Handler(mgr, terminal, stream[0].raw)
			mux.HandleFunc("POST /cli/view/action", stream[0].views.Action)
		}
		if stream[0].response != nil {
			streamHandler = func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.FormValue("command"), "monitor download") {
					stream[0].response.ServeHTTP(w, r)
					return
				}
				terminal(w, r)
			}
		}
	}
	mux.HandleFunc("POST /cli/stream", streamHandler)
	// The generic layout refreshes this optional strip on initialization.
	mux.HandleFunc("GET /config/changes", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	server := httptest.NewServer(authMiddleware(store, &recordingAuthenticator{}, noopRenderer, mux))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	browser := &downloadBrowser{t: t, ctx: ctx, session: fmt.Sprintf("ze-download-%d-%x", os.Getpid(), downloadBrowserSequence.Add(1))}
	t.Cleanup(func() {
		cancel()
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCancel()
		browser.ctx = closeCtx
		browser.command("close")
	})
	headers, err := json.Marshal(map[string]string{"Cookie": "ze-session=" + session.Token})
	require.NoError(t, err)
	browser.command("--ignore-https-errors", "set", "headers", string(headers))
	browser.command("open", server.URL+"/cli")
	browser.wait(`!!document.getElementById('terminal-input')&&document.getElementById('cli-output').textContent.length>0`, "real CLI initialization")
	browser.evaluate(downloadObserverJS)
	return browser
}

func (b *downloadBrowser) command(args ...string) string {
	b.t.Helper()
	cmd := exec.CommandContext(b.ctx, "agent-browser", args...) //nolint:gosec // arguments are test-owned browser actions
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = append(env.Without(os.Environ(), "agent.browser.session", "TMPDIR"), "AGENT_BROWSER_SESSION="+b.session)
	output, err := cmd.CombinedOutput()
	require.NoError(b.t, err, "native browser action failed: %s", output)
	return strings.TrimSpace(string(output))
}

func (b *downloadBrowser) evaluate(script string) string {
	b.t.Helper()
	return b.command("eval", script)
}

func (b *downloadBrowser) wait(predicate, description string) {
	b.t.Helper()
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		if b.evaluate(predicate) == "true" {
			return
		}
		select {
		case <-b.ctx.Done():
			b.t.Fatalf("wait for %s: %v", description, context.Cause(b.ctx))
		case <-ticker.C:
		}
	}
}

func (b *downloadBrowser) submit(line string) {
	b.t.Helper()
	b.command("fill", "#terminal-input", line)
	b.command("press", "Enter")
}

func (b *downloadBrowser) download() {
	b.t.Helper()
	require.Equal(b.t, "true", b.evaluate(`!!document.querySelector('[data-cli-local-operator="save"]')`), "the actual CLI page must offer the catalog-owned caller download control")
	b.wait(`!document.querySelector('[data-cli-local-operator="save"]').disabled`, "downloadable result")
	b.command("click", `[data-cli-local-operator="save"]`)
	b.wait(`window.__downloadProof.clicks.length>0`, "native download anchor activation")
}

type downloadProof struct {
	Size     int    `json:"size"`
	SHA256   string `json:"sha256"`
	Filename string `json:"filename"`
	Clicks   int    `json:"clicks"`
}

func (b *downloadBrowser) proof() downloadProof {
	b.t.Helper()
	var proof downloadProof
	require.NoError(b.t, json.Unmarshal([]byte(b.evaluate(downloadBytesJS)), &proof))
	return proof
}

// Observe only standard browser APIs at the artifact boundary. Preventing the
// anchor's default avoids leaving a filesystem download; its actual Blob,
// filename and native activation are still required and inspected below. The
// native method also observes an anchor that has not been attached to the DOM.
const downloadObserverJS = `(function(){
var proof={blobs:new Map(),clicks:[],revoked:new Set()};window.__downloadProof=proof;
var create=URL.createObjectURL.bind(URL),revoke=URL.revokeObjectURL.bind(URL);
URL.createObjectURL=function(blob){var url=create(blob);proof.blobs.set(url,blob);return url;};
URL.revokeObjectURL=function(url){proof.revoked.add(url);return revoke(url);};
var click=HTMLAnchorElement.prototype.click;
HTMLAnchorElement.prototype.click=function(){
if(this.hasAttribute('download')&&proof.blobs.has(this.href)){
proof.clicks.push({url:this.href,filename:this.download,text:document.getElementById('cli-output').textContent});
this.addEventListener('click',function(event){event.preventDefault();},{once:true});}
return click.call(this);};
return true;})()`

const downloadBytesJS = `(async function(){var p=window.__downloadProof,c=p.clicks[p.clicks.length-1];
if(!c)throw new Error('no native download anchor was activated');var bytes=await p.blobs.get(c.url).arrayBuffer();
var hash=await crypto.subtle.digest('SHA-256',bytes);
return {size:bytes.byteLength,sha256:Array.from(new Uint8Array(hash)).map(function(x){return x.toString(16).padStart(2,'0');}).join(''),filename:c.filename,clicks:p.clicks.length};})()`
