// Design: docs/plugin-development/protocol.md -- The Route Fields of filter-update
// Related: register_writers.go -- the writer registrations, one per BGP field
// Related: readers.go -- the readers each writer reads through
//
// The writers turn the fields a forked filter declared into the JSON object
// the filter-update message carries (spec-filter-wire-0-umbrella D-1). Each
// value takes the shape the UPDATE event's attr map gives it, written by the
// same registered event formatter, so a plugin author reads one JSON style for
// events and filters. Only the key differs: the message keys a value by its
// vocabulary field name (community), never by the event key (communities).

package policyview

import (
	"errors"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ErrNoWriter is wrapped by CheckFieldWriter and AppendFields for a field the
// message cannot carry: one that is not a field of a BGP route, or one whose
// event formatter comes with a plugin this binary does not link.
var ErrNoWriter = errors.New("no JSON writer for the field")

// ErrNotBGPField is wrapped, beside ErrNoWriter, for a name no BGP route
// carries (metric, tag): the BGP attachment check at config load refuses a
// chain that reads one, naming the attachment, so Stage 1 leaves it to it.
var ErrNotBGPField = errors.New("not a field of a BGP route")

// writeFunc appends the JSON value of one field the reader answered present,
// and false when the field states nothing for an announced route (a NEXT_HOP
// beside no body NLRI). buf is returned as it was on false or on an error.
type writeFunc func(buf []byte, v *View, value Value) ([]byte, bool, error)

// fieldWriter is one field's registered writer.
type fieldWriter struct {
	field  policy.Field
	reader Reader
	write  writeFunc
	// code is the attribute whose event formatter writes the value, checked
	// by CheckFieldWriter; zero for a field the BGP side writes itself.
	code attribute.AttributeCode
	// provider is the plugin that registers code's formatter, named when the
	// binary does not link it; empty for an engine formatter.
	provider string
}

// writerTable holds the writers by field name, filled from init() only and
// read-only after, so it is safe for concurrent use.
var writerTable = map[string]*fieldWriter{}

// mustRegisterWriter registers the writer of field. The field MUST already
// have a reader: the writer reads through it, so the message and an in-process
// member see the same value, the read-after-set overlay included. A second
// writer, or a field with no reader, is a Ze defect and panics.
func mustRegisterWriter(field policy.Field, write writeFunc, code attribute.AttributeCode, provider string) {
	reader, err := ReaderFor(field)
	if err != nil {
		panic("BUG: policyview: writer for " + field.String() + " has no reader: " + err.Error())
	}
	if _, dup := writerTable[field.String()]; dup {
		panic("BUG: policyview: field " + field.String() + " has two writers")
	}
	writerTable[field.String()] = &fieldWriter{field: field, reader: reader, write: write, code: code, provider: provider}
}

// CheckFieldWriter answers nil when the filter-update message can carry the
// field named name in this binary, and an error wrapping ErrNoWriter that
// names the field and the fix when it cannot. Stage 1 refuses a filter that
// declares such a field, because a field sent empty would read as "the route
// does not carry it" (spec R-10, AC-29).
func CheckFieldWriter(name string) error {
	w, ok := writerTable[name]
	if !ok {
		return errors.Join(ErrNotBGPField, noWriter(name, " is not a field of a BGP route, so no filter-update can carry it; "+
			"declare a BGP field (docs/plugin-development/protocol.md, \"The Route Fields of filter-update\")"))
	}
	if w.code != 0 && attribute.GetJSONFormatter(w.code) == nil {
		return noWriter(name, " cannot be written in this binary: its value formatter comes with plugin "+
			w.provider+", which the binary does not link; build Ze with that plugin, or stop declaring the field")
	}
	return nil
}

// noWriter builds the error of CheckFieldWriter. It runs at Stage 1 only.
func noWriter(name, why string) error {
	var b textbuf.Buffer
	b.Str("field ").Quoted(name).Str(why)
	return errors.Join(ErrNoWriter, errors.New(b.String()))
}

// AppendFields appends the fields names declares, read from the route on v,
// as one JSON object keyed by field name, in declaration order. A declared
// field the route does not carry is left out. The raw read is not a route
// field and is skipped: it travels in the message's raw member. When names
// holds no route field, buf is returned unchanged, so the caller sends no
// fields object at all.
//
// A field that cannot be read (a malformed attribute, ErrMalformed) or written
// answers an error and buf as it was: the filter is never sent a message that
// omits a field it asked for, which would read as absence. v MUST be Reset
// for this route; its values are borrowed and copied into buf here.
func AppendFields(buf []byte, v *View, names []string) ([]byte, error) {
	mark := len(buf)
	opened := false
	for _, name := range names {
		if name == rpc.FilterFieldRaw {
			continue
		}
		if !opened {
			buf = append(buf, '{')
			opened = true
		}
		w, ok := writerTable[name]
		if !ok {
			return buf[:mark], errors.Join(ErrNotBGPField, noWriter(name, " is not a field of a BGP route"))
		}
		value, err := v.Read(w.reader)
		if err != nil {
			return buf[:mark], err
		}
		if !value.Present() {
			continue
		}
		key := len(buf)
		if buf[key-1] != '{' {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = append(buf, name...)
		buf = append(buf, `":`...)
		out, written, err := w.write(buf, v, value)
		if err != nil {
			return buf[:mark], err
		}
		if !written {
			buf = buf[:key]
			continue
		}
		buf = out
	}
	if !opened {
		return buf, nil
	}
	return append(buf, '}'), nil
}

// formatted answers the write of a field whose value the registered event
// formatter of code writes, from the attribute the UPDATE carries.
func formatted(code attribute.AttributeCode) writeFunc {
	return func(buf []byte, v *View, _ Value) ([]byte, bool, error) {
		attrs, err := v.attrs()
		if err != nil {
			return buf, false, err
		}
		if attrs == nil {
			return buf, false, errors.New("BUG: policyview: a present attribute field with no attributes")
		}
		attr, err := attrs.Get(code)
		if err != nil {
			return buf, false, errors.Join(ErrMalformed, err)
		}
		return appendFormatted(buf, code, attr)
	}
}

// appendFormatted writes attr through the event formatter of code.
func appendFormatted(buf []byte, code attribute.AttributeCode, attr attribute.Attribute) ([]byte, bool, error) {
	f := attribute.GetJSONFormatter(code)
	if f == nil {
		return buf, false, errors.Join(ErrNoWriter, errors.New("the formatter of attribute "+strconv.Itoa(int(code))+" is not linked"))
	}
	out := f.AppendValue(buf, attr)
	if out == nil {
		return buf, false, errors.Join(ErrMalformed, errors.New("the formatter of attribute "+strconv.Itoa(int(code))+" cannot write the value"))
	}
	return out, true, nil
}

// writeASPath writes as-path through the AS_PATH event formatter, from the
// path the view reconstructed (RFC 6793 Section 4.2.3), not from the raw
// attribute.
func writeASPath(buf []byte, _ *View, value Value) ([]byte, bool, error) {
	return appendFormatted(buf, attribute.AttrASPath, value.ASPath())
}

// writeMED writes med from the view's number, so a value an earlier member set
// is the value written, through the MED event formatter.
func writeMED(buf []byte, _ *View, value Value) ([]byte, bool, error) {
	number, ok := value.Number()
	if !ok {
		return buf, false, errors.New("BUG: policyview: a present med with no number")
	}
	return appendFormatted(buf, attribute.AttrMED, attribute.MED(uint32(number))) //nolint:gosec // G115: the reader answers 4 octets
}

// writeLocalPreference is writeMED for local-preference.
func writeLocalPreference(buf []byte, _ *View, value Value) ([]byte, bool, error) {
	number, ok := value.Number()
	if !ok {
		return buf, false, errors.New("BUG: policyview: a present local-preference with no number")
	}
	return appendFormatted(buf, attribute.AttrLocalPref, attribute.LocalPref(uint32(number))) //nolint:gosec // G115: the reader answers 4 octets
}

// writePrefix writes the announced NLRI by family in the event's NLRI shapes.
func writePrefix(buf []byte, v *View, value Value) ([]byte, bool, error) {
	ctx, err := v.context()
	if err != nil {
		return buf, false, err
	}
	out, err := format.AppendAnnouncedNLRIJSON(buf, value.Wire(), value.MPReach(), ctx)
	if err != nil {
		return buf, false, errors.Join(ErrMalformed, err)
	}
	return out, true, nil
}

// writeFamily writes the families the UPDATE announces, as a list of family
// names: ipv4/unicast for the body NLRI, then the MP_REACH_NLRI family.
func writeFamily(buf []byte, _ *View, value Value) ([]byte, bool, error) {
	buf = append(buf, '[')
	body := value.Family()
	if body != (family.Family{}) {
		buf = appendQuotedFamily(buf, body)
	}
	if mpReach := value.MPReach(); mpReach != nil && mpReach.Family() != body {
		if body != (family.Family{}) {
			buf = append(buf, ',')
		}
		buf = appendQuotedFamily(buf, mpReach.Family())
	}
	return append(buf, ']'), true, nil
}

// writeNextHop writes the next hop of each announced family, keyed by family:
// NEXT_HOP for the body NLRI, the MP_REACH_NLRI next hop for its family. A
// NEXT_HOP beside no body NLRI states the next hop of no announced route and
// is not written.
func writeNextHop(buf []byte, v *View, value Value) ([]byte, bool, error) {
	body, mpReach, err := v.nlriSections()
	if err != nil {
		return buf, false, err
	}
	mark := len(buf)
	buf = append(buf, '{')
	var bodyHop netip.Addr
	if raw := value.Wire(); len(body) > 0 && len(raw) == 4 {
		bodyHop = netip.AddrFrom4([4]byte(raw))
		buf = appendQuotedFamily(buf, family.IPv4Unicast)
		buf = append(buf, `:"`...)
		buf = bodyHop.AppendTo(buf)
		buf = append(buf, '"')
	}
	if mpReach != nil {
		hop := mpReach.NextHop()
		switch {
		case bodyHop.IsValid() && mpReach.Family() == family.IPv4Unicast && hop != bodyHop:
			// One family key cannot state two next hops: the routes would read
			// as sharing one. The filter's on-error decides the route.
			return buf[:mark], false, errors.Join(ErrMalformed, errors.New(
				"NEXT_HOP and an IPv4 unicast MP_REACH_NLRI name different next hops"))
		case bodyHop.IsValid() && mpReach.Family() == family.IPv4Unicast:
		default:
			if bodyHop.IsValid() {
				buf = append(buf, ',')
			}
			buf = appendQuotedFamily(buf, mpReach.Family())
			buf = append(buf, `:"`...)
			buf = hop.AppendTo(buf)
			buf = append(buf, '"')
		}
	}
	if len(buf) == mark+1 {
		return buf[:mark], false, nil
	}
	return append(buf, '}'), true, nil
}

// writeProtocol writes protocol: ebgp or ibgp, the classes the policy
// library's protocol condition names for a BGP route.
func writeProtocol(buf []byte, _ *View, value Value) ([]byte, bool, error) {
	if value.EBGP() {
		return append(buf, `"ebgp"`...), true, nil
	}
	return append(buf, `"ibgp"`...), true, nil
}

// writeOriginAS writes origin-as: the AS number, or "none", the RFC 6811
// Section 2 distinguished value NONE, which is never written as zero.
func writeOriginAS(buf []byte, _ *View, value Value) ([]byte, bool, error) {
	if number, ok := value.Number(); ok {
		return strconv.AppendUint(buf, number, 10), true, nil
	}
	return append(buf, `"none"`...), true, nil
}

// appendQuotedFamily writes a family name as a JSON string.
func appendQuotedFamily(buf []byte, fam family.Family) []byte {
	buf = append(buf, '"')
	buf = fam.AppendTo(buf)
	return append(buf, '"')
}
