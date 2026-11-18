// Design: docs/architecture/web-interface.md -- native command outcome adapters.
// Related: command_outcome_fixture.go -- the external SDK outcome producer.
// Related: command_outcome_surfaces_fixture.go -- shared row and fault checks.

package fixture

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/command-outcome-web", commandOutcomeWeb, StopsDaemon)
}

// This module reaches the real startup schema loader, then the normal related
// descriptor lookup. It adds no handler or alternate dispatcher. Literal pipe
// text belongs to the trusted authored descriptor; no form supplies a command.
func commandOutcomeWebSchemas() []sdk.SchemaDecl {
	var module strings.Builder
	module.WriteString(`module ze-answer-outcome-web-conf {
    namespace "urn:ze:test:answer-outcome-web:conf";
    prefix outcome;
    import ze-extensions { prefix ze; }
    description "External command outcome fixture metadata.";
    revision 2026-11-18 { description "Declare related outcome tools."; }
    container answer-outcome-web {
        presence "Enable the command outcome fixture";
        ze:help "External command outcome fixture.";
        description "Related tools dispatch commands declared by the external fixture.";
`)
	declared := make(map[string]bool)
	for _, outcome := range []string{"partial", "failed", "warning"} {
		for _, question := range commandOutcomeQuestions(outcome) {
			id := commandOutcomeWebToolID(question)
			if declared[id] {
				continue
			}
			declared[id] = true
			fmt.Fprintf(&module, "        ze:related 'id=%s; label=\"Outcome %s\"; command=\"%s | raw\"; placement=detail; presentation=modal; class=inspect';\n", id, id, question.command)
		}
	}
	module.WriteString("    }\n}\n")
	return []sdk.SchemaDecl{{
		Module: "ze-answer-outcome-web-conf", Namespace: "urn:ze:test:answer-outcome-web:conf", YANGText: module.String(),
	}}
}

// commandOutcomeWeb runs inside a second external SDK process after all
// plugins are ready. HTTP remains the only route by which it runs a command.
func commandOutcomeWeb(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("command-outcome-web requires a port, terminal/related/decode, and an outcome")
	}
	address, outcome, err := commandOutcomeSurfaceArgs([]string{args[0], args[2]})
	if err != nil {
		return err
	}
	surface := args[1]
	switch surface {
	case "terminal", "related", "decode":
	default:
		return fmt.Errorf("unknown command outcome web surface %q", surface)
	}
	return Observe(ctx, "answer-outcome-observer", &sdk.Registration{}, func(ctx context.Context, _ *sdk.Plugin) error {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return err
		}
		// The daemon creates a private self-signed certificate. Only this
		// runner-leased literal loopback URL is trusted by this test client;
		// authentication and the browser cookie/origin boundary stay enabled.
		transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // ephemeral fixture certificate on validated loopback
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Jar: jar, Timeout: WaitBudget(40, time.Second), CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		base := "https://" + address
		if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
			reply, err := commandOutcomeWebRequest(ctx, client, base, http.MethodGet, "/cli", nil, "")
			if err != nil {
				return false
			}
			return reply.status == http.StatusUnauthorized
		}) {
			return errors.New("authenticated web listener did not become ready")
		}
		question := commandOutcomeQuestions(outcome)[1]
		path, form := commandOutcomeWebInvocation(surface, question)
		if err := commandOutcomeWebRefused(ctx, client, base, path, form, base, http.StatusUnauthorized); err != nil {
			return fmt.Errorf("anonymous %s: %w", surface, err)
		}
		if err := commandOutcomeWebLogin(ctx, client, base, "web-outcome-allowed"); err != nil {
			return err
		}
		// Terminal and related-tool POSTs are mutation-wrapped routes even
		// when this particular declared command only observes state. The
		// read-only native decode page has no same-origin route wrapper.
		if surface != "decode" {
			if err := commandOutcomeWebRefused(ctx, client, base, path, form, "https://untrusted.example.invalid", http.StatusForbidden); err != nil {
				return fmt.Errorf("cross-origin %s: %w", surface, err)
			}
		}
		if surface == "decode" {
			if err := commandOutcomeWebDecode(ctx, client, base); err != nil {
				return err
			}
		} else {
			for _, question := range commandOutcomeQuestions(outcome) {
				if err := commandOutcomeWebCheck(ctx, client, base, surface, question); err != nil {
					return err
				}
			}
		}
		if err := commandOutcomeWebLogin(ctx, client, base, "web-outcome-denied"); err != nil {
			return err
		}
		reply, err := commandOutcomeWebRequest(ctx, client, base, http.MethodPost, path, form, base)
		if err != nil {
			return err
		}
		if reply.status != http.StatusOK {
			return fmt.Errorf("denied %s HTTP %d: %s", surface, reply.status, reply.body)
		}
		if !strings.Contains(string(reply.body), plugin.UnauthorizedMessage) {
			return fmt.Errorf("denied %s lost the actual authorization refusal: %s", surface, reply.body)
		}
		if err := commandOutcomeWebNoProgress(reply.body); err != nil {
			return fmt.Errorf("denied %s: %w", surface, err)
		}
		fmt.Fprintf(os.Stderr, "OK: web %s %s retained native outcomes and authenticated controls\n", surface, outcome)
		return nil
	})
}

