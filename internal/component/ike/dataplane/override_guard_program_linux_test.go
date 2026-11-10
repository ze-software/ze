//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the application override guard
// Related: override_guard_linux.go -- overrideGuardInstructions, kernelDev

package dataplane

import (
	"encoding/binary"
	"testing"

	"github.com/cilium/ebpf/asm"
	"golang.org/x/sys/unix"
)

// The simulated address regions: the stack, the sockopt context and the counter's
// map value each sit in their own range, so a load names the region it reads.
const (
	simStackBase   = uint64(1) << 40
	simCtxBase     = uint64(2) << 40
	simCounterBase = uint64(3) << 40
	simMapFD       = uint64(4) << 40
	simStackSize   = 512
	simStepsMax    = 1000
)

// guardSim is one setsockopt call as the kernel would present it to the guard,
// and what the guard did with it.
type guardSim struct {
	level       uint32
	optname     uint32
	netnsCookie uint64 // bpf_get_netns_cookie of the socket
	nsAnswer    int64  // bpf_get_ns_current_pid_tgid's return: 0 when the caller runs in the named PID namespace
	nsTgid      uint32 // the caller's tgid in that namespace
	initialTgid uint32 // the caller's tgid in the initial PID namespace

	gotDev   uint64
	gotInode uint64
	counter  uint64
}

// run interprets the guard program over the call and answers its return value.
// It knows the opcodes the guard uses and fails the test on any other, so a new
// instruction cannot pass unexamined.
func (s *guardSim) run(t *testing.T, insns asm.Instructions) uint64 {
	t.Helper()
	symbols, err := insns.SymbolOffsets()
	if err != nil {
		t.Fatalf("symbols: %v", err)
	}
	var stack [simStackSize]byte
	var r [11]uint64
	r[asm.R1] = simCtxBase
	r[asm.RFP] = simStackBase + simStackSize

	pc := 0
	for range simStepsMax {
		if pc >= len(insns) {
			t.Fatalf("ran off the end of the program")
		}
		ins := insns[pc]
		pc++
		op := ins.OpCode
		operand := uint64(ins.Constant)
		if op.Source() == asm.RegSource {
			operand = r[ins.Src]
		}
		// The simulator covers only the classes the guard program emits; any other
		// class fails the test in default rather than being simulated.
		//exhaustive:ignore
		switch op.Class() {
		case asm.ALU64Class:
			// Only the ALU ops the guard program emits are simulated; any other op
			// fails the test in default.
			//exhaustive:ignore
			switch op.ALUOp() {
			case asm.Mov:
				r[ins.Dst] = operand
			case asm.Add:
				r[ins.Dst] += operand
			case asm.RSh:
				r[ins.Dst] >>= operand
			default:
				t.Fatalf("instruction %d: ALU op %v not simulated", pc-1, op.ALUOp())
			}
		case asm.LdClass:
			r[ins.Dst] = uint64(ins.Constant)
			if ins.IsLoadFromMap() {
				r[ins.Dst] = simMapFD
			}
		case asm.LdXClass:
			r[ins.Dst] = s.load(t, &stack, r[ins.Src]+uint64(int64(ins.Offset)), op.Size())
		case asm.StClass:
			s.store(t, &stack, r[ins.Dst]+uint64(int64(ins.Offset)), op.Size(), uint64(ins.Constant))
		case asm.StXClass:
			s.store(t, &stack, r[ins.Dst]+uint64(int64(ins.Offset)), op.Size(), r[ins.Src])
		case asm.JumpClass:
			if op.JumpOp() == asm.Exit {
				return r[asm.R0]
			}
			if op.JumpOp() == asm.Call {
				s.call(t, &stack, &r, ins.Constant)
				continue
			}
			target, labeled := symbols[ins.Reference()]
			if !labeled {
				t.Fatalf("instruction %d: jump to unknown label %q", pc-1, ins.Reference())
			}
			taken := false
			// Exit and Call are handled above; only the conditional jumps the guard
			// program emits are simulated, and any other op fails the test in default.
			//exhaustive:ignore
			switch op.JumpOp() {
			case asm.Ja:
				taken = true
			case asm.JEq:
				taken = r[ins.Dst] == operand
			case asm.JNE:
				taken = r[ins.Dst] != operand
			default:
				t.Fatalf("instruction %d: jump op %v not simulated", pc-1, op.JumpOp())
			}
			if taken {
				pc = target
			}
		default:
			t.Fatalf("instruction %d: class %v not simulated", pc-1, op.Class())
		}
	}
	t.Fatalf("no exit after %d steps", simStepsMax)
	return 0
}

