// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- the blind-detector diagnostic through show doctor
// Related: ddos_detect_blind.go -- doctorDDoSDetectBlind, doctorDDoSDetectBlindZero, the observers registered below

package fixture

func init() {
	// test/draft/plugin/ddos-detect-external-blind.ci (a draft until the
	// forked plugin gets an event bus): an external ddos-detect is
	// reported through show doctor, the daemon form of ze doctor, with its cause.
	Register("plugin/doctor-ddos-detect-blind", observer02("doctor-ddos-detect-blind-test", doctorDDoSDetectBlind), StopsDaemon)
	// test/plugin/ddos-detect-internal-blind.ci: an internal ddos-detect whose
	// interfaces receive nothing for one baseline window is reported through
	// show doctor with the zero-traffic text.
	Register("plugin/doctor-ddos-detect-blind-zero", observer02("doctor-ddos-detect-blind-zero-test", doctorDDoSDetectBlindZero), StopsDaemon)
}
