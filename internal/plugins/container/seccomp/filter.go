// Design: docs/architecture/container/runtime.md -- the system-call filter
// Related: profile.go -- the default profile the filter is built from
// Related: table.go -- the call numbers of each architecture
// Related: load_linux.go -- the load of the assembled program

//go:build linux

package seccomp

import (
	"errors"
	"fmt"

	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
)

// The layout of struct seccomp_data (include/uapi/linux/seccomp.h), which
// the kernel hands the filter for each call: the call number, the audit
// architecture, the instruction pointer, then six 64-bit arguments.
const (
	offsetNumber       = 0
	offsetArchitecture = 4
	offsetArguments    = 16
	argumentSize       = 8
	// argumentIndexMax is the last of the six arguments.
	argumentIndexMax = 5
)

// instructionsMax is the kernel's BPF_MAXINSNS: it refuses a longer program.
const instructionsMax = 4096

// jumpMax is the longest jump a conditional instruction encodes in its
// 8-bit offset.
const jumpMax = 255

// errnoMax is the largest error number a return value carries in its
// 16-bit SECCOMP_RET_DATA field.
const errnoMax = unix.SECCOMP_RET_DATA

// returnNotImplemented answers a call the kernel does not know: ENOSYS, so a
// program falls back to an older call, where EPERM would make it fail.
const returnNotImplemented = unix.SECCOMP_RET_ERRNO | uint32(unix.ENOSYS)

// assembleFilter builds the classic BPF program that enforces profile on the
// architecture of table, in the layout below; the first matching rule
// decides.
//
//	ld  [arch]                 a call under another architecture
//	jeq audit, 1, 0            (a 32-bit ABI) is killed with its process
//	ret KILL_PROCESS
//	ld  [nr]                   a number above the largest the profile names,
//	jgt largest, 0, 1          the x32 range (bit 30) included, answers
//	ret ERRNO(ENOSYS)          "not implemented"
//	<one block per rule>       jeq nr; argument checks; ret action
//	ret default
//
// A rule whose name the table lacks, or whose action is the default, adds no
// block. Each argument is compared as a 64-bit value on its two 32-bit
// words, low word first in memory: both shipped architectures are
// little-endian.
//
// It refuses what it cannot enforce exactly: a flag, a listener or notify
// action, another architecture, an argument index above 5, an unknown
// operator or action, an error number above 16 bits, one call with rules of
// two different actions (the kernel's libseccomp does not order rules, so
// first-match is exact only when they agree), a jump above 255 and a
// program longer than the kernel accepts.
func assembleFilter(profile *configs.Seccomp, table *syscallTable) ([]bpf.RawInstruction, error) {
	if err := checkProfile(profile, table); err != nil {
		return nil, err
	}
	returnDefault, err := actionReturn(profile.DefaultAction, profile.DefaultErrnoRet)
	if err != nil {
		return nil, fmt.Errorf("container system-call filter: the default action: %w", err)
	}
	program := []bpf.Instruction{
		bpf.LoadAbsolute{Off: offsetArchitecture, Size: 4},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: table.audit, SkipTrue: 1},
		bpf.RetConstant{Val: unix.SECCOMP_RET_KILL_PROCESS},
		loadNumber,
	}
	if largest, known := largestNumber(profile, table); known {
		program = append(program,
			bpf.JumpIf{Cond: bpf.JumpGreaterThan, Val: largest, SkipFalse: 1},
			bpf.RetConstant{Val: returnNotImplemented},
		)
	}
	rules, err := ruleBlocks(profile, table, returnDefault)
	if err != nil {
		return nil, err
	}
	program = append(program, rules...)
	program = append(program, bpf.RetConstant{Val: returnDefault})
	if len(program) > instructionsMax {
		return nil, fmt.Errorf("container system-call filter: the program holds %d instructions, above the kernel's %d", len(program), instructionsMax)
	}
	return bpf.Assemble(program)
}

// loadNumber loads the call number into the accumulator.
var loadNumber = bpf.LoadAbsolute{Off: offsetNumber, Size: 4}

// checkProfile refuses a profile whose settings the filter cannot enforce.
func checkProfile(profile *configs.Seccomp, table *syscallTable) error {
	if profile == nil {
		return errors.New("container system-call filter: no profile")
	}
	if len(profile.Flags) > 0 {
		return fmt.Errorf("container system-call filter: the profile sets the flags %v, which ze does not apply", profile.Flags)
	}
	if profile.ListenerPath != "" {
		return errors.New("container system-call filter: the profile names a notify listener, which ze does not run")
	}
	for _, architecture := range profile.Architectures {
		if architecture != table.goarch {
			return fmt.Errorf("container system-call filter: the profile lists the architecture %q, and ze filters only the native %q", architecture, table.goarch)
		}
	}
	return nil
}

