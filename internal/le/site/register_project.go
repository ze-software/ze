// Design: website/AI.md -- the project overview has one registered publisher.
package site

func init() {
	registerProducer(Producer{Name: "project", Render: renderProject})
}
