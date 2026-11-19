// Design: docs/architecture/web-interface.md -- named next-hop ranges in the route table and the next-hop graph, across a reload
// Related: lg_covering_lookup.go -- lgCoveringRequest, the reader that waits for the listener
// Related: bgp_best_route_transit.go -- bgpBestRouteReload, the reload through the daemon's own command
// Related: plugin_fixture_09.go -- the registration, observe09 and dispatch09

package fixture

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// lgNamedPrefix is the route whose next hop, lgNamedNextHop, sits inside
	// the range the test config names: 198.51.100.0/24.
	lgNamedPrefix  = "10.60.1.0/24"
	lgNamedNextHop = "198.51.100.7"
	// lgUnnamedPrefix is the route whose next hop sits in no named range.
	lgUnnamedPrefix  = "10.60.2.0/24"
	lgUnnamedNextHop = "203.0.113.9"
	// The name the first config gives the range, and the one the reloaded
	// config gives the same range.
	lgNamedFirst   = "ixp-lan-a"
	lgNamedRenamed = "ixp-lan-b"
	// lgNamedAnswerBudget bounds one request on the held connection.
	lgNamedAnswerBudget = 10 * time.Second
	// lgNamedGraphNameField starts the name field of a node line in the
	// next-hop graph's text form (internal/component/lg/graph_nexthop.go,
	// renderNextHopGraphText).
	lgNamedGraphNameField = "name="
)

// lgNamedNextHopDriver takes the config file the daemon reads and the file
// that holds the same config with the range renamed.
func lgNamedNextHopDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("named next hop requires <active config file> <renamed config file>")
	}
	activeConfig, renamedConfig := args[0], args[1]
	return observe09(ctx, "lg-named-next-hop", func(ctx context.Context, p *sdk.Plugin) error {
		if err := p12WaitPeersEOR(ctx, p, "peer1"); err != nil {
			return err
		}
		for prefix, nextHop := range map[string]string{lgNamedPrefix: lgNamedNextHop, lgUnnamedPrefix: lgUnnamedNextHop} {
			if err := lgNamedInject(ctx, p, prefix, nextHop); err != nil {
				return err
			}
		}
		port := os.Getenv("ZE_LG_PORT")
		if port == "" {
			return errors.New("named next hop: ZE_LG_PORT not set in plugin environment")
		}
		host := "127.0.0.1:" + port
		// lgCoveringRequest retries until the listener is up, so the dial
		// below meets a listening socket.
		if _, err := lgCoveringRequest(ctx, http.MethodGet, "http://"+host+"/lg/help", ""); err != nil {
			return err
		}
		// One connection carries every request, before and after the reload:
		// an answer on it after the reload comes from the listener that
		// accepted it before.
		conn, err := dialLGNamed(ctx, host)
		if err != nil {
			return err
		}
		defer conn.close()

		if err := lgNamedCheck(ctx, conn, lgNamedFirst, lgNamedRenamed); err != nil {
			return fmt.Errorf("before the reload: %w", err)
		}
		if err := lgNamedCheckUnnamed(ctx, conn); err != nil {
			return fmt.Errorf("before the reload: %w", err)
		}

		renamed, err := os.ReadFile(renamedConfig) //nolint:gosec // The .ci invocation names this functional run's own renamed configuration file.
		if err != nil {
			return err
		}
		if err := bgpBestRouteReload(ctx, p, activeConfig, string(renamed)); err != nil {
			return fmt.Errorf("named next hop: reload with the range renamed: %w", err)
		}

		if err := lgNamedCheck(ctx, conn, lgNamedRenamed, lgNamedFirst); err != nil {
			return fmt.Errorf("after the reload, on the connection opened before it: %w", err)
		}
		fmt.Fprintf(os.Stderr, "OK: next hop %s was named %s in the route table and the next-hop graph, and %s after the reload on the same listener\n",
			lgNamedNextHop, lgNamedFirst, lgNamedRenamed)
		return nil
	})
}

// lgNamedInject injects one route from addrLGPeerFirst with the next hop given.
func lgNamedInject(ctx context.Context, p *sdk.Plugin, prefix, nextHop string) error {
	command := cmdRIBInject + addrLGPeerFirst + " " + familyIPv4Unicast + " " + prefix +
		" origin igp nhop " + nextHop + " aspath 2914,65100"
	r := dispatch09(ctx, p, command)
	if !done09(r) {
		return fmt.Errorf("%s: status=%s data=%s", command, r.status, r.text)
	}
	return nil
}

