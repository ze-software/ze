package iface

import "testing"

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "nil args", args: nil, want: 1},
		{name: "help", args: []string{"help"}, want: 0},
		{name: "dash h", args: []string{"-h"}, want: 0},
		{name: "dash dash help", args: []string{"--help"}, want: 0},
		{name: "bogus subcommand", args: []string{"bogus"}, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Run(tt.args); got != tt.want {
				t.Errorf("Run(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestCmdShow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want int
	}{
		// cmdShow with nil args: fs.Parse(nil) succeeds, len(remaining)==0 -> returns 0
		{name: "nil args returns stub", args: nil, want: 0},
		{name: "dash dash help", args: []string{"--help"}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cmdShow(tt.args); got != tt.want {
				t.Errorf("cmdShow(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestCmdCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "nil args", args: nil, want: 1},
		{name: "help", args: []string{"help"}, want: 0},
		{name: "unknown type", args: []string{"unknown"}, want: 1},
		// dummy with no name: cmdCreateDummy(nil) -> len(args)!=1 -> returns 1
		{name: "dummy no name", args: []string{"dummy"}, want: 1},
		// veth with only one arg: cmdCreateVeth(["a"]) -> len(args)!=2 -> returns 1
		{name: "veth no peer", args: []string{"veth", "a"}, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cmdCreate(tt.args); got != tt.want {
				t.Errorf("cmdCreate(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestCmdDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "nil args", args: nil, want: 1},
		{name: "help", args: []string{"help"}, want: 0},
		// Two args: first arg is not help, len(args)!=1 -> returns 1
		{name: "too many args", args: []string{"a", "b"}, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cmdDelete(tt.args); got != tt.want {
				t.Errorf("cmdDelete(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestCmdUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "nil args", args: nil, want: 1},
		{name: "help", args: []string{"help"}, want: 0},
		{name: "bogus action", args: []string{"bogus"}, want: 1},
		// add with no further args: cmdUnitAdd(nil) -> len(args)!=2 -> returns 1
		{name: "add no args", args: []string{"add"}, want: 1},
		// add eth0 abc: strconv.Atoi("abc") fails -> returns 1
		{name: "add non-numeric unit", args: []string{"add", "eth0", "abc"}, want: 1},
		// add eth0 0: unitID <= 0 -> returns 1
		{name: "add unit zero", args: []string{"add", "eth0", "0"}, want: 1},
		// add eth0 -1: unitID <= 0 -> returns 1
		{name: "add negative unit", args: []string{"add", "eth0", "-1"}, want: 1},
		// del with no further args: cmdUnitDel(nil) -> len(args)!=2 -> returns 1
		{name: "del no args", args: []string{"del"}, want: 1},
		// del eth0 0: unitID <= 0 -> returns 1
		{name: "del unit zero", args: []string{"del", "eth0", "0"}, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cmdUnit(tt.args); got != tt.want {
				t.Errorf("cmdUnit(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestCmdAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "nil args", args: nil, want: 1},
		{name: "help", args: []string{"help"}, want: 0},
		{name: "bogus action", args: []string{"bogus"}, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cmdAddr(tt.args); got != tt.want {
				t.Errorf("cmdAddr(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestParseAddrArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantIface string
		wantCIDR  string
		wantOK    bool
	}{
		{
			name:      "unit 0 uses parent name",
			args:      []string{"eth0", "unit", "0", "10.0.0.1/24"},
			wantIface: "eth0",
			wantCIDR:  "10.0.0.1/24",
			wantOK:    true,
		},
		{
			name:      "unit 100 appends dot suffix",
			args:      []string{"eth0", "unit", "100", "10.0.0.1/24"},
			wantIface: "eth0.100",
			wantCIDR:  "10.0.0.1/24",
			wantOK:    true,
		},
		{
			name:   "wrong keyword instead of unit",
			args:   []string{"eth0", "notunit", "0", "10.0.0.1/24"},
			wantOK: false,
		},
		{
			name:   "non-numeric unit id",
			args:   []string{"eth0", "unit", "abc", "10.0.0.1/24"},
			wantOK: false,
		},
		{
			name:   "negative unit id",
			args:   []string{"eth0", "unit", "-1", "10.0.0.1/24"},
			wantOK: false,
		},
		{
			name:   "too few args",
			args:   []string{},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotIface, gotCIDR, gotOK := parseAddrArgs("test", tt.args)
			if gotOK != tt.wantOK {
				t.Fatalf("parseAddrArgs(%v) ok = %v, want %v", tt.args, gotOK, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if gotIface != tt.wantIface {
				t.Errorf("parseAddrArgs(%v) iface = %q, want %q", tt.args, gotIface, tt.wantIface)
			}
			if gotCIDR != tt.wantCIDR {
				t.Errorf("parseAddrArgs(%v) cidr = %q, want %q", tt.args, gotCIDR, tt.wantCIDR)
			}
		})
	}
}

func TestParseIfaceUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantName string
		wantUnit int
		wantOK   bool
	}{
		{
			name:     "eth0.0",
			input:    "eth0.0",
			wantName: "eth0",
			wantUnit: 0,
			wantOK:   true,
		},
		{
			name:     "eth0.100",
			input:    "eth0.100",
			wantName: "eth0",
			wantUnit: 100,
			wantOK:   true,
		},
		{
			name:   "no dot",
			input:  "eth0",
			wantOK: false,
		},
		{
			name:   "empty name before dot",
			input:  ".0",
			wantOK: false,
		},
		{
			name:   "empty unit after dot",
			input:  "eth0.",
			wantOK: false,
		},
		{
			name:   "negative unit",
			input:  "eth0.-1",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotName, gotUnit, gotOK := parseIfaceUnit(tt.input)
			if gotOK != tt.wantOK {
				t.Fatalf("parseIfaceUnit(%q) ok = %v, want %v", tt.input, gotOK, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if gotName != tt.wantName {
				t.Errorf("parseIfaceUnit(%q) name = %q, want %q", tt.input, gotName, tt.wantName)
			}
			if gotUnit != tt.wantUnit {
				t.Errorf("parseIfaceUnit(%q) unit = %d, want %d", tt.input, gotUnit, tt.wantUnit)
			}
		})
	}
}

func TestCmdMigrate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want int
	}{
		// nil args: fs.Parse(nil) succeeds, from=="" -> prints error -> returns 1
		{name: "nil args", args: nil, want: 1},
		{name: "dash dash help", args: []string{"--help"}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cmdMigrate(tt.args); got != tt.want {
				t.Errorf("cmdMigrate(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}
