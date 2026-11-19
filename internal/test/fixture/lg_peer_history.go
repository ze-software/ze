// Design: docs/architecture/web-interface.md -- session history on the peers page and its live stream, over a real session
// Related: lg_covering_lookup.go -- lgCoveringRequest, the looking glass reader this file shares
// Related: plugin_fixture_09.go -- the registration, observe09 and dispatch09

package fixture

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// lgPeerHistoryAddress is the one configured peer, the scripted test peer.
	lgPeerHistoryAddress = "127.0.0.1"
	// lgPeerHistoryAnnounce is the route whose UPDATE the test peer answers
	// with its second NOTIFICATION. The scenario sends it only once the page
	// was read and the stream is open, so the second drop happens when the
	// scenario decides and never before.
	lgPeerHistoryAnnounce = "update text nhop 1.1.1.1 nlri add 1.1.0.0/16"
	// The wait for the session to come back after the first drop: the peer
	// reconnects on a five-second timer, and 120 reads a quarter of a second
	// apart are thirty seconds.
	lgPeerHistoryAttempts = 120
	lgPeerHistoryDelay    = 250 * time.Millisecond
	// The stream sends one message every five seconds
	// (internal/component/lg/handler_ui.go, handleUIEvents), so eight messages
	// are forty seconds, inside lgPeerHistoryStreamBudget.
	lgPeerHistoryStreamMessages = 8
	lgPeerHistoryStreamBudget   = 50 * time.Second
	// One stream message is one peer row here. A megabyte bounds a line of
	// it, so a stream that never ends a line is an error, never a full memory.
	lgPeerHistoryLineBytesMax = 1 << 20
)

// The two history cells of a peers page row. The class is how a reader, this
// one included, tells the drop count from the other numbers of the row.
var (
	lgPeerHistoryStateChangedCell = regexp.MustCompile(`<td class="state-changed">([^<]*)</td>`)
	lgPeerHistoryDroppedCell      = regexp.MustCompile(`<td class="connections-dropped">([^<]*)</td>`)
)

// lgPeerHistoryEngine is the history `show bgp` answers for the peer: the
// values the looking glass is expected to show, read from the producer the
// looking glass reads.
type lgPeerHistoryEngine struct {
	stateChanged string
	lastError    string
	dropped      int
	eorSent      int
}

// lgPeerHistory lets the test peer close the session with a NOTIFICATION,
// waits for the session to come back, and checks that the peers page shows one
// drop, the time of the last change and the last error. It then keeps the live
// stream open, makes the peer close the session a second time, and checks that
// a stream message carries the row with two drops.
func lgPeerHistory(ctx context.Context, _ []string) error {
	return observe09(ctx, "lg-peer-history", func(ctx context.Context, p *sdk.Plugin) error {
		if err := p12WaitPeersEOR(ctx, p, "peer1"); err != nil {
			return err
		}
		port := os.Getenv("ZE_LG_PORT")
		if port == "" {
			return errors.New("peer history: ZE_LG_PORT not set in plugin environment")
		}
		base := "http://127.0.0.1:" + port

		// The peer answers the first End-of-RIB with a NOTIFICATION. A second
		// End-of-RIB on the wire says the session came back, and it stays up
		// until this scenario announces a route, so the engine's answer and
		// the page are read over one unchanging state.
		once, err := lgPeerHistoryWait(ctx, p, 1, 2)
		if err != nil {
			return err
		}
		if once.lastError == "" {
			return fmt.Errorf("peer history: show bgp carries no last-error for %s after its NOTIFICATION", lgPeerHistoryAddress)
		}
		if once.stateChanged == "" {
			return fmt.Errorf("peer history: show bgp carries no state-changed for %s after its session came back", lgPeerHistoryAddress)
		}
		body, err := lgCoveringRequest(ctx, http.MethodGet, base+"/lg/peers", "")
		if err != nil {
			return err
		}
		row, err := lgPeerHistoryCheckRow("GET /lg/peers", body, once)
		if err != nil {
			return err
		}
		if err := lgPeerHistoryCheckLastError(ctx, base, row, once.lastError); err != nil {
			return err
		}
		if err := lgPeerHistoryWatchSecondDrop(ctx, p, base); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "OK: the peers page showed one drop, its time %s and the last error %q, and the live stream carried the second drop\n",
			once.stateChanged, once.lastError)
		return nil
	})
}