type commandOutcomeWebReply struct {
	status  int
	body    []byte
	cookies []*http.Cookie
}

func commandOutcomeWebRequest(ctx context.Context, client *http.Client, base, method, path string, form url.Values, origin string) (commandOutcomeWebReply, error) {
	request, err := http.NewRequestWithContext(ctx, method, base+path, strings.NewReader(form.Encode()))
	if err != nil {
		return commandOutcomeWebReply{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response, err := client.Do(request)
	if err != nil {
		return commandOutcomeWebReply{}, err
	}
	defer response.Body.Close() //nolint:errcheck // the bounded complete body determines this assertion
	const bodyMax = 1 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, bodyMax+1))
	if err != nil {
		return commandOutcomeWebReply{}, err
	}
	if len(body) > bodyMax {
		return commandOutcomeWebReply{}, errors.New("web outcome response exceeds the fixture bound")
	}
	return commandOutcomeWebReply{status: response.StatusCode, body: body, cookies: response.Cookies()}, nil
}

func commandOutcomeWebLogin(ctx context.Context, client *http.Client, base, username string) error {
	reply, err := commandOutcomeWebRequest(ctx, client, base, http.MethodPost, "/login", url.Values{"username": {username}, "password": {"testpass"}}, base)
	if err != nil {
		return err
	}
	if reply.status != http.StatusOK {
		return fmt.Errorf("web login %s HTTP %d: %s", username, reply.status, reply.body)
	}
	for _, cookie := range reply.cookies {
		if cookie.Name != "ze-session" {
			continue
		}
		if cookie.Value == "" {
			return errors.New("web login returned an empty session token")
		}
		if !cookie.Secure {
			return errors.New("web session cookie lost Secure")
		}
		if !cookie.HttpOnly {
			return errors.New("web session cookie lost HttpOnly")
		}
		if cookie.SameSite != http.SameSiteStrictMode {
			return errors.New("web session cookie lost SameSite=Strict")
		}
		return nil
	}
	return errors.New("web login returned no session cookie")
}

func commandOutcomeWebInvocation(surface string, question commandOutcomeQuestion) (string, url.Values) {
	switch surface {
	case "terminal":
		return "/cli/terminal", url.Values{"command": {question.command + " | raw"}, "mode": {"operational"}, "path": {""}}
	case "related":
		return "/tools/related/run", url.Values{"tool_id": {commandOutcomeWebToolID(question)}, "context_path": {"answer-outcome-web"}}
	case "decode":
		return "/show/tools/bgp-decode/", url.Values{"hex": {"FF"}}
	default:
		panic("BUG: unvalidated web outcome fixture surface")
	}
}

func commandOutcomeWebToolID(question commandOutcomeQuestion) string {
	return strings.ReplaceAll(strings.TrimPrefix(question.command, "show answer-outcome "), " ", "-")
}

func commandOutcomeWebRefused(ctx context.Context, client *http.Client, base, path string, form url.Values, origin string, status int) error {
	reply, err := commandOutcomeWebRequest(ctx, client, base, http.MethodPost, path, form, origin)
	if err != nil {
		return err
	}
	if reply.status != status {
		return fmt.Errorf("HTTP %d, want %d: %s", reply.status, status, reply.body)
	}
	return commandOutcomeWebNoProgress(reply.body)
}

func commandOutcomeWebNoProgress(body []byte) error {
	text := strings.ToLower(string(body))
	for _, private := range []string{"ordinal", "fixture acknowledgement refused", "keepalive"} {
		if strings.Contains(text, private) {
			return fmt.Errorf("refused web request exposed %q", private)
		}
	}
	return nil
}

