// Design: docs/architecture/plugin/plugin-system.md -- participant, send, and receive declarations
// Related: registry.go -- Registration fields validated by Register here
// Related: ../server/stage1_checks.go -- Stage 1 check table invokes these same checks
package registry

import (
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Bounds on bus participant and message declarations from compiled
// registrations and Stage 1 wire payloads.
const (
	maxParticipantNodes  = 64
	maxMessageDecls      = 256
	maxDeclPathLen       = 256
	maxDeclModuleNameLen = 128
	maxDeclPrefixLen     = 64
)

func validDeclToken(value string, maxLen int) bool {
	if value == "" {
		return false
	}
	if len(value) > maxLen {
		return false
	}
	for i := range len(value) {
		c := value[i]
		if c <= 0x20 {
			return false
		}
		if c == 0x7f {
			return false
		}
	}
	return true
}

func checkMessageTagDecls(pluginName, direction, msg string, tags []rpc.MessageTagDecl) error {
	if len(tags) > events.MaxMessageTags {
		return fmt.Errorf("plugin %q %s %q declares %d tags (max %d)",
			pluginName, direction, msg, len(tags), events.MaxMessageTags)
	}
	seenTags := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if !validDeclToken(tag.Name, events.MaxMessageTagNameLen) {
			return fmt.Errorf("plugin %q %s %q declares invalid tag name %q",
				pluginName, direction, msg, tag.Name)
		}
		if seenTags[tag.Name] {
			return fmt.Errorf("plugin %q %s %q declares duplicate tag %q",
				pluginName, direction, msg, tag.Name)
		}
		seenTags[tag.Name] = true
		if len(tag.Values) > events.MaxMessageTagValues {
			return fmt.Errorf("plugin %q %s %q tag %q declares %d values (max %d)",
				pluginName, direction, msg, tag.Name, len(tag.Values), events.MaxMessageTagValues)
		}
		seenValues := make(map[string]bool, len(tag.Values))
		for _, v := range tag.Values {
			if !validDeclToken(v, events.MaxMessageTagValueLen) {
				return fmt.Errorf("plugin %q %s %q tag %q declares invalid value %q",
					pluginName, direction, msg, tag.Name, v)
			}
			if seenValues[v] {
				return fmt.Errorf("plugin %q %s %q tag %q declares duplicate value %q",
					pluginName, direction, msg, tag.Name, v)
			}
			seenValues[v] = true
		}
	}
	return nil
}

// CheckParticipantNodeDecls validates ParticipantNodes declared in a compiled
// Registration or a Stage 1 DeclareRegistrationInput.
func CheckParticipantNodeDecls(pluginName string, nodes []rpc.ParticipantNodeDecl) error {
	if len(nodes) > maxParticipantNodes {
		return fmt.Errorf("plugin %q declares %d participant nodes (max %d)",
			pluginName, len(nodes), maxParticipantNodes)
	}
	seen := make(map[string]bool, len(nodes))
	for i := range nodes {
		n := &nodes[i]
		if !validDeclToken(n.SchemaPath, maxDeclPathLen) {
			return fmt.Errorf("plugin %q participant node %d has invalid schema-path %q",
				pluginName, i, n.SchemaPath)
		}
		if seen[n.SchemaPath] {
			return fmt.Errorf("plugin %q declares duplicate participant node schema-path %q",
				pluginName, n.SchemaPath)
		}
		seen[n.SchemaPath] = true
		if n.Module != "" {
			if !validDeclToken(n.Module, maxDeclModuleNameLen) {
				return fmt.Errorf("plugin %q participant node %q has invalid module %q",
					pluginName, n.SchemaPath, n.Module)
			}
		}
		if n.Prefix != "" {
			if !validDeclToken(n.Prefix, maxDeclPrefixLen) {
				return fmt.Errorf("plugin %q participant node %q has invalid prefix %q",
					pluginName, n.SchemaPath, n.Prefix)
			}
		}
		if n.SecondaryIPPath != "" {
			if !validDeclToken(n.SecondaryIPPath, maxDeclPathLen) {
				return fmt.Errorf("plugin %q participant node %q has invalid secondary-ip-path %q",
					pluginName, n.SchemaPath, n.SecondaryIPPath)
			}
		}
	}
	return nil
}

