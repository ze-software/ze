// Design: docs/architecture/host/tuning.md -- runtime hardware tuning
// Overview: tuning.go — TuningConfig, TuningResult types

//go:build !linux

package host

func applyTuning(cfg TuningConfig) TuningResult {
	_ = cfg
	return TuningResult{
		Errors: []TuningError{{
			Operation: tuningOpUnsupported,
			Subject:   "platform",
			Err:       ErrUnsupported,
		}},
	}
}