func commandOutcomeWebCheck(ctx context.Context, client *http.Client, base, surface string, question commandOutcomeQuestion) error {
	path, form := commandOutcomeWebInvocation(surface, question)
	reply, err := commandOutcomeWebRequest(ctx, client, base, http.MethodPost, path, form, base)
	if err != nil {
		return err
	}
	if reply.status != http.StatusOK {
		return fmt.Errorf("web %s %s HTTP %d: %s", surface, question.command, reply.status, reply.body)
	}
	var output string
	if surface == "terminal" {
		var terminal struct {
			Output   string `json:"output"`
			Complete bool   `json:"output-complete"`
		}
		if err := json.Unmarshal(reply.body, &terminal); err != nil {
			return err
		}
		wantComplete := false
		switch question.outcome {
		case "healthy", "warning":
			wantComplete = true
		case "partial", "failed":
		default:
			return fmt.Errorf("unknown web outcome %q", question.outcome)
		}
		if terminal.Complete != wantComplete {
			return fmt.Errorf("web terminal %s completion=%t, want %t", question.command, terminal.Complete, wantComplete)
		}
		output = terminal.Output
	} else {
		class := "tool-overlay-output"
		switch question.outcome {
		case "partial", "failed":
			class = "tool-overlay-error-message"
		case "healthy", "warning":
		default:
			return fmt.Errorf("unknown web outcome %q", question.outcome)
		}
		output, err = commandOutcomeWebText(reply.body, "pre", class)
		if err != nil {
			return err
		}
	}
	start := strings.IndexByte(output, '{')
	if start < 0 {
		return fmt.Errorf("web %s %s lost the completed document: %s", surface, question.command, output)
	}
	if err := commandOutcomeSurfaceDocument(json.RawMessage(output[start:]), question); err != nil {
		return fmt.Errorf("web %s %s: %w", surface, question.command, err)
	}
	indicator := strings.TrimSpace(output[:start])
	switch question.outcome {
	case "healthy":
		if indicator != "" {
			return fmt.Errorf("healthy web %s acquired a failure/warning indicator: %q", surface, indicator)
		}
	case "partial":
		if strings.Count(indicator, string(rpc.VerdictPartial)) != 1 {
			return fmt.Errorf("partial web %s lost its distinct verdict: %q", surface, indicator)
		}
	case "failed":
		if strings.Count(indicator, rpc.AnswerFailureUnstated) != 1 {
			return fmt.Errorf("failed web %s lost its original failure: %q", surface, indicator)
		}
	case "warning":
		// A warning terminator need not supply warning lines. The operator
		// must still see its warning verdict; no invented producer line can
		// satisfy this test in place of preserving that actual verdict.
		if !strings.Contains(strings.ToLower(indicator), "warning") {
			return fmt.Errorf("warning web %s appeared successful without its warning verdict: %q", surface, indicator)
		}
	default:
		return fmt.Errorf("unknown web outcome %q", question.outcome)
	}
	return nil
}

// The tokenizer consumes at least one byte per token. The complete response
// is already bounded above, so this traversal cannot follow unbounded markup.
func commandOutcomeWebText(body []byte, element, class string) (string, error) {
	tokens := html.NewTokenizer(strings.NewReader(string(body)))
	var text strings.Builder
	active := false
	for range len(body) + 1 {
		switch tokens.Next() {
		case html.StartTagToken:
			token := tokens.Token()
			if token.Data != element {
				continue
			}
			for _, attr := range token.Attr {
				if attr.Key != "class" {
					continue
				}
				for _, field := range strings.Fields(attr.Val) {
					if field == class {
						active = true
					}
				}
			}
		case html.EndTagToken:
			if active {
				if tokens.Token().Data == element {
					return text.String(), nil
				}
			}
		case html.TextToken:
			if active {
				text.Write(tokens.Text())
			}
		case html.ErrorToken:
			return "", fmt.Errorf("web response has no complete %s.%s result", element, class)
		case html.CommentToken, html.DoctypeToken, html.SelfClosingTagToken:
		}
	}
	return "", errors.New("web outcome markup token bound exceeded")
}

// The decode page can only call its registered native decode producer. A
// malformed hexadecimal BGP body is a natural producer refusal, not a fake
// plugin command. This is deliberately not claimed as record-partial proof.
func commandOutcomeWebDecode(ctx context.Context, client *http.Client, base string) error {
	const path = "/show/tools/bgp-decode/"
	healthy, err := commandOutcomeWebRequest(ctx, client, base, http.MethodPost, path, url.Values{"hex": {"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304"}}, base)
	if err != nil {
		return err
	}
	if healthy.status != http.StatusOK {
		return fmt.Errorf("native decode healthy HTTP %d: %s", healthy.status, healthy.body)
	}
	output, err := commandOutcomeWebText(healthy.body, "pre", "wb-tool-output")
	if err != nil {
		return err
	}
	if !strings.Contains(strings.ToLower(output), "keepalive") {
		return fmt.Errorf("native decode lost its healthy KEEPALIVE: %s", output)
	}
	if strings.Contains(string(healthy.body), "wb-tool-error") {
		return fmt.Errorf("native decode healthy result acquired an error: %s", healthy.body)
	}
	failed, err := commandOutcomeWebRequest(ctx, client, base, http.MethodPost, path, url.Values{"hex": {"FF"}}, base)
	if err != nil {
		return err
	}
	if failed.status != http.StatusOK {
		return fmt.Errorf("native decode refusal HTTP %d: %s", failed.status, failed.body)
	}
	failure, err := commandOutcomeWebText(failed.body, "div", "wb-tool-error")
	if err != nil {
		return err
	}
	if !strings.Contains(failure, "show bgp decode could not decode the payload:") {
		return fmt.Errorf("native decode lost its producer refusal: %s", failure)
	}
	if !strings.Contains(failure, "give one BGP message in hexadecimal") {
		return fmt.Errorf("native decode lost its recovery instruction: %s", failure)
	}
	return nil
}
