// Design: docs/architecture/container/runtime.md -- the system-call filter

//go:build linux

package seccomp

import (
	"encoding/binary"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
)

// The return values the tests expect, spelled from the kernel's constants.
const (
	returnAllow       = unix.SECCOMP_RET_ALLOW
	returnPermission  = unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM)
	returnKillProcess = unix.SECCOMP_RET_KILL_PROCESS
)

// x32Bit is the bit an x32 call sets in its number under the x86-64 audit
// architecture.
const x32Bit = 0x40000000

// errnoOf is a pointer to an error number, as a profile holds one.
func errnoOf(errno uint) *uint {
	return new(errno)
}

// eperm is a profile whose default is EPERM and whose rules are calls.
func eperm(calls ...*configs.Syscall) *configs.Seccomp {
	return &configs.Seccomp{DefaultAction: configs.Errno, DefaultErrnoRet: errnoOf(uint(unix.EPERM)), Syscalls: calls}
}

// assemble builds the filter of profile on table and fails the test on a
// refusal.
func assemble(t *testing.T, profile *configs.Seccomp, table *syscallTable) []bpf.RawInstruction {
	t.Helper()
	program, err := assembleFilter(profile, table)
	if err != nil {
		t.Fatalf("assembleFilter refused the profile: %v", err)
	}
	return program
}

// runFilter runs program in the bpf package's virtual machine on the
// seccomp_data of a call, and answers the filter's return value.
//
// The kernel lays seccomp_data out in native order, little-endian on both
// shipped architectures, and the virtual machine reads every word
// big-endian. So each 32-bit field is written big-endian at the offset the
// kernel uses, and an argument's low word at 16+8i, its high word at 20+8i:
// the program then reads the values the kernel would hand it.
func runFilter(t *testing.T, program []bpf.RawInstruction, audit, number uint32, arguments ...uint64) uint32 {
	t.Helper()
	instructions, decoded := bpf.Disassemble(program)
	if !decoded {
		t.Fatal("the assembled program holds an instruction the bpf package cannot decode")
	}
	vm, err := bpf.NewVM(instructions)
	if err != nil {
		t.Fatalf("the virtual machine refused the program: %v", err)
	}
	data := make([]byte, 64)
	binary.BigEndian.PutUint32(data[offsetNumber:], number)
	binary.BigEndian.PutUint32(data[offsetArchitecture:], audit)
	for i, argument := range arguments {
		high, low := words(argument)
		binary.BigEndian.PutUint32(data[offsetArguments+argumentSize*i:], low)
		binary.BigEndian.PutUint32(data[offsetArguments+argumentSize*i+4:], high)
	}
	answer, err := vm.Run(data)
	if err != nil {
		t.Fatalf("the virtual machine failed: %v", err)
	}
	return uint32(answer) //nolint:gosec // the program's return is a 32-bit value
}

// call runs program on table's number of name.
func call(t *testing.T, program []bpf.RawInstruction, table *syscallTable, name string, arguments ...uint64) uint32 {
	t.Helper()
	number, known := table.numbers[name]
	if !known {
		t.Fatalf("the %s table holds no %q", table.goarch, name)
	}
	return runFilter(t, program, table.audit, number, arguments...)
}

