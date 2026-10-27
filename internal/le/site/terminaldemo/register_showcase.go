// Design: docs/contributing/gh-pages.md -- terminal demo scenarios
// Related: scenarios_showcase.go -- runShowcaseLab, the lab every topic recording starts from

package siteterminaldemo

func init() {
	scenarios.add(showcaseLabID, scenario{run: runShowcaseLab, validate: validateShowcaseLab})
	scenarios.add("showcase", scenario{run: runShowcaseLab, validate: validateShowcase})
	registerLiveTopic("ipsec", "topics/ipsec/configure.tape", validateTopicIPsec, awaitTopicIPsec)
}