// call answers the four helpers the guard may call, then clobbers R1 to R5 as the
// kernel's calling convention allows.
func (s *guardSim) call(t *testing.T, stack *[simStackSize]byte, r *[11]uint64, helper int64) {
	t.Helper()
	switch helper {
	case asm.FnGetNetnsCookie.Call().Constant:
		if r[asm.R1] != simCtxBase {
			t.Fatalf("bpf_get_netns_cookie given %#x, want the sockopt context", r[asm.R1])
		}
		r[asm.R0] = s.netnsCookie
	case asm.FnGetNsCurrentPidTgid.Call().Constant:
		s.gotDev, s.gotInode = r[asm.R1], r[asm.R2]
		if r[asm.R4] != pidnsInfoSize {
			t.Fatalf("bpf_get_ns_current_pid_tgid size %d, want %d", r[asm.R4], pidnsInfoSize)
		}
		pid, tgid := uint64(0), uint64(0)
		if s.nsAnswer == 0 {
			pid, tgid = uint64(s.nsTgid), uint64(s.nsTgid)
		}
		s.store(t, stack, r[asm.R3], asm.Word, pid)
		s.store(t, stack, r[asm.R3]+4, asm.Word, tgid)
		r[asm.R0] = uint64(s.nsAnswer)
	case asm.FnGetCurrentPidTgid.Call().Constant:
		r[asm.R0] = uint64(s.initialTgid)<<32 | uint64(s.initialTgid)
	case asm.FnMapLookupElem.Call().Constant:
		if r[asm.R1] != simMapFD {
			t.Fatalf("map lookup on %#x, want the refusal counter", r[asm.R1])
		}
		if key := s.load(t, stack, r[asm.R2], asm.Word); key != 0 {
			t.Fatalf("map lookup of key %d, want 0", key)
		}
		r[asm.R0] = simCounterBase
	default:
		t.Fatalf("helper %d not simulated", helper)
	}
	for reg := asm.R1; reg <= asm.R5; reg++ {
		r[reg] = 0xdeadbeef
	}
}

func (s *guardSim) load(t *testing.T, stack *[simStackSize]byte, addr uint64, size asm.Size) uint64 {
	t.Helper()
	switch addr &^ (simStackBase - 1) {
	case simStackBase:
		b := stackSlice(t, stack, addr, size)
		if size == asm.DWord {
			return binary.LittleEndian.Uint64(b)
		}
		return uint64(binary.LittleEndian.Uint32(b))
	case simCtxBase:
		if size != asm.Word {
			t.Fatalf("context read of size %v", size)
		}
		switch addr - simCtxBase {
		case sockoptLevelOffset:
			return uint64(s.level)
		case sockoptOptnameOffset:
			return uint64(s.optname)
		}
		t.Fatalf("context read at offset %d", addr-simCtxBase)
	case simCounterBase:
		return s.counter
	}
	t.Fatalf("load from unmapped address %#x", addr)
	return 0
}

func (s *guardSim) store(t *testing.T, stack *[simStackSize]byte, addr uint64, size asm.Size, value uint64) {
	t.Helper()
	switch addr &^ (simStackBase - 1) {
	case simStackBase:
		b := stackSlice(t, stack, addr, size)
		if size == asm.DWord {
			binary.LittleEndian.PutUint64(b, value)
			return
		}
		binary.LittleEndian.PutUint32(b, uint32(value))
	case simCounterBase:
		s.counter = value
	default:
		t.Fatalf("store to unmapped address %#x", addr)
	}
}

func stackSlice(t *testing.T, stack *[simStackSize]byte, addr uint64, size asm.Size) []byte {
	t.Helper()
	offset := int(addr - simStackBase)
	width := size.Sizeof()
	if offset < 0 || offset+width > simStackSize {
		t.Fatalf("stack access at fp%+d, width %d, outside the frame", offset-simStackSize, width)
	}
	return stack[offset : offset+width]
}

