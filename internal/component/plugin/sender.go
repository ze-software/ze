// Design: docs/architecture/api/architecture.md -- who issued a command
// Related: server/command.go -- CommandContext.Sender, set by every dispatch path
// Related: ../bgp/reactor/send_permission.go -- the guard that reads it

package plugin

// Sender says who issued a command that puts a message on a peer's wire: one
// attached process, or the operator.
//
// The zero value is neither, and that is deliberate. An operator command
// carries the operator's own authority, checked by AAA at dispatch, while a
// process command carries only what a peer's `attach process <name>` block
// grants it. A guard that read "no process" as "the operator" would give that
// operator authority to any dispatch path that forgot to say who it was, so a
// guard MUST refuse a Sender whose IsSet reports false (ai/rules/evidence.md).
//
// One field carries all three states, so no two fields can disagree about who
// sent the command. The field is unexported and the two constructors below are
// the only way to fill it, so the zero value is the one Sender a caller can
// build without naming a state.
type Sender struct {
	who string
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