// lgNamedCheck checks that the route table row of lgNamedPrefix and the
// next-hop graph node of lgNamedNextHop both carry name, and that neither
// carries absent, the name the range does not have at this point.
func lgNamedCheck(ctx context.Context, conn *lgNamedConn, name, absent string) error {
	row, err := lgNamedSearchRow(ctx, conn, lgNamedPrefix)
	if err != nil {
		return err
	}
	if !strings.Contains(row, lgNamedNextHop) {
		return fmt.Errorf("named next hop: the route table row of %s does not show next hop %s: %.400q", lgNamedPrefix, lgNamedNextHop, row)
	}
	if !strings.Contains(row, name) {
		return fmt.Errorf("named next hop: the route table row of %s shows next hop %s without the name %s of its range: %.400q",
			lgNamedPrefix, lgNamedNextHop, name, row)
	}
	if strings.Contains(row, absent) {
		return fmt.Errorf("named next hop: the route table row of %s shows the name %s, which its range does not have: %.400q", lgNamedPrefix, absent, row)
	}
	node, err := lgNamedGraphNode(ctx, conn, lgNamedPrefix, lgNamedNextHop)
	if err != nil {
		return err
	}
	if !slices.Contains(node, lgNamedGraphNameField+name) {
		return fmt.Errorf("named next hop: the next-hop graph node of %s is %v, without %s%s", lgNamedNextHop, node, lgNamedGraphNameField, name)
	}
	return nil
}

// lgNamedCheckUnnamed checks that a next hop in no named range shows its
// address alone, in the route table and in the next-hop graph.
func lgNamedCheckUnnamed(ctx context.Context, conn *lgNamedConn) error {
	row, err := lgNamedSearchRow(ctx, conn, lgUnnamedPrefix)
	if err != nil {
		return err
	}
	for _, name := range []string{lgNamedFirst, lgNamedRenamed} {
		if strings.Contains(row, name) {
			return fmt.Errorf("named next hop: the route table row of %s shows the name %s for next hop %s, which is in no named range: %.400q",
				lgUnnamedPrefix, name, lgUnnamedNextHop, row)
		}
	}
	node, err := lgNamedGraphNode(ctx, conn, lgUnnamedPrefix, lgUnnamedNextHop)
	if err != nil {
		return err
	}
	for _, field := range node {
		if strings.HasPrefix(field, lgNamedGraphNameField) {
			return fmt.Errorf("named next hop: the next-hop graph node of %s, which is in no named range, carries %s", lgUnnamedNextHop, field)
		}
	}
	return nil
}

// lgNamedSearchRow searches the page for one prefix and answers its table row.
func lgNamedSearchRow(ctx context.Context, conn *lgNamedConn, prefix string) (string, error) {
	body, err := conn.ask(ctx, http.MethodPost, "/lg/search", url.Values{columnPrefix: {prefix}}.Encode())
	if err != nil {
		return "", err
	}
	for _, row := range strings.Split(body, "<tr")[1:] {
		if strings.Contains(row, "<td>"+prefix+"</td>") {
			return row, nil
		}
	}
	return "", fmt.Errorf("named next hop: the search for %s lists no row for it: %.600q", prefix, body)
}

// lgNamedGraphNode asks the next-hop graph of one prefix in its text form and
// answers the fields of the node line of one address.
func lgNamedGraphNode(ctx context.Context, conn *lgNamedConn, prefix, address string) ([]string, error) {
	query := url.Values{columnPrefix: {prefix}, "mode": {"nexthop"}, "format": {"text"}}.Encode()
	body, err := conn.ask(ctx, http.MethodGet, "/lg/graph?"+query, "")
	if err != nil {
		return nil, err
	}
	for line := range strings.SplitSeq(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] != "node" {
			continue
		}
		if fields[1] == address {
			return fields, nil
		}
	}
	return nil, fmt.Errorf("named next hop: the next-hop graph of %s has no node %s: %.400q", prefix, address, body)
}

// lgNamedConn is one plaintext connection to the looking glass, with the
// reader that owns whatever the last response left buffered, so a caller holds
// the connection open across a reload and uses it again. Not safe for
// concurrent use. MUST be closed with close.
type lgNamedConn struct {
	conn   net.Conn
	reader *bufio.Reader
	host   string
}

// dialLGNamed opens the connection. The caller MUST call close on it.
func dialLGNamed(ctx context.Context, host string) (*lgNamedConn, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("named next hop: dial the looking glass at %s: %w", host, err)
	}
	return &lgNamedConn{conn: conn, reader: bufio.NewReader(conn), host: host}, nil
}

// close MUST be called once, after the last ask.
func (c *lgNamedConn) close() {
	_ = c.conn.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion
}

// ask sends one request on the held connection and answers its body. An
// answer that is not 200, and a connection the server closed, are each an
// error.
func (c *lgNamedConn) ask(ctx context.Context, method, target, form string) (string, error) {
	var body io.Reader = http.NoBody
	if form != "" {
		body = strings.NewReader(form)
	}
	request, err := http.NewRequestWithContext(ctx, method, "http://"+c.host+target, body)
	if err != nil {
		return "", err
	}
	if form != "" {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err := c.conn.SetDeadline(time.Now().Add(lgNamedAnswerBudget)); err != nil {
		return "", err
	}
	if err := request.Write(c.conn); err != nil {
		return "", fmt.Errorf("named next hop: send %s %s on the held connection: %w", method, target, err)
	}
	resp, err := http.ReadResponse(c.reader, request)
	if err != nil {
		return "", fmt.Errorf("named next hop: no answer to %s %s on the held connection: %w", method, target, err)
	}
	defer resp.Body.Close() //nolint:errcheck // the body is read
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("named next hop: read %s %s: %w", method, target, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("named next hop: %s %s answered %d: %.300q", method, target, resp.StatusCode, data)
	}
	return string(data), nil
}
