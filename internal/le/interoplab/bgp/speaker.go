package bgp

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	bgpHeaderLength = 19
	bgpOpen         = 1
	bgpUpdate       = 2
	bgpNotification = 3
	bgpKeepalive    = 4
	mpReach         = 14
	mpUnreach       = 15
)

type pathAttribute struct {
	flags byte
	code  byte
	value []byte
}

type speakerUpdate struct {
	withdrawn  []byte
	attributes []pathAttribute
	nlri       []byte
	raw        []byte
}

type speakerVerdict struct {
	plugin   string
	failures []string
	notes    []string
	evpnNLRI int
}

func (v *speakerVerdict) fail(message string) {
	v.failures = append(v.failures, "["+v.plugin+"] "+message)
}

func decodeSpeakerUpdate(body []byte) speakerUpdate {
	result := speakerUpdate{raw: body}
	if len(body) < 4 {
		return result
	}
	withdrawnLength := min(int(binary.BigEndian.Uint16(body)), len(body)-2)
	result.withdrawn = body[2 : 2+withdrawnLength]
	attributeLengthOffset := 2 + withdrawnLength
	if attributeLengthOffset+2 > len(body) {
		return result
	}
	attributeStart := attributeLengthOffset + 2
	attributeLength := min(int(binary.BigEndian.Uint16(body[attributeLengthOffset:])), len(body)-attributeStart)
	attributeEnd := attributeStart + attributeLength
	result.attributes = decodeSpeakerAttributes(body[attributeStart:attributeEnd])
	result.nlri = body[attributeEnd:]
	return result
}

func decodeSpeakerAttributes(data []byte) []pathAttribute {
	attributes := make([]pathAttribute, 0, 8)
	for position := 0; position+2 <= len(data); {
		flags, code := data[position], data[position+1]
		headerLength := 3
		if position+headerLength > len(data) {
			break
		}
		length := int(data[position+2])
		if flags&0x10 != 0 {
			headerLength = 4
			if position+headerLength > len(data) {
				break
			}
			length = int(binary.BigEndian.Uint16(data[position+2:]))
		}
		end := position + headerLength + length
		if end > len(data) {
			break
		}
		attributes = append(attributes, pathAttribute{flags: flags, code: code, value: data[position+headerLength : end]})
		position = end
	}
	return attributes
}

func speakerCarriesRoutes(update speakerUpdate) bool {
	if len(update.nlri) > 0 || len(update.withdrawn) > 0 {
		return true
	}
	for _, attribute := range update.attributes {
		switch attribute.code {
		case mpReach:
			if len(attribute.value) >= 4 && len(attribute.value) > 5+int(attribute.value[3]) {
				return true
			}
		case mpUnreach:
			if len(attribute.value) > 3 {
				return true
			}
		}
	}
	return false
}

type bgpFamily struct {
	afi  uint16
	safi byte
}

type familyFlags []bgpFamily

func (f *familyFlags) String() string {
	values := make([]string, len(*f))
	for index, family := range *f {
		values[index] = fmt.Sprintf("%d:%d", family.afi, family.safi)
	}
	return strings.Join(values, ",")
}

func (f *familyFlags) Set(value string) error {
	afiText, safiText, ok := strings.Cut(value, ":")
	if !ok || afiText == "" || safiText == "" {
		return fmt.Errorf("--family wants AFI:SAFI, got %q", value)
	}
	afi, err := strconv.ParseUint(afiText, 10, 16)
	if err != nil {
		return fmt.Errorf("parse AFI %q: %w", afiText, err)
	}
	safi, err := strconv.ParseUint(safiText, 10, 8)
	if err != nil {
		return fmt.Errorf("parse SAFI %q: %w", safiText, err)
	}
	*f = append(*f, bgpFamily{afi: uint16(afi), safi: byte(safi)})
	return nil
}

func speakerMessage(messageType byte, body []byte) []byte {
	message := make([]byte, bgpHeaderLength+len(body))
	for index := 0; index < 16; index++ {
		message[index] = 0xff
	}
	binary.BigEndian.PutUint16(message[16:18], uint16(len(message)))
	message[18] = messageType
	copy(message[bgpHeaderLength:], body)
	return message
}

