// Design: docs/architecture/testing/interop.md -- discovered native interop scenario.
package bgp

func init() {
	specialCheckers["bgp-parsed-empty-mp-unreach-frr"] = checkParsedEmptyMPFRR
	speakerRunners[parsedEmptyMPSource] = runParsedEmptyMPSource
}