// CheckMessageSendDecls validates Sends declared in a compiled Registration or
// a Stage 1 DeclareRegistrationInput.
func CheckMessageSendDecls(pluginName string, sends []rpc.MessageSendDecl) error {
	if len(sends) > maxMessageDecls {
		return fmt.Errorf("plugin %q declares %d sends (max %d)",
			pluginName, len(sends), maxMessageDecls)
	}
	seen := make(map[string]bool, len(sends))
	for i := range sends {
		s := &sends[i]
		if _, _, ok := events.SplitMessageName(s.Message); !ok {
			return fmt.Errorf("plugin %q send %d has invalid message name %q (must be namespace:name)",
				pluginName, i, s.Message)
		}
		if seen[s.Message] {
			return fmt.Errorf("plugin %q declares duplicate send %q", pluginName, s.Message)
		}
		seen[s.Message] = true
		if err := checkMessageTagDecls(pluginName, "send", s.Message, s.Tags); err != nil {
			return err
		}
		if len(s.DefaultDestinations) > events.MaxMessageDefaultDestinations {
			return fmt.Errorf("plugin %q send %q declares %d default destinations (max %d)",
				pluginName, s.Message, len(s.DefaultDestinations), events.MaxMessageDefaultDestinations)
		}
		for _, dst := range s.DefaultDestinations {
			if !validDeclToken(dst, maxDeclPathLen) {
				return fmt.Errorf("plugin %q send %q declares invalid default destination %q",
					pluginName, s.Message, dst)
			}
		}
	}
	return nil
}

// CheckMessageReceiveDecls validates Receives declared in a compiled
// Registration or a Stage 1 DeclareRegistrationInput.
func CheckMessageReceiveDecls(pluginName string, receives []rpc.MessageReceiveDecl) error {
	if len(receives) > maxMessageDecls {
		return fmt.Errorf("plugin %q declares %d receives (max %d)",
			pluginName, len(receives), maxMessageDecls)
	}
	seen := make(map[string]bool, len(receives))
	for i := range receives {
		r := &receives[i]
		if _, _, ok := events.SplitMessageName(r.Message); !ok {
			return fmt.Errorf("plugin %q receive %d has invalid message name %q (must be namespace:name)",
				pluginName, i, r.Message)
		}
		if seen[r.Message] {
			return fmt.Errorf("plugin %q declares duplicate receive %q", pluginName, r.Message)
		}
		seen[r.Message] = true
		if len(r.From) > events.MaxMessageDefaultDestinations {
			return fmt.Errorf("plugin %q receive %q declares %d from paths (max %d)",
				pluginName, r.Message, len(r.From), events.MaxMessageDefaultDestinations)
		}
		for _, from := range r.From {
			if !validDeclToken(from, maxDeclPathLen) {
				return fmt.Errorf("plugin %q receive %q declares invalid from path %q",
					pluginName, r.Message, from)
			}
		}
		if err := checkMessageTagDecls(pluginName, "receive", r.Message, r.Tags); err != nil {
			return err
		}
	}
	return nil
}

// CheckBusDeclarationsMatch verifies that a compiled-in plugin's Stage 1
// ParticipantNodes, Sends, and Receives match its compiled Registration so
// running it as a separate process cannot drift from the compiled declaration
// (spec R-5).
func CheckBusDeclarationsMatch(pluginName string, compiled *Registration, declared *rpc.DeclareRegistrationInput) error {
	if compiled == nil {
		return nil
	}
	if !participantNodesEqual(compiled.ParticipantNodes, declared.ParticipantNodes) {
		return fmt.Errorf("plugin %q Stage 1 participant-nodes disagree with compiled registration %q",
			pluginName, compiled.Name)
	}
	if !messageSendsEqual(compiled.Sends, declared.Sends) {
		return fmt.Errorf("plugin %q Stage 1 sends disagree with compiled registration %q",
			pluginName, compiled.Name)
	}
	if !messageReceivesEqual(compiled.Receives, declared.Receives) {
		return fmt.Errorf("plugin %q Stage 1 receives disagree with compiled registration %q",
			pluginName, compiled.Name)
	}
	return nil
}

func participantNodesEqual(a, b []rpc.ParticipantNodeDecl) bool {
	return slices.Equal(a, b)
}

func messageTagsEqual(a, b []rpc.MessageTagDecl) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
		if !slices.Equal(a[i].Values, b[i].Values) {
			return false
		}
	}
	return true
}

func messageSendsEqual(a, b []rpc.MessageSendDecl) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Message != b[i].Message {
			return false
		}
		if !slices.Equal(a[i].DefaultDestinations, b[i].DefaultDestinations) {
			return false
		}
		if !messageTagsEqual(a[i].Tags, b[i].Tags) {
			return false
		}
	}
	return true
}

func messageReceivesEqual(a, b []rpc.MessageReceiveDecl) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Message != b[i].Message {
			return false
		}
		if !slices.Equal(a[i].From, b[i].From) {
			return false
		}
		if !messageTagsEqual(a[i].Tags, b[i].Tags) {
			return false
		}
	}
	return true
}
