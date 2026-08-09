// Design: docs/architecture/storage/smart-health.md -- SMART disk health management
// Related: manager.go — Manager uses Config for health polling

package storage

import "time"

// Config holds SMART management configuration parsed from YANG.
type Config struct {
	Enabled       bool
	CheckInterval time.Duration
	Temperature   TemperatureConfig
	SelfTest      SelfTestConfig
}

// TemperatureConfig defines the temperature alert thresholds.
type TemperatureConfig struct {
	Difference    int
	Informational int
	Critical      int
}

// SelfTestConfig defines periodic self-test scheduling.
type SelfTestConfig struct {
	Short SelfTestSchedule
	Long  SelfTestSchedule
}

// SelfTestSchedule defines when and how often a self-test runs.
type SelfTestSchedule struct {
	Interval  time.Duration
	TimeOfDay string
	Day       string
}

// DefaultConfig returns the default SMART management configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:       false,
		CheckInterval: 30 * time.Minute,
		Temperature: TemperatureConfig{
			Difference:    4,
			Informational: 45,
			Critical:      55,
		},
		SelfTest: SelfTestConfig{
			Short: SelfTestSchedule{
				Interval:  24 * time.Hour,
				TimeOfDay: "02:00",
			},
			Long: SelfTestSchedule{
				Interval:  7 * 24 * time.Hour,
				TimeOfDay: "03:00",
				Day:       "sunday",
			},
		},
	}
}