// lgPeerHistoryRead answers the history `show bgp` carries for the peer.
func lgPeerHistoryRead(ctx context.Context, p *sdk.Plugin) (lgPeerHistoryEngine, error) {
	r := dispatch09(ctx, p, "show bgp")
	if !done09(r) {
		return lgPeerHistoryEngine{}, fmt.Errorf("show bgp: status=%s data=%s", r.status, r.text)
	}
	rows, ok := map09(r.data)["peers"].([]any)
	if !ok {
		return lgPeerHistoryEngine{}, fmt.Errorf("show bgp: answer holds no peers list: %s", r.text)
	}
	for _, raw := range rows {
		row := map09(raw)
		if row["address"] != lgPeerHistoryAddress {
			continue
		}
		dropped, ok := p12Number(row["connections-dropped"])
		if !ok {
			return lgPeerHistoryEngine{}, fmt.Errorf("show bgp: connections-dropped of %s is not a number: %v", lgPeerHistoryAddress, row["connections-dropped"])
		}
		eorSent, ok := p12Number(row["eor-sent"])
		if !ok {
			return lgPeerHistoryEngine{}, fmt.Errorf("show bgp: eor-sent of %s is not a number: %v", lgPeerHistoryAddress, row["eor-sent"])
		}
		stateChanged, _ := row["state-changed"].(string)
		lastError, _ := row["last-error"].(string)
		return lgPeerHistoryEngine{stateChanged: stateChanged, lastError: lastError, dropped: dropped, eorSent: eorSent}, nil
	}
	return lgPeerHistoryEngine{}, fmt.Errorf("show bgp: no row for %s: %s", lgPeerHistoryAddress, r.text)
}

// lgPeerHistoryWait reads the engine until the peer has dropped exactly
// dropped times and at least eorSent End-of-RIB markers reached the wire.
func lgPeerHistoryWait(ctx context.Context, p *sdk.Plugin, dropped, eorSent int) (lgPeerHistoryEngine, error) {
	var engine lgPeerHistoryEngine
	var readErr error
	if !Poll(ctx, lgPeerHistoryAttempts, lgPeerHistoryDelay, func() bool {
		engine, readErr = lgPeerHistoryRead(ctx, p)
		if readErr != nil {
			return false
		}
		if engine.dropped != dropped {
			return false
		}
		return engine.eorSent >= eorSent
	}) {
		if readErr != nil {
			return lgPeerHistoryEngine{}, fmt.Errorf("peer history: %s never reached %d drops and %d End-of-RIB sent: last read %+v: %w",
				lgPeerHistoryAddress, dropped, eorSent, engine, readErr)
		}
		return lgPeerHistoryEngine{}, fmt.Errorf("peer history: %s never reached %d drops and %d End-of-RIB sent: last read %+v",
			lgPeerHistoryAddress, dropped, eorSent, engine)
	}
	return engine, nil
}

// lgPeerHistoryRow answers the table row of the peer in a peers page or in one
// stream message.
func lgPeerHistoryRow(where, body string) (string, error) {
	for _, row := range strings.Split(body, "<tr")[1:] {
		if strings.Contains(row, "<td>"+lgPeerHistoryAddress+"</td>") {
			return row, nil
		}
	}
	return "", fmt.Errorf("peer history: %s holds no row for %s: %.600q", where, lgPeerHistoryAddress, body)
}

// lgPeerHistoryCheckRow checks that the peer's row shows the time of the last
// state change and the drop count the engine answers, and answers the row.
func lgPeerHistoryCheckRow(where, body string, want lgPeerHistoryEngine) (string, error) {
	row, err := lgPeerHistoryRow(where, body)
	if err != nil {
		return "", err
	}
	changed := lgPeerHistoryStateChangedCell.FindStringSubmatch(row)
	if changed == nil {
		return "", fmt.Errorf("peer history: the row of %s in %s has no state-changed cell, so it does not show when the session last changed: %.600q",
			lgPeerHistoryAddress, where, row)
	}
	if html.UnescapeString(changed[1]) != want.stateChanged {
		return "", fmt.Errorf("peer history: the row of %s in %s shows the last change at %q, and show bgp answers %q",
			lgPeerHistoryAddress, where, changed[1], want.stateChanged)
	}
	dropped := lgPeerHistoryDroppedCell.FindStringSubmatch(row)
	if dropped == nil {
		return "", fmt.Errorf("peer history: the row of %s in %s has no connections-dropped cell, so it does not show how often the session dropped: %.600q",
			lgPeerHistoryAddress, where, row)
	}
	if dropped[1] != strconv.Itoa(want.dropped) {
		return "", fmt.Errorf("peer history: the row of %s in %s shows %q drops, and show bgp answers %d",
			lgPeerHistoryAddress, where, dropped[1], want.dropped)
	}
	return row, nil
}

