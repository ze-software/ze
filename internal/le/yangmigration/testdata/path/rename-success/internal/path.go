package internal

func path(peer Node) Node {
	return peer.GetContainer("session").GetContainer("asn")
}

type Node interface {
	GetContainer(string) Node
}
