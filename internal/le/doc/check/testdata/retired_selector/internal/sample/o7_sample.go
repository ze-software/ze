package sample
func cmds() {
	run("nft", "list")
	run("tc", "qdisc")
	Register("nft", newBackend)
	run("tc", "backend")
}
