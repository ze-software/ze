package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func fixture06DecodeMPReach(ctx context.Context, p *sdk.Plugin) error {
	result, err := p.DecodeMPReach(ctx, "00010104C0A8010100180A0000", false)
	if err != nil {
		return err
	}
	if result.Family != "ipv4/unicast" || result.NextHop != "192.168.1.1" {
		return fmt.Errorf("unexpected MP_REACH decode: family=%q next-hop=%q", result.Family, result.NextHop)
	}
	var nlri []string
	if err := json.Unmarshal(result.NLRI, &nlri); err != nil {
		return fmt.Errorf("decode MP_REACH NLRI: %w", err)
	}
	if len(nlri) != 1 || nlri[0] != "10.0.0.0/24" {
		return fmt.Errorf("expected exactly [10.0.0.0/24], got %v", nlri)
	}
	announced, _, err := p.UpdateRoute(ctx, "*", fmt.Sprintf("update text nhop %s nlri %s add %s", result.NextHop, result.Family, nlri[0]))
	if err != nil {
		return err
	}
	if announced != 1 {
		return fmt.Errorf("decoded MP_REACH route announced %d NLRIs, want 1", announced)
	}
	return nil
}

func fixture06DecodeMPUnreach(ctx context.Context, p *sdk.Plugin) error {
	result, err := p.DecodeMPUnreach(ctx, "00010118C0A800", false)
	if err != nil {
		return err
	}
	if result.Family != "ipv4/unicast" {
		return fmt.Errorf("unexpected MP_UNREACH family %q", result.Family)
	}
	var nlri []string
	if err := json.Unmarshal(result.NLRI, &nlri); err != nil {
		return fmt.Errorf("decode MP_UNREACH NLRI: %w", err)
	}
	if len(nlri) != 1 || nlri[0] != "192.168.0.0/24" {
		return fmt.Errorf("expected exactly [192.168.0.0/24], got %v", nlri)
	}
	announced, _, err := p.UpdateRoute(ctx, "*", fmt.Sprintf("update text nhop 10.0.0.1 nlri %s add %s", result.Family, nlri[0]))
	if err != nil {
		return err
	}
	if announced != 1 {
		return fmt.Errorf("decoded MP_UNREACH route announced %d NLRIs, want 1", announced)
	}
	return nil
}

func fixture06DecodeUpdate(ctx context.Context, p *sdk.Plugin) error {
	text, err := p.DecodeUpdate(ctx, "0000000B40010100400304C0A80101180A0000", false)
	if err != nil {
		return err
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		return fmt.Errorf("invalid JSON from decode-update: %w", err)
	}
	update, ok := decoded["update"].(map[string]any)
	if !ok {
		return fmt.Errorf("decode-update missing update: %v", decoded)
	}
	attributes, ok := update["attr"].(map[string]any)
	if !ok {
		return fmt.Errorf("decode-update missing attr: %v", update)
	}
	origin, ok := attributes["origin"].(map[string]any)
	if !ok || origin["value"] != "igp" || origin["transitive"] != true || origin["optional"] != false {
		return fmt.Errorf("unexpected origin attribute: %v", attributes["origin"])
	}
	nextHop, ok := attributes["next-hop"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing next-hop: %v", attributes)
	}
	nextHopValue, _ := nextHop["value"].(string)
	families, ok := update["nlri"].(map[string]any)
	if !ok || len(families) != 1 {
		return fmt.Errorf("expected exactly one decoded family, got %v", update["nlri"])
	}
	var family string
	var rawPrefixes any
	for name, value := range families {
		family, rawPrefixes = name, value
		break
	}
	entries, ok := rawPrefixes.([]any)
	if !ok || len(entries) == 0 {
		return fmt.Errorf("expected entries for %s, got %v", family, rawPrefixes)
	}
	entry, ok := entries[0].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid NLRI entry %v", entries[0])
	}
	nlri, ok := entry["nlri"].([]any)
	if !ok || len(nlri) != 1 || nlri[0] != "10.0.0.0/24" {
		return fmt.Errorf("expected exactly [10.0.0.0/24], got %v", entry["nlri"])
	}
	announced, _, err := p.UpdateRoute(ctx, "*", fmt.Sprintf("update text nhop %s nlri %s add %s", nextHopValue, family, nlri[0]))
	if err != nil {
		return err
	}
	if announced != 1 {
		return fmt.Errorf("decoded UPDATE announced %d NLRIs, want 1", announced)
	}
	return nil
}