// TestFilterReturnsEachActionForItsCall proves each action of the runtime
// library reaches the kernel as its return value, with its error number.
//
// Method: one listed call per action, run in the virtual machine.
//
// MUTATION: map Kill to KILL_PROCESS, or drop the error number of Trace; the
// row of that action fails.
func TestFilterReturnsEachActionForItsCall(t *testing.T) {
	rows := []struct {
		name   string
		action configs.Action
		errno  *uint
		want   uint32
	}{
		{"read", configs.Allow, nil, unix.SECCOMP_RET_ALLOW},
		{"write", configs.Errno, errnoOf(13), unix.SECCOMP_RET_ERRNO | 13},
		{"mmap", configs.Errno, nil, unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM)},
		{"open", configs.Trap, nil, unix.SECCOMP_RET_TRAP},
		{"close", configs.Trace, errnoOf(38), unix.SECCOMP_RET_TRACE | 38},
		{"stat", configs.Log, nil, unix.SECCOMP_RET_LOG},
		{"fstat", configs.Kill, nil, unix.SECCOMP_RET_KILL_THREAD},
		{"lstat", configs.KillThread, nil, unix.SECCOMP_RET_KILL_THREAD},
		{"poll", configs.KillProcess, nil, unix.SECCOMP_RET_KILL_PROCESS},
	}
	profile := &configs.Seccomp{DefaultAction: configs.Allow}
	for _, row := range rows {
		profile.Syscalls = append(profile.Syscalls, &configs.Syscall{Name: row.name, Action: row.action, ErrnoRet: row.errno})
	}
	// The default is Allow here, so the Allow row adds no block; a second
	// profile with an EPERM default proves the Allow return itself.
	program := assemble(t, profile, &amd64Table)
	for _, row := range rows {
		if got := call(t, program, &amd64Table, row.name); got != row.want {
			t.Errorf("%s with action %d answered %#x, want %#x", row.name, row.action, got, row.want)
		}
	}
	allowing := assemble(t, eperm(&configs.Syscall{Name: "read", Action: configs.Allow}), &arm64Table)
	if got := call(t, allowing, &arm64Table, "read"); got != returnAllow {
		t.Errorf("an allowed read answered %#x, want ALLOW", got)
	}
}

// TestFilterReturnsTheDefaultForAnUnlistedCall proves a call no rule names,
// below the largest number, gets the default action with its error number.
func TestFilterReturnsTheDefaultForAnUnlistedCall(t *testing.T) {
	listed := []*configs.Syscall{{Name: "read", Action: configs.Allow}, {Name: "clone3", Action: configs.Allow}}

	program := assemble(t, eperm(listed...), &amd64Table)
	if got := call(t, program, &amd64Table, "unshare"); got != returnPermission {
		t.Errorf("unlisted unshare answered %#x, want ERRNO(EPERM)", got)
	}

	other := &configs.Seccomp{DefaultAction: configs.Errno, DefaultErrnoRet: errnoOf(13), Syscalls: listed}
	program = assemble(t, other, &amd64Table)
	if got := call(t, program, &amd64Table, "unshare"); got != unix.SECCOMP_RET_ERRNO|13 {
		t.Errorf("unlisted unshare under an EACCES default answered %#x, want ERRNO(13)", got)
	}

	// A default with no error number answers EPERM, as the runtime library does.
	bare := &configs.Seccomp{DefaultAction: configs.Errno, Syscalls: listed}
	program = assemble(t, bare, &amd64Table)
	if got := call(t, program, &amd64Table, "unshare"); got != returnPermission {
		t.Errorf("unlisted unshare under a bare Errno default answered %#x, want ERRNO(EPERM)", got)
	}
}

// TestFilterReturnsNotImplementedAboveTheProfile proves a number above the
// largest the profile names, the x32 range included, answers ENOSYS, and the
// largest itself does not.
//
// MUTATION: drop the ENOSYS check; 436 then answers the default EPERM.
func TestFilterReturnsNotImplementedAboveTheProfile(t *testing.T) {
	program := assemble(t, eperm(
		&configs.Syscall{Name: "read", Action: configs.Allow},
		&configs.Syscall{Name: "clone3", Action: configs.Allow},
	), &amd64Table)
	clone3 := amd64Table.numbers["clone3"]

	if got := runFilter(t, program, amd64Table.audit, clone3); got != returnAllow {
		t.Errorf("clone3, the largest listed number, answered %#x, want ALLOW", got)
	}
	if got := runFilter(t, program, amd64Table.audit, clone3+1); got != returnNotImplemented {
		t.Errorf("number %d answered %#x, want ERRNO(ENOSYS)", clone3+1, got)
	}
	if got := runFilter(t, program, amd64Table.audit, x32Bit|amd64Table.numbers["read"]); got != returnNotImplemented {
		t.Errorf("the x32 read answered %#x, want ERRNO(ENOSYS)", got)
	}
}

// TestFilterKillsAnotherArchitecture proves a call under an audit
// architecture other than the table's kills the process, whatever its
// number.
func TestFilterKillsAnotherArchitecture(t *testing.T) {
	profile := eperm(&configs.Syscall{Name: "read", Action: configs.Allow})
	for _, row := range []struct {
		table *syscallTable
		audit uint32
	}{
		{&amd64Table, unix.AUDIT_ARCH_I386},
		{&amd64Table, unix.AUDIT_ARCH_AARCH64},
		{&arm64Table, unix.AUDIT_ARCH_ARM},
		{&arm64Table, unix.AUDIT_ARCH_X86_64},
	} {
		program := assemble(t, profile, row.table)
		if got := runFilter(t, program, row.audit, 0); got != returnKillProcess {
			t.Errorf("on the %s table, a call under audit %#x answered %#x, want KILL_PROCESS", row.table.goarch, row.audit, got)
		}
	}
}

