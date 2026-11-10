// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- the blind-detector diagnostic through show doctor
// Related: ddos_detect_blind.go -- doctorDDoSDetectBlind, the observer registered below

package fixture

func init() {
	// test/plugin/ddos-detect-external-blind.ci: an external ddos-detect is
	// reported through show doctor, the daemon form of ze doctor, with its cause.
	Register("plugin/doctor-ddos-detect-blind", observer02("doctor-ddos-detect-blind-test", doctorDDoSDetectBlind))
}