func speakerOpen(asn uint32, holdTime uint16, routerID net.IP, families []bgpFamily, addPath bool) ([]byte, error) {
	if len(families) == 0 {
		families = []bgpFamily{{afi: 1, safi: 1}}
	}
	routerID = routerID.To4()
	if routerID == nil {
		return nil, errors.New("speaker router-id must be IPv4")
	}
	capabilities := make([]byte, 0, 32)
	for _, family := range families {
		capabilities = append(capabilities, 1, 4, byte(family.afi>>8), byte(family.afi), 0, family.safi)
	}
	capabilities = append(capabilities, 65, 4, byte(asn>>24), byte(asn>>16), byte(asn>>8), byte(asn))
	if addPath {
		valueLength := len(families) * 4
		if valueLength > 255 {
			return nil, errors.New("ADD-PATH family capability is too large")
		}
		capabilities = append(capabilities, 69, byte(valueLength))
		for _, family := range families {
			capabilities = append(capabilities, byte(family.afi>>8), byte(family.afi), family.safi, 1)
		}
	}
	if len(capabilities) > 255 {
		return nil, errors.New("speaker capability parameter is too large")
	}
	myAS := uint16(asn)
	if asn > 0xffff {
		myAS = 23456
	}
	body := make([]byte, 10+len(capabilities))
	body[0] = 4
	binary.BigEndian.PutUint16(body[1:3], myAS)
	binary.BigEndian.PutUint16(body[3:5], holdTime)
	copy(body[5:9], routerID)
	body[9] = byte(2 + len(capabilities))
	body = append(body[:10], 2, byte(len(capabilities)))
	body = append(body, capabilities...)
	return speakerMessage(bgpOpen, body), nil
}

func speakerEOR() []byte       { return speakerMessage(bgpUpdate, []byte{0, 0, 0, 0}) }
func speakerKeepalive() []byte { return speakerMessage(bgpKeepalive, nil) }

type speakerOptions struct {
	connect      string
	asn          uint
	routerID     string
	test         string
	result       string
	holdTime     uint
	duration     time.Duration
	stopAfter    int
	connectDelay time.Duration
	families     familyFlags
	addPath      bool
}

func parseSpeakerOptions(args []string) (speakerOptions, error) {
	options := speakerOptions{}
	flags := flag.NewFlagSet("interop-bgp speaker", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.connect, "connect", "", "Ze address")
	flags.UintVar(&options.asn, "asn", 0, "local ASN")
	flags.StringVar(&options.routerID, "router-id", "", "BGP router ID")
	flags.StringVar(&options.test, "test", "", "native oracle")
	flags.StringVar(&options.result, "result", "", "result path")
	flags.UintVar(&options.holdTime, "hold-time", 90, "hold time")
	duration := flags.Float64("duration", 45, "duration in seconds")
	flags.IntVar(&options.stopAfter, "stop-after-updates", 0, "route-bearing update limit")
	connectDelay := flags.Float64("connect-delay", 0, "connect delay in seconds")
	flags.Var(&options.families, "family", "AFI:SAFI")
	flags.BoolVar(&options.addPath, "add-path", false, "receive ADD-PATH")
	if err := flags.Parse(args); err != nil {
		return options, err
	}
	if options.connect == "" || options.asn == 0 || options.routerID == "" || options.test == "" {
		return options, errors.New("speaker requires --connect, --asn, --router-id, and --test")
	}
	options.duration = time.Duration(*duration * float64(time.Second))
	options.connectDelay = time.Duration(*connectDelay * float64(time.Second))
	return options, nil
}

func runSpeakerHelper(args []string, output io.Writer) error {
	options, err := parseSpeakerOptions(args)
	if err != nil {
		return err
	}
	plugin := filepath.Base(options.test)
	switch plugin {
	case "no-duplicate-attribute":
		plugin = "no-duplicate-attribute"
	case "no-unrecognized-evpn-type":
		plugin = "no-unrecognized-evpn-type"
	default:
		return fmt.Errorf("unknown native speaker oracle %q", options.test)
	}
	verdict := &speakerVerdict{plugin: plugin}
	if err := runSpeakerSession(options, verdict); err != nil {
		verdict.fail("engine crashed: " + err.Error())
	}
	status := "PASS"
	if len(verdict.failures) > 0 {
		status = "FAIL"
	}
	var report strings.Builder
	_, _ = fmt.Fprintf(&report, "result: %s\nplugin: %s\n", status, verdict.plugin)
	for _, failure := range verdict.failures {
		_, _ = fmt.Fprintf(&report, "fail: %s\n", failure)
	}
	for _, note := range verdict.notes {
		_, _ = fmt.Fprintf(&report, "note: %s\n", note)
	}
	if _, err := io.WriteString(output, report.String()); err != nil {
		return err
	}
	if options.result != "" {
		if err := os.WriteFile(options.result, []byte(report.String()), 0o600); err != nil {
			return err
		}
	}
	if status == "FAIL" {
		return errors.New("native speaker oracle failed")
	}
	return nil
}