// TestFilterComparesArguments proves each operator compares the whole 64-bit
// argument: each row holds values that differ from the rule's in the high
// word alone or the low word alone.
//
// Method: one rule allowing read on its argument, the default EPERM.
//
// MUTATION: compare the low word only, or swap the two words; a row whose
// values differ in the high word fails.
func TestFilterComparesArguments(t *testing.T) {
	const value = 0x00000001_00000005
	rows := []struct {
		op      configs.Operator
		allowed []uint64
		refused []uint64
	}{
		{configs.EqualTo, []uint64{value}, []uint64{0x2_00000005, 0x1_00000006}},
		{configs.NotEqualTo, []uint64{0x2_00000005, 0x1_00000006}, []uint64{value}},
		{configs.GreaterThan, []uint64{0x1_00000006, 0x2_00000000}, []uint64{value, 0x0_ffffffff}},
		{configs.GreaterThanOrEqualTo, []uint64{value, 0x2_00000000}, []uint64{0x1_00000004, 0x0_ffffffff}},
		{configs.LessThan, []uint64{0x1_00000004, 0x0_ffffffff}, []uint64{value, 0x2_00000000}},
		{configs.LessThanOrEqualTo, []uint64{value, 0x0_ffffffff}, []uint64{0x1_00000006, 0x2_00000000}},
	}
	for _, row := range rows {
		for _, index := range []uint{0, argumentIndexMax} {
			program := assemble(t, eperm(&configs.Syscall{
				Name: "read", Action: configs.Allow,
				Args: []*configs.Arg{{Index: index, Value: value, Op: row.op}},
			}), &amd64Table)
			for _, argument := range row.allowed {
				if got := call(t, program, &amd64Table, "read", argumentAt(index, argument)...); got != returnAllow {
					t.Errorf("operator %d, argument %d = %#x answered %#x, want ALLOW", row.op, index, argument, got)
				}
			}
			for _, argument := range row.refused {
				if got := call(t, program, &amd64Table, "read", argumentAt(index, argument)...); got != returnPermission {
					t.Errorf("operator %d, argument %d = %#x answered %#x, want ERRNO(EPERM)", row.op, index, argument, got)
				}
			}
		}
	}

	// A masked compare: (argument & Value) == ValueTwo on both words.
	program := assemble(t, eperm(&configs.Syscall{
		Name: "read", Action: configs.Allow,
		Args: []*configs.Arg{{Index: 1, Value: 0xff000000_000000ff, ValueTwo: 0x12000000_00000034, Op: configs.MaskEqualTo}},
	}), &amd64Table)
	for argument, want := range map[uint64]uint32{
		0x12345678_9abcde34: returnAllow,
		0x13000000_00000034: returnPermission,
		0x12000000_00000035: returnPermission,
	} {
		if got := call(t, program, &amd64Table, "read", 0, argument); got != want {
			t.Errorf("masked argument %#x answered %#x, want %#x", argument, got, want)
		}
	}

	// Two arguments in one rule must both hold, and a rule after a failed
	// argument check still matches its own call: the number is loaded again.
	program = assemble(t, eperm(
		&configs.Syscall{Name: "read", Action: configs.Allow, Args: []*configs.Arg{
			{Index: 0, Value: 7, Op: configs.EqualTo},
			{Index: 2, Value: 9, Op: configs.EqualTo},
		}},
		&configs.Syscall{Name: "write", Action: configs.Allow},
	), &amd64Table)
	if got := call(t, program, &amd64Table, "read", 7, 0, 9); got != returnAllow {
		t.Errorf("read(7, 0, 9) answered %#x, want ALLOW", got)
	}
	if got := call(t, program, &amd64Table, "read", 7, 0, 8); got != returnPermission {
		t.Errorf("read(7, 0, 8) answered %#x, want ERRNO(EPERM)", got)
	}
	if got := call(t, program, &amd64Table, "write", 7, 0, 8); got != returnAllow {
		t.Errorf("write after a failed read argument answered %#x, want ALLOW", got)
	}
}

