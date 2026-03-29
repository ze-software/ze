// Design: docs/architecture/web-interface.md -- Display-time value decoration
// Overview: render.go -- Template rendering uses decorators via FieldMeta
// Related: decorator_asn.go -- ASN name decorator implementation

package web

// Decorator resolves supplementary display text for a leaf value.
// Implementations are registered by name and matched to YANG leaves
// via the ze:decorate extension.
type Decorator interface {
	// Name returns the decorator name matching the YANG extension argument.
	Name() string
	// Decorate returns an annotation string for the given value.
	// Returns empty string and nil error if no annotation is available.
	Decorate(value string) (string, error)
}

// DecoratorRegistry holds registered decorators keyed by name.
// Safe for concurrent reads after initial registration at startup.
type DecoratorRegistry struct {
	decorators map[string]Decorator
}

// NewDecoratorRegistry creates an empty decorator registry.
func NewDecoratorRegistry() *DecoratorRegistry {
	return &DecoratorRegistry{
		decorators: make(map[string]Decorator),
	}
}

// Register adds a decorator to the registry. d MUST NOT be nil.
// MUST be called before the HTTP server starts serving (not concurrent-safe for writes).
func (r *DecoratorRegistry) Register(d Decorator) {
	r.decorators[d.Name()] = d
}

// Get returns the decorator for the given name, or nil if not registered.
func (r *DecoratorRegistry) Get(name string) Decorator {
	return r.decorators[name]
}

// ResolveField resolves the decoration for a single FieldMeta. field MUST NOT be nil.
// If the field has a DecoratorName and the value is non-empty,
// the matching decorator is called and Decoration is set.
// Errors are silently ignored (graceful degradation).
func (r *DecoratorRegistry) ResolveField(field *FieldMeta) {
	if field.DecoratorName == "" || field.Value == "" {
		return
	}

	d := r.decorators[field.DecoratorName]
	if d == nil {
		return
	}

	annotation, err := d.Decorate(field.Value)
	if err != nil {
		return
	}

	field.Decoration = annotation
}

// decoratorFunc is a convenience adapter from a function to Decorator.
type decoratorFunc struct {
	name string
	fn   func(string) (string, error)
}

func (d *decoratorFunc) Name() string                          { return d.name }
func (d *decoratorFunc) Decorate(value string) (string, error) { return d.fn(value) }

// DecoratorFunc creates a Decorator from a name and function.
func DecoratorFunc(name string, fn func(string) (string, error)) Decorator {
	return &decoratorFunc{name: name, fn: fn}
}
