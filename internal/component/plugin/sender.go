// Design: docs/architecture/api/architecture.md -- who issued a command
// Related: server/command.go -- CommandContext.Sender, set by every dispatch path
// Related: ../bgp/reactor/send_permission.go -- the guard that reads it

package plugin

import "github.com/ze-software/ze/internal/core/opererr"

// Sender says who issued a command that puts a message on a peer's wire: one
// attached process, or the operator.
//
// The zero value is neither, and that is deliberate. An operator command
// carries the operator's own authority, checked by AAA at dispatch, while a
// process command carries only what a peer's `bus` delivery rules grant it. A guard that read "no process" as "the operator" would give that
// operator authority to any dispatch path that forgot to say who it was, so a
// guard MUST refuse a Sender whose IsSet reports false (ai/rules/evidence.md).
//
// The private who field carries all three issuer states. The two private
// destination fields only narrow a request; they never change its issuer.
// Constructors establish both-or-none destination validity. The zero value
// still names nobody and cannot acquire operator authority by omission.
type Sender struct {
	who                string
	destinationOwner   uint64
	destinationSession uint64
}

// operatorName is the reserved name a Sender holds for the operator. It carries
// the NUL byte for the same reason aaa.ReservedInternalPrefix does: a config
// line is tokenized on whitespace, so no configured name can hold one. The
// guarantee does not rest on that: ProcessSender refuses this exact name, so no
// process presents itself as the operator whatever a parser accepts.
const operatorName = "\x00operator"

// OperatorSender is the sender of a command an operator typed at the CLI, the
// SSH surface, the REST API, or any other operator surface.
func OperatorSender() Sender {
	return Sender{who: operatorName}
}

// ProcessSender is the sender of a command the named attached process issued.
//
// An empty name, or the reserved operator name, yields the zero Sender. A
// process the caller cannot name is the omission this type exists to catch, and
// a guard refuses the result rather than reading it as the operator.
func ProcessSender(name string) Sender {
	if name == "" || name == operatorName {
		return Sender{}
	}
	return Sender{who: name}
}

// IsSet reports whether anything said who issued the command.
func (s Sender) IsSet() bool {
	return s.who != ""
}

// IsOperator reports whether an operator issued the command.
func (s Sender) IsOperator() bool {
	return s.who == operatorName
}

// Process returns the name of the attached process that issued the command. The
// second result is false when the operator issued it, and when nothing said who
// did, so a caller that needs a process name handles both misses in one branch.
func (s Sender) Process() (string, bool) {
	if s.who == "" || s.who == operatorName {
		return "", false
	}
	return s.who, true
}

// String names the sender for a log line, an error, or a metric label. The
// values are bounded: a process name from the config, "operator", or "unset".
func (s Sender) String() string {
	switch s.who {
	case "":
		return "unset"
	case operatorName:
		return "operator"
	}
	return s.who
}

// WithDestination narrows an existing issuer to one original destination.
// Both zero retains the explicit unbounded command contract. Every other
// successful value holds both nonzero identifiers; neither grants permission.
func (s Sender) WithDestination(owner, session uint64) (Sender, error) {
	if owner == 0 {
		if session == 0 {
			s.destinationOwner = 0
			s.destinationSession = 0
			return s, nil
		}
		return Sender{}, destinationConstraintRefused()
	}
	if session == 0 {
		return Sender{}, destinationConstraintRefused()
	}
	if !s.IsSet() {
		return Sender{}, destinationConstraintRefused()
	}
	s.destinationOwner = owner
	s.destinationSession = session
	return s, nil
}

func destinationConstraintRefused() error {
	return opererr.New("Destination constraint is incomplete",
		"No destination was selected and the command grants no send permission.",
		"Supply both destination-owner and destination-session as nonzero uint64 values, or omit both for an explicit unbounded command.")
}

// Destination returns the immutable narrowing tuple. Both zero is the existing
// unbounded operation. A nonzero tuple still needs the issuer's ordinary grant.
func (s Sender) Destination() (owner, session uint64) {
	return s.destinationOwner, s.destinationSession
}

// SameIssuer compares authenticated identity only. A destination restriction
// cannot create another producer or change an accepted entry's original owner.
// It does not compare Process incarnations; SelectedOwner owns that contract.
func (s Sender) SameIssuer(other Sender) bool { return s.who == other.who }
