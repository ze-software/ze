// Design: docs/architecture/testing/ci-format.md -- the .ci directive key vocabulary
// Overview: expect.go -- parseOptionConfig, the dispatcher these tables drive

package peer

import (
	"errors"
	"maps"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The key and value words more than one arm names. The arm's key list and the
// parser's map read are then the same declaration, so the two cannot drift.
const (
	keyFamily = "family"
	keyPrefix = "prefix"
	keyValue  = "value"

	optOpenAddCapability  = "add-capability"
	optOpenDropCapability = "drop-capability"
)

// peerOptionArm is one option ze-peer claims: the keys its parser reads, and
// the parser. The entry is the whole of what ze-peer knows about the option, so
// a key cannot be accepted without a parser beside it, and a parser cannot read
// a key its entry does not list without every .ci that writes it going red.
type peerOptionArm struct {
	keys  []string
	parse func(config *Config, kv map[string]string) error
}

// peerOptionArms holds every option= type ze-peer claims. An option absent here
// is the runner's (file, timeout, env, ...), so ze-peer leaves it, and its keys,
// to the runner's parser, which refuses its own unknown keys.
var peerOptionArms = map[string]peerOptionArm{
	"asn":              {keys: []string{keyValue, "peer"}, parse: parsePeerOptionASN},
	"await-file":       {keys: []string{"path"}, parse: parsePeerOptionAwaitFile},
	"await_eor":        {keys: []string{keyValue}, parse: parsePeerOptionAwaitEOR},
	"bind":             {keys: []string{keyValue}, parse: parsePeerOptionBind},
	"conn_map":         {keys: []string{keyValue}, parse: parsePeerOptionConnMap},
	"established-file": {keys: []string{"path"}, parse: parsePeerOptionEstablishedFile},
	"linger":           {keys: []string{keyValue}, parse: parsePeerOptionLinger},
	"silent":           {keys: []string{keyValue}, parse: parsePeerOptionSilent},
	"tcp_connections":  {keys: []string{keyValue}, parse: parsePeerOptionTCPConnections},
}

// peerValueArms holds, for option=open and option=update, one arm per value=.
// An arm's keys are the ones it reads beside value= itself. A value absent here
// is one ze-peer has no parser for, and is refused with the values it does have.
var peerValueArms = map[string]map[string]peerOptionArm{
	"open": {
		optOpenAddCapability:      {keys: []string{"code", "hex"}, parse: parseOpenAddCapability},
		optOpenDropCapability:     {keys: []string{"code"}, parse: parseOpenDropCapability},
		"inspect-open-message":    {parse: parseOpenInspectMessage},
		"router-id":               {keys: []string{"id"}, parse: parseOpenRouterID},
		"send-unknown-capability": {parse: parseOpenSendUnknownCapability},
		"send-unknown-message":    {parse: parseOpenSendUnknownMessage},
		optOpenGracefulRestart:    {keys: []string{"restart-time", keyFamily, "forward-state"}, parse: parseOpenGracefulRestart},
		optOpenHoldTime:           {keys: []string{"seconds"}, parse: parseOpenHoldTimeOption},
		optOpenLLGR:               {keys: []string{"stale-time", keyFamily, "forward-state"}, parse: parseOpenLLGR},
		optOpenPathsLimit:         {keys: []string{"limit", keyFamily}, parse: parseOpenPathsLimit},
	},
	"update": {
		"send-bulk":          {keys: []string{keyPrefix, "count", "next-hop", "origin-as", "max-msg", "eor"}, parse: parseUpdateSendBulk},
		"send-default-route": {parse: parseUpdateSendDefaultRoute},
		"send-route": {
			keys: []string{
				keyPrefix, "origin-as", "next-hop", "as-set", "as-path", "originator-id", "cluster-list", "label",
			},
			parse: parseUpdateSendRoute,
		},
	},
}

// peerOptionArmFor answers the arm ze-peer holds for option=optType, and
// whether ze-peer claims the option at all. It refuses an option line ze-peer
// claims that carries a key its arm does not read, and an open=/update= value
// it has no arm for. An unknown key used to be dropped, so the session ze-peer
// ran was not the one the .ci described and nothing said so.
func peerOptionArmFor(optType string, kv map[string]string) (peerOptionArm, bool, error) {
	if values, ok := peerValueArms[optType]; ok {
		value := kv[keyValue]
		arm, known := values[value]
		if !known {
			var msg textbuf.Buffer
			msg.Str("option=").Str(optType).Str(":value=").Quoted(value).Str(" is not a value ze-peer knows (accepts ").
				Join(slices.Sorted(maps.Keys(values)), ", ").Byte(')')
			return peerOptionArm{}, false, errors.New(msg.String())
		}
		directive := "option=" + optType + ":value=" + value
		if err := refuseUnknownKeys(directive, kv, append([]string{keyValue}, arm.keys...)); err != nil {
			return peerOptionArm{}, false, err
		}
		return arm, true, nil
	}
	arm, ok := peerOptionArms[optType]
	if !ok {
		return peerOptionArm{}, false, nil
	}
	if err := refuseUnknownKeys("option="+optType, kv, arm.keys); err != nil {
		return peerOptionArm{}, false, err
	}
	return arm, true, nil
}

// refuseUnknownKeys names every key of kv that known does not list, and the
// keys it accepts, in the shape the runner's checkKeys uses, so an author meets
// one message on both sides of the peer boundary.
func refuseUnknownKeys(directive string, kv map[string]string, known []string) error {
	var unknown []string
	for key := range kv {
		if !slices.Contains(known, key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	slices.Sort(unknown)
	var msg textbuf.Buffer
	msg.Str(directive).Str(": unknown key")
	if len(unknown) > 1 {
		msg.Byte('s')
	}
	msg.Byte(' ')
	for i, key := range unknown {
		if i > 0 {
			msg.Str(", ")
		}
		msg.Quoted(key)
	}
	msg.Str(" (accepts ").Join(slices.Sorted(slices.Values(known)), ", ").Byte(')')
	return errors.New(msg.String())
}
