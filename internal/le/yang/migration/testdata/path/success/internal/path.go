package internal

func paths(peer Node) {
	_ = "bgp/peer/p1/connection/remote/ip"
	_ = peer.GetContainer("connection").GetContainer("remote")
	terminal := peer.GetContainer("connection")
	_ = terminal
}

type Node interface {
	GetContainer(string) Node
}