// lgPeerHistoryCheckLastError checks that the last error the engine answers is
// shown in the peer's detail. The detail is the row itself or the page the row
// opens, /lg/peer/{address}: either holds the text for an operator who looks
// at that peer.
func lgPeerHistoryCheckLastError(ctx context.Context, base, row, lastError string) error {
	shown := html.EscapeString(lastError)
	if strings.Contains(row, shown) {
		return nil
	}
	detail, err := lgCoveringRequest(ctx, http.MethodGet, base+"/lg/peer/"+lgPeerHistoryAddress, "")
	if err != nil {
		return err
	}
	if strings.Contains(detail, shown) {
		return nil
	}
	return fmt.Errorf("peer history: neither the row of %s nor /lg/peer/%s shows its last error %q: row %.400q",
		lgPeerHistoryAddress, lgPeerHistoryAddress, lastError, row)
}

// lgPeerHistoryWatchSecondDrop opens the live stream as an open peers page
// does, then announces the route the test peer answers with its second
// NOTIFICATION, and reads stream messages until one carries the peer's row
// with two drops.
func lgPeerHistoryWatchSecondDrop(ctx context.Context, p *sdk.Plugin, base string) error {
	streamCtx, cancel := context.WithTimeout(ctx, lgPeerHistoryStreamBudget)
	defer cancel()
	request, err := http.NewRequestWithContext(streamCtx, http.MethodGet, base+"/lg/events", http.NoBody)
	if err != nil {
		return err
	}
	// No client timeout: the stream stays open, and streamCtx bounds it.
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("peer history: open /lg/events: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // the stream is read until the verdict, then abandoned
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer history: /lg/events answered %d", resp.StatusCode)
	}

	if _, _, err := p.UpdateRoute(ctx, "*", lgPeerHistoryAnnounce); err != nil {
		return fmt.Errorf("peer history: announce the route that makes the peer drop again: %w", err)
	}

	lines := bufio.NewScanner(resp.Body)
	lines.Buffer(make([]byte, 0, 64*1024), lgPeerHistoryLineBytesMax)
	shown := ""
	for range lgPeerHistoryStreamMessages {
		message, err := lgPeerHistoryStreamMessage(lines)
		if err != nil {
			return fmt.Errorf("peer history: /lg/events ended before it carried two drops, last count shown %q: %w", shown, err)
		}
		row, err := lgPeerHistoryRow("a /lg/events message", message)
		if err != nil {
			return err
		}
		dropped := lgPeerHistoryDroppedCell.FindStringSubmatch(row)
		if dropped == nil {
			return fmt.Errorf("peer history: the row of %s in a /lg/events message has no connections-dropped cell: %.600q",
				lgPeerHistoryAddress, row)
		}
		shown = dropped[1]
		if shown == "2" {
			return nil
		}
	}
	engine, readErr := lgPeerHistoryRead(ctx, p)
	if readErr != nil {
		return fmt.Errorf("peer history: %d /lg/events messages never carried two drops for %s, last count shown %q: %w",
			lgPeerHistoryStreamMessages, lgPeerHistoryAddress, shown, readErr)
	}
	return fmt.Errorf("peer history: %d /lg/events messages never carried two drops for %s, last count shown %q; show bgp answers %+v",
		lgPeerHistoryStreamMessages, lgPeerHistoryAddress, shown, engine)
}

// lgPeerHistoryStreamMessage reads one server-sent message and answers its
// payload, the `data:` lines joined as the server split them.
func lgPeerHistoryStreamMessage(lines *bufio.Scanner) (string, error) {
	var payload []string
	for lines.Scan() {
		line := lines.Text()
		if line == "" {
			if len(payload) > 0 {
				return strings.Join(payload, "\n"), nil
			}
			continue
		}
		if text, isData := strings.CutPrefix(line, "data: "); isData {
			payload = append(payload, text)
		}
	}
	if err := lines.Err(); err != nil {
		return "", err
	}
	return "", io.ErrUnexpectedEOF
}