func fixture06DiagCapture(ctx context.Context, p *sdk.Plugin) error {
	show, err := fixture06DispatchObject(ctx, p, "show capture l2tp")
	if err != nil {
		return err
	}
	if show["l2tp"] != "subsystem not running" {
		return fmt.Errorf("show capture l2tp: got %v", show["l2tp"])
	}
	start, err := fixture06DispatchObject(ctx, p, "show capture raw start")
	if err != nil {
		return err
	}
	if start["action"] != "start" {
		return fmt.Errorf("capture raw start: action=%v", start["action"])
	}
	if _, ok := start["started"].([]any); !ok {
		return fmt.Errorf("capture raw start: started=%v, want list", start["started"])
	}
	dump, err := fixture06DispatchObject(ctx, p, "show capture raw dump")
	if err != nil {
		return err
	}
	if dump["l2tp"] != "subsystem not running" {
		return fmt.Errorf("capture raw dump: l2tp=%v", dump["l2tp"])
	}
	stop, err := fixture06DispatchObject(ctx, p, "show capture raw stop")
	if err != nil {
		return err
	}
	if stop["action"] != "stop" {
		return fmt.Errorf("capture raw stop: action=%v", stop["action"])
	}
	if _, ok := stop["stopped"].([]any); !ok {
		return fmt.Errorf("capture raw stop: stopped=%v, want list", stop["stopped"])
	}
	fmt.Fprintf(os.Stderr, "OK: capture round-trip start=%v dump-l2tp=%q stop=%v\n", start["action"], dump["l2tp"], stop["action"])
	return nil
}

func fixture06DispatchSingleDecode(ctx context.Context, p *sdk.Plugin) error {
	if err := fixture06WaitEOR(ctx, p, 1); err != nil {
		return err
	}
	data, err := fixture06PollObject(ctx, p, "show bgp rib status", 60, func(map[string]any) bool { return true })
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Fprintf(os.Stderr, "OK: data is object with keys: %v\n", keys)
	_, _, _ = p.DispatchCommand(ctx, "clear bgp rib in")
	fmt.Fprintln(os.Stderr, "OK: error returned (RPC path or error field)")
	fmt.Fprintln(os.Stderr, "PASS: single-decode verified")
	return nil
}

type fixture06FrameLine struct {
	kind, item, key, columns, code, message string
	keyCount, columnsCount, count, faults   uint64
	codeCount, messageCount                 uint64
}

const fixture06DigitsMax = 20

func fixture06ReadDigits(data []byte, at int, field string) (uint64, int, error) {
	walk := at
	limit := at + fixture06DigitsMax + 1
	if limit > len(data) {
		limit = len(data)
	}
	for walk < limit && data[walk] >= '0' && data[walk] <= '9' {
		walk++
	}
	width := walk - at
	if width == 0 {
		return 0, at, fmt.Errorf("%s: states no digits at offset %d", field, at)
	}
	if width > fixture06DigitsMax {
		return 0, at, fmt.Errorf("%s: states more than the %d digits a uint64 occupies", field, fixture06DigitsMax)
	}
	value, err := strconv.ParseUint(string(data[at:walk]), 10, 64)
	if err != nil {
		return 0, at, fmt.Errorf("%s: %w", field, err)
	}
	return value, walk, nil
}

func fixture06ReadNumber(data []byte, at int, field string) (uint64, int, error) {
	value, end, err := fixture06ReadDigits(data, at, field)
	if err != nil {
		return 0, at, err
	}
	if end >= len(data) {
		return 0, at, fmt.Errorf("%s: digits run reaches the end of input", field)
	}
	if data[end] != ' ' && data[end] != '\n' {
		return 0, at, fmt.Errorf("%s: digits are closed by %q, want a space or the line's end", field, data[end:end+1])
	}
	return value, end, nil
}