// TestOverrideGuardProgramExemptsZeInItsPIDNamespace proves the guard's verdicts
// by interpreting its instructions over simulated setsockopt calls: an
// unguarded option or a socket in another network namespace passes uncounted,
// Ze's own call passes when its initial-namespace tgid differs from os.Getpid (a
// container), and every other caller of a per-socket policy option is refused
// and counted, a caller in another PID namespace included even when its
// initial-namespace tgid happens to equal Ze's.
//
// VALIDATES: overrideGuardInstructions, the bpf_get_ns_current_pid_tgid exemption.
// PREVENTS: the exemption comparing the initial-namespace tgid with os.Getpid,
// which refused Ze's own ESP form reader policy in every container.
func TestOverrideGuardProgramExemptsZeInItsPIDNamespace(t *testing.T) {
	const (
		cookie = uint64(0xc00c1e)
		tgid   = uint32(7)
		absent = -22 // -EINVAL: the caller does not run in the named PID namespace
	)
	pidns := pidNamespace{dev: 4, inode: 0xeffffffc}
	insns := overrideGuardInstructions(cookie, pidns, tgid, 3)

	for _, tc := range []struct {
		name    string
		call    guardSim
		verdict uint64
		counted bool
		askedNS bool
	}{
		{"other level", guardSim{level: unix.SOL_SOCKET, optname: unix.SO_REUSEADDR, netnsCookie: cookie, nsAnswer: absent}, 1, false, false},
		{"unguarded IPv4 option", guardSim{level: unix.SOL_IP, optname: unix.IP_TOS, netnsCookie: cookie, nsAnswer: absent}, 1, false, false},
		{"another network namespace", guardSim{level: unix.SOL_IP, optname: unix.IP_XFRM_POLICY, netnsCookie: cookie + 1, nsAnswer: absent}, 1, false, false},
		{"Ze in a container", guardSim{level: unix.SOL_IPV6, optname: unix.IPV6_XFRM_POLICY, netnsCookie: cookie, nsTgid: tgid, initialTgid: tgid + 4000}, 1, false, true},
		{"Ze on the host", guardSim{level: unix.SOL_IP, optname: unix.IP_XFRM_POLICY, netnsCookie: cookie, nsTgid: tgid, initialTgid: tgid}, 1, false, true},
		{"another process", guardSim{level: unix.SOL_IP, optname: unix.IP_IPSEC_POLICY, netnsCookie: cookie, nsTgid: tgid + 1, initialTgid: tgid + 1}, 0, true, true},
		{"another IPv6 process", guardSim{level: unix.SOL_IPV6, optname: unix.IPV6_IPSEC_POLICY, netnsCookie: cookie, nsTgid: tgid + 1}, 0, true, true},
		{"another PID namespace", guardSim{level: unix.SOL_IP, optname: unix.IP_XFRM_POLICY, netnsCookie: cookie, nsAnswer: absent, initialTgid: tgid}, 0, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := tc.call
			if got := call.run(t, insns); got != tc.verdict {
				t.Fatalf("verdict %d, want %d", got, tc.verdict)
			}
			if counted := call.counter == 1; counted != tc.counted {
				t.Fatalf("refusal counter %d, counted %v, want %v", call.counter, counted, tc.counted)
			}
			if !tc.askedNS {
				return
			}
			if call.gotDev != pidns.dev || call.gotInode != pidns.inode {
				t.Fatalf("PID namespace asked (%d, %d), want (%d, %d)", call.gotDev, call.gotInode, pidns.dev, pidns.inode)
			}
		})
	}
}

// TestKernelDevEncodesMKDEV proves the PID namespace device is handed to the
// kernel in its own dev_t encoding (major << 20 | minor), which differs from the
// stat(2) encoding once the minor reaches 256.
//
// VALIDATES: kernelDev.
// PREVENTS: an nsfs device with a large minor never matching ns_match, which would
// refuse Ze's own call.
func TestKernelDevEncodesMKDEV(t *testing.T) {
	for _, tc := range []struct{ major, minor uint32 }{{0, 4}, {0, 300}, {8, 1}, {259, 70000}} {
		got := kernelDev(unix.Mkdev(tc.major, tc.minor))
		want := uint64(tc.major)<<20 | uint64(tc.minor)
		if got != want {
			t.Errorf("major %d minor %d: %#x, want %#x", tc.major, tc.minor, got, want)
		}
	}
}