// largestNumber answers the largest number of a call the profile names and
// the table knows, a rule that adds no block included: upstream counts every
// call the profile names. It answers false when the table knows none, and
// the filter then answers no number with "not implemented".
func largestNumber(profile *configs.Seccomp, table *syscallTable) (uint32, bool) {
	var largest uint32
	known := false
	for _, call := range profile.Syscalls {
		number, found := table.numbers[call.Name]
		if !found {
			continue
		}
		if !known {
			largest = number
			known = true
			continue
		}
		if number > largest {
			largest = number
		}
	}
	return largest, known
}

// ruleBlocks answers the block of every rule that adds one, in profile order.
func ruleBlocks(profile *configs.Seccomp, table *syscallTable, returnDefault uint32) ([]bpf.Instruction, error) {
	var program []bpf.Instruction
	returnOf := make(map[string]uint32, len(profile.Syscalls))
	argumentLoaded := false
	for _, call := range profile.Syscalls {
		returnCall, err := actionReturn(call.Action, call.ErrnoRet)
		if err != nil {
			return nil, fmt.Errorf("container system-call filter: the rule for %q: %w", call.Name, err)
		}
		if earlier, seen := returnOf[call.Name]; seen {
			if earlier != returnCall {
				return nil, fmt.Errorf("container system-call filter: %q has rules of two different actions", call.Name)
			}
		}
		returnOf[call.Name] = returnCall
		number, known := table.numbers[call.Name]
		if !known {
			continue
		}
		if returnCall == returnDefault {
			continue
		}
		block, err := ruleBlock(number, call.Args, returnCall)
		if err != nil {
			return nil, fmt.Errorf("container system-call filter: the rule for %q: %w", call.Name, err)
		}
		// A failed argument check of the block before left an argument in
		// the accumulator: load the number again.
		if argumentLoaded {
			program = append(program, loadNumber)
		}
		program = append(program, block...)
		argumentLoaded = len(call.Args) > 0
	}
	return program, nil
}

// failBranch is the branch of a conditional step that leaves the block, to
// the first instruction after its return.
type failBranch int

const (
	failNone failBranch = iota
	failOnTrue
	failOnFalse
)

// blockStep is one instruction of a rule block, and the branch of it that
// fails the rule.
type blockStep struct {
	instruction bpf.Instruction
	fail        failBranch
}

// ruleBlock is the block of one rule: the number check, each argument check,
// then the return of the rule's action. Every failing branch jumps to the
// instruction after the return.
func ruleBlock(number uint32, arguments []*configs.Arg, returnCall uint32) ([]bpf.Instruction, error) {
	steps := []blockStep{failUnless(bpf.JumpEqual, number)}
	for _, argument := range arguments {
		checks, err := argumentSteps(argument)
		if err != nil {
			return nil, err
		}
		steps = append(steps, checks...)
	}
	steps = append(steps, blockStep{instruction: bpf.RetConstant{Val: returnCall}})
	block := make([]bpf.Instruction, len(steps))
	for i, step := range steps {
		instruction, err := resolveFail(step, len(steps)-i-1)
		if err != nil {
			return nil, err
		}
		block[i] = instruction
	}
	return block, nil
}

// resolveFail sets the failing branch of step to skip the skip instructions
// that follow it in its block.
func resolveFail(step blockStep, skip int) (bpf.Instruction, error) {
	if step.fail == failNone {
		return step.instruction, nil
	}
	if skip > jumpMax {
		return nil, fmt.Errorf("a jump of %d instructions, above the %d a filter encodes", skip, jumpMax)
	}
	jump, isJump := step.instruction.(bpf.JumpIf)
	if !isJump {
		panic("BUG: a failing branch on an instruction that does not jump")
	}
	if step.fail == failOnTrue {
		jump.SkipTrue = uint8(skip) //nolint:gosec // skip is at most jumpMax, checked above
		return jump, nil
	}
	jump.SkipFalse = uint8(skip) //nolint:gosec // skip is at most jumpMax, checked above
	return jump, nil
}

// failUnless is a step that goes on when the accumulator compares true to
// value, and fails the rule otherwise.
func failUnless(condition bpf.JumpTest, value uint32) blockStep {
	return blockStep{instruction: bpf.JumpIf{Cond: condition, Val: value}, fail: failOnFalse}
}

// failIf is a step that fails the rule when the accumulator compares true to
// value, and goes on otherwise.
func failIf(condition bpf.JumpTest, value uint32) blockStep {
	return blockStep{instruction: bpf.JumpIf{Cond: condition, Val: value}, fail: failOnTrue}
}

// passIf is a step that skips skip instructions when the accumulator
// compares true to value, and goes on otherwise.
func passIf(condition bpf.JumpTest, value uint32, skip uint8) blockStep {
	return blockStep{instruction: bpf.JumpIf{Cond: condition, Val: value, SkipTrue: skip}}
}

// load is a step that loads the 32-bit word at offset.
func load(offset uint32) blockStep {
	return blockStep{instruction: bpf.LoadAbsolute{Off: offset, Size: 4}}
}

