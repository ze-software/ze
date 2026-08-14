package parse

import "fmt"

type Position struct {
	Index, Line, Col int
}

func (pos Position) String() string {
	// The line number is 0-based, but text editors are usually 1-based.
	// So the number is incremented by 1 for display.
	return fmt.Sprintf("line %d, col %d", pos.Line+1, pos.Col)
}

type pf[T any] struct {
	f func(in *Input) (item T, ok bool, err error)
}

func (p pf[T]) Parse(in *Input) (item T, ok bool, err error) {
	return p.f(in)
}

// Func creates a parser from an input function.
func Func[T any](f func(in *Input) (item T, ok bool, err error)) Parser[T] {
	return pf[T]{
		f: f,
	}
}

// Parser is implemented by all parsers.
type Parser[T any] interface {
	Parse(in *Input) (item T, ok bool, err error)
}
