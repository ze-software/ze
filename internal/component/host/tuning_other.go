// Design: plan/learned/697-host-2-tuning.md — runtime hardware tuning
// Overview: tuning.go — TuningConfig, TuningResult types

//go:build !linux

package host

func applyTuning(cfg TuningConfig) TuningResult {
	_ = cfg
	return TuningResult{
		Errors: []TuningError{{
			Operation: "tuning",
			Subject:   "platform",
			Err:       ErrUnsupported,
		}},
	}
}
