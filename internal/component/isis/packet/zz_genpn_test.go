// test-relax: TestZZGenPseudonodeHex was a one-shot generator used to mint the
// pseudo-node LSP hex for test/isis/isis-dis.ci (isis-8). Its output is now pinned
// and validated by pseudonode_ci_test.go (TestISISPseudonodeCIFixtureDecodes),
// which decodes those exact bytes, asserts the non-zero pseudonode LAN ID and the
// metric-0 members, and round-trips the encoder. The generator is removed as
// obsolete scaffolding; it was never spec coverage.
package packet
