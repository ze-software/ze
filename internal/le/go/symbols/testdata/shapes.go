// Fixture for TestListMatchesGoplsSymbols: every declaration shape whose
// symbol line gopls prints, with its output recorded in shapes.symbols.
package fixture

import (
	"io"
	"sync"
)

type Color int

const (
	Red Color = iota
	Green
	_
	Blue
)

const a, b = 1, 2

var 𝒳, yy = 1, 2

var ünï = 3

var _ = a

var grouped struct {
	sync.Mutex
	inner struct {
		deep  int
		deep2 string
	}
	n int
}

var plain Color

var empty struct{}

type Shape interface {
	io.Reader
	comparable
	~int | ~string
	Area() float64
	Perimeter(scale float64) (float64, error)
}

type Empty interface{}

type Literal interface {
	struct{ x int } | interface{ M() }
}

type List[T any] struct {
	*sync.RWMutex
	io.Writer
	Pair[T, int]
	head, tail *node[T]
	_          int
	z, Z, aa   int
	nested     struct{ x, y int }
	iface      interface{ Do() }
	fn         func(int) error
}

type Pair[K comparable, V any] struct {
	key K
	val V
}

type node[T any] struct{ v T }

type Alias = List[int]

type GenericAlias[T any] = List[T]

type StructAlias = struct{ s int }

type Handler func(int) error

type Lookup map[string]int

type Ptr *int

type _ int

type (
	One   int
	Two   struct{ a int }
	Three interface{ M() }
)

func (l *List[T]) Push(v T) {}

func (l List[T]) Len() int { return 0 }

func (p *Pair[K, V]) Key() K { return p.key }

func (Color) String() string { return "" }

func (c *Color) Set(v int) { *c = Color(v) }

func _() {}

func Map[T, U any](xs []T, f func(T) U) []U { return nil }

func multi(
	x int,
) {
}

//line moved.go:500
func Moved() {}
