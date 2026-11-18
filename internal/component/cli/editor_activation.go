// Design: docs/guide/config-editor.md — deactivate and activate
// Related: model_commands_edit.go — runActivation, the SSH editor's caller
// Related: editor_commands.go — DeactivateLeaf, DeactivatePath and their activate twins

package cli

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ApplyActivation deactivates (activate false) or activates the node a token
// path names, and returns the status line every editor prints. It is the one
// dispatch the SSH and web editors share: a leaf-list value, a leaf, or a
// container or list entry, told apart by the schema. A node already in the
// requested state answers "<path> already deactivated|active" and no error.
// Not safe for concurrent use: the caller holds the editor.
func (e *Editor) ApplyActivation(fullPath []string, activate bool) (string, error) {
	verb, pastTense, alreadyState := "deactivate", "Deactivated", "deactivated"
	if activate {
		verb, pastTense, alreadyState = "activate", "Activated", "active"
	}
	if len(fullPath) == 0 {
		return "", opererr.New(verb+" needs a configuration path",
			"No node was selected. Inspect the configuration before choosing a target.", "show")
	}

	var tb textbuf.Buffer
	if parentPath, leafListName, isLeafList := e.ResolveLeafListValue(fullPath); isLeafList {
		value := fullPath[len(fullPath)-1]
		var err error
		if activate {
			err = e.ActivateLeafListValue(parentPath, leafListName, value)
		} else {
			err = e.DeactivateLeafListValue(parentPath, leafListName, value)
		}
		if err != nil {
			return "", activationRefusal(err, verb, fullPath)
		}
		return tb.Str(pastTense).Byte(' ').Str(value).Str(" in ").Str(leafListName).String(), nil
	}

	err := e.applyNodeActivation(fullPath, activate)
	if err != nil {
		if isAlreadyInState(err) {
			return tb.Join(fullPath, " ").Str(" already ").Str(alreadyState).String(), nil
		}
		return "", activationRefusal(err, verb, fullPath)
	}
	return tb.Str(pastTense).Byte(' ').Join(fullPath, " ").String(), nil
}

// ActivationError keeps an ancestor key's typed facts available to a Model
// that frames the correction for its current position. Its native refusal
// already carries the full-path activation correction and verb context.
// Only activationRefusal builds one; its zero value names the native defect.
type ActivationError struct {
	refusal *opererr.Error
	key     *configyang.ValidationError
}

func (e *ActivationError) Error() string {
	if e.refusal == nil {
		return (&opererr.Error{}).Error()
	}
	return e.refusal.Error()
}

// OperatorError is the complete activation refusal, rendered without adding
// the original key error as a second displayed cause.
func (e *ActivationError) OperatorError() *opererr.Error { return e.refusal }

// Unwrap exposes both the native rendering and the original key facts.
func (e *ActivationError) Unwrap() []error {
	var causes []error
	if e.refusal != nil {
		causes = append(causes, e.refusal)
	}
	if e.key != nil {
		causes = append(causes, e.key)
	}
	return causes
}

// activationRefusal owns the verb context once. A schema key retains its typed
// cause so a Model can frame its correction; other failures keep their whole cause.
func activationRefusal(err error, verb string, fullPath []string) error {
	var prefix textbuf.Buffer
	prefix.Str(verb).Str(" failed: ")
	if value, known := err.(*configyang.ValidationError); known && value.HoldsOperatorError() { //nolint:errorlint // Preserve this exact typed key, never the first child of a join.
		framed := FramedKeyRefusal(value, verb, nil, fullPath, nil)
		key, typed := framed.(*configyang.ValidationError) //nolint:errorlint // FramedKeyRefusal preserves this direct schema value type.
		if !typed {
			panic("BUG: FramedKeyRefusal replaced a typed activation key")
		}
		if value.LineNumber > 0 {
			prefix.Str("line ").Int(int64(value.LineNumber)).Str(": ")
		}
		native, typed := key.Unwrap().(*opererr.Error) //nolint:errorlint // HoldsOperatorError and WithFix retain the direct native refusal.
		if !typed {
			panic("BUG: framed activation key lost its native refusal")
		}
		return &ActivationError{key: value, refusal: opererr.Wrap(native.Unwrap(),
			prefix.Str(native.Problem()).String(), native.Why(), native.Fix())}
	}
	if native, known := err.(*opererr.Error); known { //nolint:errorlint // Preserve a direct complete refusal, never the first native child of a join.
		return opererr.Wrap(native.Unwrap(), prefix.Str(native.Problem()).String(), native.Why(), native.Fix())
	}
	return opererr.Wrap(err, prefix.Str("the selected node could not be changed").String(),
		"No successful change was confirmed. Inspect the configuration and choose a supported target before retrying.",
		"show")
}

// isAlreadyInState names the four refusals that mean the node already holds
// the state asked for, which the editors report as a status, not an error.
func isAlreadyInState(err error) bool {
	if errors.Is(err, ErrLeafAlreadyInactive) {
		return true
	}
	if errors.Is(err, ErrPathAlreadyInactive) {
		return true
	}
	if errors.Is(err, ErrLeafNotInactive) {
		return true
	}
	return errors.Is(err, ErrPathNotInactive)
}

// applyNodeActivation toggles the leaf, container or list entry fullPath
// names. A path the schema does not know goes to DeactivatePath or
// ActivatePath, which refuse it as "path not found". A positional list entry
// (a list whose children are all leaves, such as capability nexthop) and the
// freeform, flex and inline-list kinds carry no inactive marker, so they are
// refused rather than toggled.
func (e *Editor) applyNodeActivation(fullPath []string, activate bool) error {
	node := e.schema.LookupTokenPath(fullPath)
	if node == nil {
		return e.applyPathActivation(fullPath, activate)
	}
	var tb textbuf.Buffer
	switch node.Kind() {
	case config.NodeLeaf:
		parentPath, leafName := fullPath[:len(fullPath)-1], fullPath[len(fullPath)-1]
		if activate {
			return e.ActivateLeaf(parentPath, leafName)
		}
		return e.DeactivateLeaf(parentPath, leafName)
	case config.NodeList:
		listNode, ok := node.(*config.ListNode)
		if ok && !listNode.HasStructuralChildren() {
			return fmt.Errorf("path %q is a positional list entry; deactivate the parent container instead", tb.Join(fullPath, " ").String())
		}
		return e.applyPathActivation(fullPath, activate)
	case config.NodeContainer:
		container, ok := node.(*config.ContainerNode)
		if ok && container.Presence {
			parentPath, name := fullPath[:len(fullPath)-1], fullPath[len(fullPath)-1]
			if parent := e.WalkPath(parentPath); parent != nil && parent.GetContainer(name) == nil {
				if _, present := parent.Get(name); present {
					if activate {
						return e.ActivateLeaf(parentPath, name)
					}
					return e.DeactivateLeaf(parentPath, name)
				}
			}
		}
		return e.applyPathActivation(fullPath, activate)
	case config.NodeFreeform, config.NodeFlex, config.NodeInlineList:
		return fmt.Errorf("path %q resolves to a node type that does not support deactivation", tb.Join(fullPath, " ").String())
	}
	return fmt.Errorf("path %q resolves to an unknown node kind %d", tb.Join(fullPath, " ").String(), node.Kind())
}

// applyPathActivation toggles a container or list entry.
func (e *Editor) applyPathActivation(fullPath []string, activate bool) error {
	if activate {
		return e.ActivatePath(fullPath)
	}
	return e.DeactivatePath(fullPath)
}