// words splits a 64-bit value into its high and low 32-bit words.
func words(value uint64) (uint32, uint32) {
	return uint32(value >> 32), uint32(value) //nolint:gosec // each half is 32 bits by construction
}

// argumentSteps compares one argument, unsigned on 64 bits: the high words
// decide unless they are equal, and the low words decide then. Each sequence
// falls through to the next check when the comparison holds.
func argumentSteps(argument *configs.Arg) ([]blockStep, error) {
	if argument.Index > argumentIndexMax {
		return nil, fmt.Errorf("argument index %d, and a call has six arguments, 0 to %d", argument.Index, argumentIndexMax)
	}
	low := uint32(offsetArguments + argumentSize*argument.Index) //nolint:gosec // the index is at most argumentIndexMax, checked above
	high := low + 4
	valueHigh, valueLow := words(argument.Value)
	switch argument.Op {
	case configs.EqualTo:
		return []blockStep{
			load(high), failUnless(bpf.JumpEqual, valueHigh),
			load(low), failUnless(bpf.JumpEqual, valueLow),
		}, nil
	case configs.NotEqualTo:
		return []blockStep{
			load(high), {instruction: bpf.JumpIf{Cond: bpf.JumpEqual, Val: valueHigh, SkipFalse: 2}},
			load(low), failIf(bpf.JumpEqual, valueLow),
		}, nil
	case configs.GreaterThan:
		return orderSteps(high, low, valueHigh, valueLow, bpf.JumpGreaterThan, bpf.JumpGreaterThan), nil
	case configs.GreaterThanOrEqualTo:
		return orderSteps(high, low, valueHigh, valueLow, bpf.JumpGreaterThan, bpf.JumpGreaterOrEqual), nil
	case configs.LessThan:
		return orderSteps(high, low, valueHigh, valueLow, bpf.JumpLessThan, bpf.JumpLessThan), nil
	case configs.LessThanOrEqualTo:
		return orderSteps(high, low, valueHigh, valueLow, bpf.JumpLessThan, bpf.JumpLessOrEqual), nil
	case configs.MaskEqualTo:
		// Value is the mask and ValueTwo the masked value it must equal.
		wantHigh, wantLow := words(argument.ValueTwo)
		return []blockStep{
			load(high), {instruction: bpf.ALUOpConstant{Op: bpf.ALUOpAnd, Val: valueHigh}}, failUnless(bpf.JumpEqual, wantHigh),
			load(low), {instruction: bpf.ALUOpConstant{Op: bpf.ALUOpAnd, Val: valueLow}}, failUnless(bpf.JumpEqual, wantLow),
		}, nil
	default:
		// The set is open: a profile is a value the runtime library also
		// decodes from JSON.
		return nil, fmt.Errorf("unknown comparison operator %d", argument.Op)
	}
}

// orderSteps compares for an order: a high word strictly in order passes, an
// equal high word lets the low word decide under lowTest, and any other high
// word fails.
func orderSteps(high, low, valueHigh, valueLow uint32, highTest, lowTest bpf.JumpTest) []blockStep {
	return []blockStep{
		load(high), passIf(highTest, valueHigh, 3), failUnless(bpf.JumpEqual, valueHigh),
		load(low), failUnless(lowTest, valueLow),
	}
}

// actionReturn is the filter's return value for action, as the runtime
// library maps its actions: Kill is KILL_THREAD, and Errno and Trace carry
// errnoRet, or EPERM when it is nil.
func actionReturn(action configs.Action, errnoRet *uint) (uint32, error) {
	switch action {
	case configs.Kill, configs.KillThread:
		return unix.SECCOMP_RET_KILL_THREAD, nil
	case configs.KillProcess:
		return unix.SECCOMP_RET_KILL_PROCESS, nil
	case configs.Errno:
		data, err := errnoData(errnoRet)
		return unix.SECCOMP_RET_ERRNO | data, err
	case configs.Trace:
		data, err := errnoData(errnoRet)
		return unix.SECCOMP_RET_TRACE | data, err
	case configs.Trap:
		return unix.SECCOMP_RET_TRAP, nil
	case configs.Allow:
		return unix.SECCOMP_RET_ALLOW, nil
	case configs.Log:
		return unix.SECCOMP_RET_LOG, nil
	case configs.Notify:
		return 0, errors.New("the notify action, which needs a listener ze does not run")
	default:
		// The set is open: a profile is a value the runtime library also
		// decodes from JSON.
		return 0, fmt.Errorf("unknown action %d", action)
	}
}

// errnoData is the error number a return carries: errnoRet, or EPERM when it
// is nil.
func errnoData(errnoRet *uint) (uint32, error) {
	if errnoRet == nil {
		return uint32(unix.EPERM), nil
	}
	if *errnoRet > errnoMax {
		return 0, fmt.Errorf("error number %d, above the %d a return carries", *errnoRet, errnoMax)
	}
	return uint32(*errnoRet), nil //nolint:gosec // at most errnoMax, checked above
}
