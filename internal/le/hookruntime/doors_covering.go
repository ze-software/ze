// Design: docs/architecture/core-design.md -- the door read gate's door index, for other le tools
// Related: readgate.go -- loadDoors, coveringDoors: the rule this file exposes

package hookruntime

// DoorsCovering answers a function that names the door pages covering a
// checkout-relative, slash-separated file: those whose door-base header names
// the longest whole-component prefix of it, every tied door included. A file
// no door reaches answers nothing.
//
// It applies the same rule the door read gate applies, through the same
// functions, so a tool that groups files by door area cannot disagree with
// the gate about which door a file sits under. A page under ai/doors whose
// header is missing or malformed is an error naming the page, never a door
// that covers nothing.
//
// The answered function only reads the doors loaded here, so it is safe for
// concurrent use.
func DoorsCovering(root string) (func(file string) []string, error) {
	doors, err := loadDoors(root)
	if err != nil {
		return nil, err
	}
	return func(file string) []string { return coveringDoors(doors, file) }, nil
}
