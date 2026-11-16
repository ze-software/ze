// Design: docs/labs/l2tp-interop.md -- readable selection of real equal-SID subscribers
// Related: scale.go -- this proof runs after the original FRR census, before teardown
package l2tp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var scaleSubscriberReference = regexp.MustCompile(`^l2tp-[0-9a-f]{32}$`)

type scaleSelectorSession struct {
	ID       string `json:"subscriber-id"`
	Listener string `json:"listener"`
	Tunnel   uint16 `json:"tunnel-local-tid"`
	Local    uint16 `json:"local-sid"`
	Remote   uint16 `json:"remote-sid"`
	Username string `json:"username"`
	Address  string `json:"assigned-addr"`
	State    string `json:"state"`
}

// readScaleSelectorSessions adds a stricter identity census without changing
// the existing population decoder or any of its old fixture expectations.
func readScaleSelectorSessions(raw []byte, original scalePopulation) (map[string]scaleSelectorSession, error) {
	var rows []scaleSelectorSession
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	if len(rows) != len(original.byUser) {
		return nil, errors.New("selector census changed subscriber count")
	}
	result := make(map[string]scaleSelectorSession, len(rows))
	ids := make(map[string]bool, len(rows))
	for _, row := range rows {
		expected, ok := original.byUser[row.Username]
		if !ok {
			return nil, fmt.Errorf("selector census has unknown subscriber %q", row.Username)
		}
		if _, duplicate := result[row.Username]; duplicate {
			return nil, errors.New("selector census duplicated a subscriber")
		}
		if row.Tunnel != expected.tunnel || row.Local != expected.local || row.Remote != expected.remote || row.Address != expected.address.String() || row.State != "established" {
			return nil, fmt.Errorf("selector census changed %s's established tuple/address", row.Username)
		}
		if !scaleSubscriberReference.MatchString(row.ID) || row.ID == "l2tp-00000000000000000000000000000000" {
			return nil, fmt.Errorf("%s has no canonical nonzero subscriber reference", row.Username)
		}
		if ids[row.ID] {
			return nil, errors.New("two real subscribers share one immutable reference")
		}
		endpoint, err := netip.ParseAddrPort(row.Listener)
		if err != nil || !endpoint.IsValid() || endpoint.Port() == 0 {
			return nil, fmt.Errorf("%s has no original listener endpoint", row.Username)
		}
		ids[row.ID], result[row.Username] = true, row
	}
	return result, nil
}

