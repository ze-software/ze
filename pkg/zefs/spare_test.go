package zefs

import "testing"

// VALIDATES: the default policy adds 10% to the used length, rounded down,
// which is the data padding every writer applied before the policy existed.
// PREVENTS: a default that moves padding the spec did not ask to move.
func TestWritePolicyCapacityDefault(t *testing.T) {
	policy, err := newWritePolicy(nil)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		used int
		want int
	}{
		{"zero data", 0, 0},
		{"small data", 10, 11},
		{"rounds down", 19, 20},
		{"medium data", 100, 110},
		{"very large", 1_000_000, 1_100_000},
	}
	for _, tt := range tests {
		if got := policy.capacity(tt.used); got != tt.want {
			t.Errorf("%s: capacity(%d) = %d, want %d", tt.name, tt.used, got, tt.want)
		}
	}
}

// VALIDATES: every valid policy returns a capacity that holds the data.
// PREVENTS: an allocated capacity smaller than the data it must hold.
func TestWritePolicyCapacityAlwaysFits(t *testing.T) {
	for _, percent := range []int{0, 1, 10, 25, 100} {
		policy, err := newWritePolicy([]Option{Spare(percent)})
		if err != nil {
			t.Fatal(err)
		}
		for _, used := range []int{0, 1, 63, 64, 65, 100, 1000, 100000, 1000000} {
			capacity := policy.capacity(used)
			if capacity < used {
				t.Errorf("spare %d: capacity(%d) = %d, below used", percent, used, capacity)
			}
			if percent == 0 && capacity != used {
				t.Errorf("spare 0: capacity(%d) = %d, want exact-fit", used, capacity)
			}
		}
	}
}