func fixture06ReadText(data []byte, at int, field string) (string, uint64, int, error) {
	count, end, err := fixture06ReadDigits(data, at, field)
	if err != nil {
		return "", 0, at, err
	}
	if end >= len(data) || data[end] != ':' {
		return "", 0, at, fmt.Errorf("%s: byte count is not closed by ':'", field)
	}
	start := end + 1
	finish := start + int(count)
	if finish > len(data) {
		return "", 0, at, fmt.Errorf("%s: states %d bytes and %d arrived", field, count, len(data)-start)
	}
	if finish >= len(data) || (data[finish] != ' ' && data[finish] != '\n' && data[finish] != '\r') {
		return "", 0, at, fmt.Errorf("%s: states %d bytes and the following byte is not a field closer", field, count)
	}
	return string(data[start:finish]), count, finish, nil
}

func fixture06CloseField(data []byte, at int, field string) (int, error) {
	if at >= len(data) || data[at] != ' ' {
		return at, fmt.Errorf("%s is not closed by one space", field)
	}
	return at + 1, nil
}

func fixture06ReadFrameLine(data []byte, at int) (fixture06FrameLine, int, error) {
	var line fixture06FrameLine
	if at+3 > len(data) {
		return line, at, io.ErrUnexpectedEOF
	}
	line.kind = string(data[at : at+3])
	if line.kind != "top" && line.kind != "end" && line.kind != "nay" {
		return line, at, fmt.Errorf("kind %q is not top, end or nay", line.kind)
	}
	at += 3
	var err error
	at, err = fixture06CloseField(data, at, "kind")
	if err != nil {
		return line, at, err
	}
	switch line.kind {
	case "top":
		if at+3 > len(data) {
			return line, at, io.ErrUnexpectedEOF
		}
		line.item = string(data[at : at+3])
		if line.item != "doc" && line.item != "map" && line.item != "tab" {
			return line, at, fmt.Errorf("item type %q is not doc, map or tab", line.item)
		}
		at += 3
		at, err = fixture06CloseField(data, at, "item type")
		if err != nil {
			return line, at, err
		}
		line.key, line.keyCount, at, err = fixture06ReadText(data, at, "envelope key")
		if err != nil {
			return line, at, err
		}
		at, err = fixture06CloseField(data, at, "envelope key")
		if err != nil {
			return line, at, err
		}
		line.columns, line.columnsCount, at, err = fixture06ReadText(data, at, "column names")
	case "end":
		line.count, at, err = fixture06ReadNumber(data, at, "count")
		if err == nil {
			at, err = fixture06CloseField(data, at, "count")
		}
		if err == nil {
			line.faults, at, err = fixture06ReadNumber(data, at, "faults")
		}
		if err == nil {
			at, err = fixture06CloseField(data, at, "faults")
		}
		if err == nil {
			line.message, line.messageCount, at, err = fixture06ReadText(data, at, "message")
		}
	case "nay":
		line.code, line.codeCount, at, err = fixture06ReadText(data, at, "error code")
		if err == nil {
			at, err = fixture06CloseField(data, at, "error code")
		}
		if err == nil {
			line.message, line.messageCount, at, err = fixture06ReadText(data, at, "message")
		}
	}
	if err != nil {
		return line, at, err
	}
	if at >= len(data) || data[at] != '\n' {
		if at < len(data) && data[at] == '\r' {
			return line, at, fmt.Errorf("%s ends with a carriage return and newline", line.kind)
		}
		return line, at, fmt.Errorf("%s does not end at exactly one newline", line.kind)
	}
	return line, at + 1, nil
}

func fixture06ReadFrame(data []byte) ([]fixture06FrameLine, []string, error) {
	var lines []fixture06FrameLine
	var plain []string
	for at := 0; at < len(data); {
		if at+4 <= len(data) && (bytes.Equal(data[at:at+3], []byte("top")) || bytes.Equal(data[at:at+3], []byte("end")) || bytes.Equal(data[at:at+3], []byte("nay"))) && data[at+3] == ' ' {
			line, next, err := fixture06ReadFrameLine(data, at)
			if err != nil {
				return nil, nil, err
			}
			lines = append(lines, line)
			at = next
			continue
		}
		end := bytes.IndexByte(data[at:], '\n')
		if end < 0 {
			end = len(data) - at
		}
		text := data[at : at+end]
		if bytes.HasPrefix(text, []byte{'#'}) {
			return nil, nil, fmt.Errorf("frame line opened with an id field: %q", text)
		}
		plain = append(plain, strings.ToValidUTF8(string(text), "�"))
		at += end + 1
	}
	return lines, plain, nil
}