func runSpeakerSession(options speakerOptions, verdict *speakerVerdict) error {
	if options.connectDelay > 0 {
		time.Sleep(options.connectDelay)
	}
	connection, err := net.DialTimeout("tcp", options.connect, 10*time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }()
	open, err := speakerOpen(uint32(options.asn), uint16(options.holdTime), net.ParseIP(options.routerID), options.families, options.addPath)
	if err != nil {
		return err
	}
	if _, err := connection.Write(open); err != nil {
		return err
	}
	deadline := time.Now().Add(options.duration)
	nextKeepalive := time.Now().Add(time.Duration(options.holdTime) * time.Second / 3)
	established, routes := false, 0
	for time.Now().Before(deadline) {
		if !time.Now().Before(nextKeepalive) {
			if _, err := connection.Write(speakerKeepalive()); err != nil {
				break
			}
			nextKeepalive = time.Now().Add(time.Duration(options.holdTime) * time.Second / 3)
		}
		messageType, body, idle, err := readSpeakerMessage(connection)
		if idle {
			continue
		}
		if err != nil {
			if !established {
				verdict.notes = append(verdict.notes, "peer closed before establishment")
			}
			break
		}
		switch messageType {
		case bgpOpen:
			_, err = connection.Write(speakerKeepalive())
		case bgpKeepalive:
			if !established {
				established = true
				_, err = connection.Write(speakerEOR())
			}
			if err == nil {
				_, err = connection.Write(speakerKeepalive())
			}
		case bgpUpdate:
			if !established {
				established = true
				_, err = connection.Write(speakerEOR())
			}
			update := decodeSpeakerUpdate(body)
			applySpeakerOracle(update, verdict)
			if speakerCarriesRoutes(update) {
				routes++
				verdict.notes = append(verdict.notes, fmt.Sprintf("update-hex: %x", body))
				if options.stopAfter > 0 && routes >= options.stopAfter {
					deadline = time.Time{}
				}
			}
		case bgpNotification:
			verdict.notes = append(verdict.notes, fmt.Sprintf("received NOTIFICATION: %x", body))
			deadline = time.Time{}
		}
		if err != nil {
			verdict.notes = append(verdict.notes, "send failed, peer closed")
			break
		}
	}
	verdict.notes = append(verdict.notes, fmt.Sprintf("route-bearing-updates: %d", routes))
	if established {
		verdict.notes = append(verdict.notes, "established: yes")
	} else {
		verdict.notes = append(verdict.notes, "established: no")
	}
	if verdict.plugin == "no-unrecognized-evpn-type" {
		verdict.notes = append(verdict.notes, fmt.Sprintf("evpn-nlri: %d", verdict.evpnNLRI))
	}
	return nil
}

func readSpeakerMessage(connection net.Conn) (byte, []byte, bool, error) {
	header, idle, err := readSpeakerExact(connection, bgpHeaderLength)
	if idle || err != nil {
		return 0, nil, idle, err
	}
	length := int(binary.BigEndian.Uint16(header[16:18]))
	if length <= bgpHeaderLength {
		return header[18], nil, false, nil
	}
	body, _, err := readSpeakerExact(connection, length-bgpHeaderLength)
	return header[18], body, false, err
}

func readSpeakerExact(connection net.Conn, size int) ([]byte, bool, error) {
	buffer := make([]byte, size)
	position := 0
	for position < size {
		_ = connection.SetReadDeadline(time.Now().Add(time.Second))
		count, err := connection.Read(buffer[position:])
		position += count
		if err == nil {
			continue
		}
		if timeout, ok := errors.AsType[net.Error](err); ok && timeout.Timeout() {
			if position == 0 {
				return nil, true, nil
			}
			continue
		}
		return nil, false, err
	}
	return buffer, false, nil
}

func applySpeakerOracle(update speakerUpdate, verdict *speakerVerdict) {
	switch verdict.plugin {
	case "no-duplicate-attribute":
		seen := make(map[byte]struct{}, len(update.attributes))
		for _, attribute := range update.attributes {
			if _, exists := seen[attribute.code]; exists {
				verdict.fail(fmt.Sprintf("path attribute type %d appears more than once in one UPDATE", attribute.code))
			}
			seen[attribute.code] = struct{}{}
		}
	case "no-unrecognized-evpn-type":
		applyEVPNOracle(update, verdict)
	}
}

func applyEVPNOracle(update speakerUpdate, verdict *speakerVerdict) {
	for _, attribute := range update.attributes {
		value := attribute.value
		if (attribute.code != mpReach && attribute.code != mpUnreach) || len(value) < 3 ||
			binary.BigEndian.Uint16(value[:2]) != 25 || value[2] != 70 {
			continue
		}
		nlri := value[3:]
		if attribute.code == mpReach {
			if len(value) < 4 {
				continue
			}
			start := 5 + int(value[3])
			if start > len(value) {
				continue
			}
			nlri = value[start:]
		}
		for offset := 0; offset < len(nlri); {
			if offset+2 > len(nlri) {
				verdict.fail(fmt.Sprintf("EVPN NLRI section is not well framed: truncated EVPN header at offset %d", offset))
				break
			}
			length := int(nlri[offset+1])
			if offset+2+length > len(nlri) {
				verdict.fail(fmt.Sprintf("EVPN NLRI section is not well framed: EVPN NLRI at offset %d overruns the section", offset))
				break
			}
			routeType := nlri[offset]
			verdict.evpnNLRI++
			if routeType < 1 || routeType > 5 {
				verdict.fail(fmt.Sprintf("RFC 7606 Section 5.4: received EVPN route type %d, which is not assigned; it must have been discarded, not relayed", routeType))
			}
			offset += 2 + length
		}
	}
}