// argumentAt is an argument list whose argument index is value and whose
// others are zero.
func argumentAt(index uint, value uint64) []uint64 {
	arguments := make([]uint64, index+1)
	arguments[index] = value
	return arguments
}

// TestFilterSkipsUnknownNamesAndDefaultRules proves a name the table lacks
// and a rule whose action is the default add no instruction, and leave the
// answer of the default.
func TestFilterSkipsUnknownNamesAndDefaultRules(t *testing.T) {
	base := assemble(t, eperm(
		&configs.Syscall{Name: "read", Action: configs.Allow},
		&configs.Syscall{Name: "write", Action: configs.Allow},
	), &arm64Table)
	padded := assemble(t, eperm(
		&configs.Syscall{Name: "read", Action: configs.Allow},
		// arch_prctl exists on x86-64 only.
		&configs.Syscall{Name: "arch_prctl", Action: configs.Allow},
		&configs.Syscall{Name: "no_such_call", Action: configs.Allow},
		&configs.Syscall{Name: "mmap", Action: configs.Errno, ErrnoRet: errnoOf(uint(unix.EPERM))},
		&configs.Syscall{Name: "write", Action: configs.Allow},
	), &arm64Table)

	if len(padded) != len(base) {
		t.Errorf("the padded program holds %d instructions, want the base program's %d", len(padded), len(base))
	}
	if got := call(t, padded, &arm64Table, "mmap"); got != returnPermission {
		t.Errorf("mmap answered %#x, want the default ERRNO(EPERM)", got)
	}
}

// TestFilterRefusesWhatItCannotEnforce proves each setting the filter cannot
// enforce exactly is refused, rather than built into a weaker filter.
func TestFilterRefusesWhatItCannotEnforce(t *testing.T) {
	many := eperm()
	for range instructionsMax / 2 {
		many.Syscalls = append(many.Syscalls, &configs.Syscall{Name: "read", Action: configs.Allow})
	}
	// Four instructions per argument: seventy put the rule's first check more
	// than 255 instructions before the block's end.
	longArguments := make([]*configs.Arg, 0, 70)
	for range 70 {
		longArguments = append(longArguments, &configs.Arg{Index: 0, Value: 1, Op: configs.EqualTo})
	}
	rows := map[string]*configs.Seccomp{
		"no profile":                    nil,
		"a flag":                        {DefaultAction: configs.Errno, Flags: []specs.LinuxSeccompFlag{specs.LinuxSeccompFlagLog}},
		"a notify listener":             {DefaultAction: configs.Errno, ListenerPath: "/run/listener"},
		"another architecture":          {DefaultAction: configs.Errno, Architectures: []string{"x86"}},
		"an unknown default":            {DefaultAction: configs.Action(99)},
		"a notify action":               eperm(&configs.Syscall{Name: "read", Action: configs.Notify}),
		"an unknown action":             eperm(&configs.Syscall{Name: "read", Action: configs.Action(99)}),
		"an argument index of 6":        eperm(&configs.Syscall{Name: "read", Action: configs.Allow, Args: []*configs.Arg{{Index: 6, Op: configs.EqualTo}}}),
		"an unknown operator":           eperm(&configs.Syscall{Name: "read", Action: configs.Allow, Args: []*configs.Arg{{Index: 0, Op: configs.Operator(99)}}}),
		"an error number above 16 bits": eperm(&configs.Syscall{Name: "read", Action: configs.Errno, ErrnoRet: errnoOf(errnoMax + 1)}),
		"two actions for one call": eperm(
			&configs.Syscall{Name: "read", Action: configs.Allow},
			&configs.Syscall{Name: "read", Action: configs.Log},
		),
		"a jump above 255":               eperm(&configs.Syscall{Name: "read", Action: configs.Allow, Args: longArguments}),
		"more than the kernel's maximum": many,
	}
	for reason, profile := range rows {
		if _, err := assembleFilter(profile, &amd64Table); err == nil {
			t.Errorf("a profile with %s was assembled, want a refusal", reason)
		}
	}
	if _, err := tableFor("riscv64"); err == nil {
		t.Error("tableFor answered a table for riscv64, which ze holds none for")
	}
}