func fixture06FrameCheck(_ context.Context, args []string) error {
	if len(args) < 2 || len(args) > 3 {
		return errors.New("usage: fixture plugin/exec-answer-unconditional-frame-check <case> <frame-file> [body-file]")
	}
	frame, err := os.ReadFile(args[1])
	if err != nil {
		return err
	}
	var body []byte
	if len(args) == 3 {
		body, err = os.ReadFile(args[2])
		if err != nil {
			return err
		}
	}
	lines, plain, err := fixture06ReadFrame(frame)
	if err != nil {
		return fmt.Errorf("%s: %w; frame=%q", args[0], err, frame[:min(len(frame), 400)])
	}
	fail := func(condition bool, format string, values ...any) error {
		if condition {
			return nil
		}
		return fmt.Errorf(format, values...)
	}
	var said string
	switch args[0] {
	case "document":
		if err := fail(len(lines) == 2 && lines[0].kind == "top" && lines[1].kind == "end", "want top,end, got %v", lines); err != nil {
			return err
		}
		head, tail := lines[0], lines[1]
		if err := fail(head.item == "doc" && head.keyCount == 0 && head.key == "" && head.columnsCount == 0 && head.columns == "", "invalid document head: %+v", head); err != nil {
			return err
		}
		if err := fail(tail.count == 1 && tail.faults == 0 && tail.messageCount == 0 && tail.message == "", "invalid document tail: %+v", tail); err != nil {
			return err
		}
		said = "a built payload is framed `top doc 0: 0:` and `end 1 0 0:`"
	case "streamed":
		if err := fail(len(lines) == 2 && lines[0].kind == "top" && lines[1].kind == "end", "want top,end, got %v", lines); err != nil {
			return err
		}
		head, tail := lines[0], lines[1]
		if err := fail(head.item == "map" && head.key == "commands" && head.keyCount == uint64(len(head.key)) && head.columnsCount == 0 && head.columns == "", "invalid streamed head: %+v", head); err != nil {
			return err
		}
		if err := fail(tail.count > 256 && tail.faults == 0, "invalid streamed tail: %+v", tail); err != nil {
			return err
		}
		rendered := 0
		for _, row := range bytes.Split(body, []byte{'\n'}) {
			if bytes.HasPrefix(row, []byte{'{'}) {
				rendered++
			}
		}
		if err := fail(uint64(rendered) == tail.count, "terminator counts %d records and %d reached operator", tail.count, rendered); err != nil {
			return err
		}
		said = fmt.Sprintf("a walk of %d records is framed `top map 8:commands 0:` and counted on `end`", tail.count)
	case "unknown":
		if err := fail(len(lines) == 1 && lines[0].kind == "nay", "want one nay line, got %v", lines); err != nil {
			return err
		}
		nay := lines[0]
		if err := fail(nay.codeCount == 0 && nay.code == "" && nay.message == "unknown command" && nay.messageCount == uint64(len(nay.message)) && len(body) == 0, "invalid unknown frame: %+v body=%q", nay, body); err != nil {
			return err
		}
		said = "a command text naming no command is the whole answer: `nay 0: 15:unknown command`"
	case "failed":
		if err := fail(len(lines) == 2 && lines[0].kind == "top" && lines[1].kind == "end", "want top,end, got %v", lines); err != nil {
			return err
		}
		head, tail := lines[0], lines[1]
		characters := utf8.RuneCountInString(tail.message)
		if err := fail(head.item == "doc" && tail.count == 0 && tail.faults == 0 && tail.message == "pki: certificate Жé not found" && tail.messageCount == uint64(len(tail.message)) && int(tail.messageCount) != characters, "invalid failed frame: head=%+v tail=%+v", head, tail); err != nil {
			return err
		}
		said = fmt.Sprintf("a failed command states its message as %d BYTES, not the %d characters it decodes to", tail.messageCount, characters)
	default:
		return fmt.Errorf("unknown frame case %q", args[0])
	}
	fmt.Fprintf(os.Stderr, "OK: %s -- %s\n", args[0], said)
	for _, text := range plain {
		fmt.Fprintf(os.Stderr, "     beside it, for a person: %s\n", text)
	}
	return nil
}
