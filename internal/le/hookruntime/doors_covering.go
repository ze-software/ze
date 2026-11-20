// Design: docs/architecture/core-design.md -- the door read gate's door index, for other le tools
// Related: writeedit.go -- unreadDoors, the gate that applies the same rule

package hookruntime

import "github.com/ze-software/ze/internal/le/doc/doorindex"

// DoorsCovering answers a function that names the door pages covering a
// checkout-relative, slash-separated file: those whose door-base header names
// the longest whole-component prefix of it, every tied door included. A file
// no door reaches answers nothing.
//
// It applies the rule the door read gate applies, doorindex's, so a tool that
// groups files by door area cannot disagree with the gate about which door a
// file sits under. A page under ai/doors whose header is missing or malformed
// is an error naming the page, never a door that covers nothing. Its callers
// (internal/le/arch/errorinventory, internal/le/arch/doctorcoverage) may call
// doorindex.Load directly; this function stays until they do.
//
// The answered function only reads the doors loaded here, so it is safe for
// concurrent use.
func DoorsCovering(root string) (func(file string) []string, error) {
	index, err := doorindex.Load(root)
	if err != nil {
		return nil, err
	}
	return index.Covering, nil
}