func scaleSelectorListeners(ctx context.Context, lab labOperations) (map[string]string, error) {
	raw, err := lnsPolicyCommand(ctx, lab, "show l2tp listeners")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		Port    uint16 `json:"port"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for _, row := range rows {
		address, err := netip.ParseAddr(row.Address)
		if err != nil || row.Port == 0 || row.Name == "" {
			return nil, errors.New("runtime listener inventory lacks a bound name/address")
		}
		endpoint := netip.AddrPortFrom(address, row.Port).String()
		if _, duplicate := result[endpoint]; duplicate {
			return nil, errors.New("runtime listener endpoint has more than one name")
		}
		result[endpoint] = row.Name
	}
	return result, nil
}

func scaleSelectorArguments(session scaleSelectorSession, listeners map[string]string) (string, error) {
	name := listeners[session.Listener]
	if name == "" {
		return "", errors.New("session's original listener has no runtime name")
	}
	return fmt.Sprintf("id %d tunnel-id %d listener %s", session.Local, session.Tunnel, strconv.Quote(name)), nil
}

// checkScaleReadableSelection uses the real authenticated REST dispatcher.
// Its rejected actions MUST leave all 48 original references, interfaces,
// routes, histories and accounting owners intact before the existing tail.
func checkScaleReadableSelection(ctx context.Context, lab labOperations, before scaleObservation, starts scaleAccounting, first, second string) error {
	raw, err := lnsPolicyCommand(ctx, lab, "show l2tp sessions")
	if err != nil {
		return err
	}
	original, err := readScaleSelectorSessions(raw, before.population)
	if err != nil {
		return err
	}
	listeners, err := scaleSelectorListeners(ctx, lab)
	if err != nil {
		return err
	}
	a, ok := original[first]
	if !ok {
		return errors.New("first equal-SID target is missing")
	}
	b, ok := original[second]
	if !ok || a.Local != b.Local || a.Tunnel == b.Tunnel || a.ID == b.ID {
		return errors.New("selector proof lacks distinct real admissions with equal SID")
	}
	for _, row := range []scaleSelectorSession{a, b} {
		if err := scaleSelectedViews(ctx, lab, row, listeners); err != nil {
			return err
		}
	}
	if err := scaleSelectorRefusal(ctx, lab, fmt.Sprintf("show l2tp session id %d", a.Local), "ambiguous"); err != nil {
		return err
	}
	if err := scaleSelectorRefusal(ctx, lab, fmt.Sprintf("clear l2tp session id %d reason %s", a.Local, strconv.Quote("selector ambiguity proof")), "ambiguous"); err != nil {
		return err
	}
	arguments, err := scaleSelectorArguments(b, listeners)
	if err != nil {
		return err
	}
	if err := scaleSelectorRefusal(ctx, lab, "clear l2tp session "+arguments+" expected-id "+a.ID+" reason "+strconv.Quote("foreign original reference proof"), "no l2tp session matches"); err != nil {
		return err
	}
	after, err := observeScalePopulation(ctx, lab, scaleUsers(scaleLACCount))
	if err != nil {
		return err
	}
	for user, device := range before.links {
		if after.links[user] != device {
			return fmt.Errorf("refused selector changed %s's native interface", user)
		}
	}
	raw, err = lnsPolicyCommand(ctx, lab, "show l2tp sessions")
	if err != nil {
		return err
	}
	current, err := readScaleSelectorSessions(raw, before.population)
	if err != nil {
		return err
	}
	for user, originalRow := range original {
		if current[user] != originalRow {
			return fmt.Errorf("refused selector changed %s's captured reference", user)
		}
		if err := scaleSelectedObserver(ctx, lab, originalRow, listeners); err != nil {
			return err
		}
	}
	if err := requireScaleFRR(ctx, lab, before.population, after.population); err != nil {
		return err
	}
	if err := scaleSelectorAccountingUnchanged(ctx, lab, before.population, starts); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, "L2TP-SCALE-SELECTORS: qualified equal-SID reads, ambiguous and foreign-reference refusal, all original subscribers/routes/histories/accounting retained")
	return err
}

func scaleSelectorRefusal(ctx context.Context, lab labOperations, command, required string) error {
	reply, err := lnsPolicyExecute(ctx, lab, command)
	if err != nil {
		return err
	}
	if reply.Status != "error" || !strings.Contains(strings.ToLower(reply.Error), required) {
		return fmt.Errorf("selector command %q did not return the required %q refusal: %s", command, required, reply.raw)
	}
	_, err = fmt.Fprintln(os.Stdout, "LNS selector refusal:", command, reply.Error)
	return err
}

func scaleSelectedScope(raw []byte, session scaleSelectorSession) (map[string]json.RawMessage, error) {
	var row map[string]json.RawMessage
	if err := json.Unmarshal(raw, &row); err != nil {
		return nil, err
	}
	var id, listener string
	var sid, tid uint16
	if err := json.Unmarshal(row["subscriber-id"], &id); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(row["listener"], &listener); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(row["session-id"], &sid); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(row["tunnel-id"], &tid); err != nil {
		return nil, err
	}
	if id != session.ID || listener != session.Listener || sid != session.Local || tid != session.Tunnel {
		return nil, errors.New("selected command returned another subscriber's scope")
	}
	return row, nil
}

func scaleSelectedViews(ctx context.Context, lab labOperations, session scaleSelectorSession, listeners map[string]string) error {
	arguments, err := scaleSelectorArguments(session, listeners)
	if err != nil {
		return err
	}
	raw, err := lnsPolicyCommand(ctx, lab, "show l2tp session "+arguments)
	if err != nil {
		return err
	}
	var detail scaleSelectorSession
	if err := json.Unmarshal(raw, &detail); err != nil {
		return err
	}
	if detail != session {
		return errors.New("qualified detail selected another real subscriber")
	}
	raw, err = lnsPolicyCommand(ctx, lab, "show l2tp session history "+arguments)
	if err != nil {
		return err
	}
	history, err := scaleSelectedScope(raw, session)
	if err != nil {
		return err
	}
	var transitions []json.RawMessage
	if err := json.Unmarshal(history["transitions"], &transitions); err != nil {
		return err
	}
	if len(transitions) == 0 {
		return errors.New("qualified history has no actual establishment transition")
	}
	if err := scaleSelectedObserver(ctx, lab, session, listeners); err != nil {
		return err
	}
	raw, err = lnsPolicyCommand(ctx, lab, "show l2tp session traffic "+arguments)
	if err != nil {
		return err
	}
	traffic, err := scaleSelectedScope(raw, session)
	if err != nil {
		return err
	}
	for _, name := range []string{"rx-bytes", "tx-bytes", "rx-packets", "tx-packets", "rx-errors", "tx-errors", "rx-dropped", "tx-dropped"} {
		var value *uint64
		if err := json.Unmarshal(traffic[name], &value); err != nil {
			return fmt.Errorf("qualified traffic lacks measured %s: %w", name, err)
		}
		if value == nil {
			return fmt.Errorf("qualified traffic reports unknown %s instead of a measurement", name)
		}
		if (name == "rx-packets" || name == "tx-packets") && *value == 0 {
			return fmt.Errorf("qualified %s has no traffic after the original bound pings", name)
		}
	}
	return nil
}

func scaleSelectedObserver(ctx context.Context, lab labOperations, session scaleSelectorSession, listeners map[string]string) error {
	arguments, err := scaleSelectorArguments(session, listeners)
	if err != nil {
		return err
	}
	raw, err := lnsPolicyCommand(ctx, lab, "show l2tp observer "+arguments)
	if err != nil {
		return err
	}
	row, err := scaleSelectedScope(raw, session)
	if err != nil {
		return err
	}
	var events []struct {
		ID   string `json:"subscriber-id"`
		SID  uint16 `json:"session-id"`
		TID  uint16 `json:"tunnel-id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(row["events"], &events); err != nil {
		return err
	}
	if len(events) == 0 {
		return errors.New("qualified observer has no real session history")
	}
	for _, event := range events {
		if event.ID != session.ID || event.SID != session.Local || event.TID != session.Tunnel {
			return errors.New("observer mixed equal-SID subscribers' histories")
		}
		if event.Type == "disconnect-requested" {
			return errors.New("refused selection recorded an administrative event on a subscriber")
		}
	}
	return nil
}

func scaleSelectorAccountingUnchanged(ctx context.Context, lab labOperations, population scalePopulation, starts scaleAccounting) error {
	detail, err := execOutput(ctx, lab, peerRadius, []string{commandCat, scaleAccountingPath})
	if err != nil {
		return err
	}
	current, err := readScaleAccounting(detail, population, "Start")
	if err != nil {
		return err
	}
	for user, id := range starts.byUser {
		if current.byUser[user] != id {
			return errors.New("refused selection changed an accounting owner")
		}
	}
	for line := range strings.SplitSeq(detail, "\n") {
		name, value, found := strings.Cut(strings.TrimSpace(line), " = ")
		if found && name == "Acct-Status-Type" && strings.Trim(value, "\"") == "Stop" {
			return errors.New("refused selection sent an Accounting-Stop")
		}
	}
	return nil
}
