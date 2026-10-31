# Mutation Testing Framework

This package provides a mutation testing framework with automatic mutator registry management.

## Creating New Mutators

### Quick Start

1. **Generate scaffolding:**
   ```bash
   cd internal/mutation
   go run scaffold.go <mutator_name>
   ```
   
   Example:
   ```bash
   go run scaffold.go bitwise
   ```

2. **Implement the mutator:**
   - Edit the generated `<mutator_name>.go` file
   - Update the TODO items with actual implementation
   - Add supported operators and mutation logic

3. **Add representative code:**
   - Add code to the generated `testdata/<mutator_name>/input.go` that the
     mutator should mutate, and similar code it should not
   - Run `go test ./internal/mutation -run TestMutators/<mutator_name> -update`
   - Review the generated `testdata/<mutator_name>/*.golden` files

4. **Register automatically:**
   ```bash
   go generate
   ```

5. **Test:**
   ```bash
   go test
   ```

### Example: Bitwise Mutator

```bash
# 1. Generate scaffolding
go run scaffold.go bitwise

# 2. Edit bitwise.go - implement these methods:
#    - isBitwiseOp() - check if token is bitwise operator
#    - getBitwiseMutations() - return mutation mappings

# 3. Add representative code to testdata/bitwise/input.go:
#    - Include supported operations like "a & b", "a | b"
#    - Run: go test ./internal/mutation -run TestMutators/bitwise -update

# 4. Register and test
go generate
go test
```

## Manual Registration (Not Recommended)

The framework automatically discovers mutators by scanning for `*Mutator` struct types. However, if you create a mutator manually:

1. Create a struct ending with `Mutator`
2. Implement the `Mutator` interface:
   - `Name() string`
   - `CanMutate(node ast.Node) bool`
   - `Mutate(node ast.Node, fset *token.FileSet) []Mutant`
3. Run `go generate` to update the registry

## Registry System

The registry is automatically generated from existing mutator files:

- **`generate.go`** - Scans for `*Mutator` structs
- **`registry.go`** - Auto-generated mutator list (DO NOT EDIT)
- **`engine.go`** - Uses `getAllMutators()` for initialization

### How It Works

1. `go generate` runs `generate.go`
2. `generate.go` scans `*.go` files for `*Mutator` structs
3. Generates `registry.go` with `getAllMutators()` function
4. Engine uses registry instead of hardcoded list

## Existing Mutators

- **ArithmeticMutator** - Mutates `+`, `-`, `*`, `/`, `%`, `++`, `--`
- **ConditionalMutator** - Mutates `==`, `!=`, `<`, `<=`, `>`, `>=`  
- **LogicalMutator** - Mutates `&&`, `||`, `!`
- **SwitchMutator** - Empties non-empty `case`/`default` clause bodies (type-switch clauses that leave the switch variable unused are reported `NOT_VIABLE`; no clause deletion or forced case selection)

## Architecture

```
Engine
├── getAllMutators() ← registry.go (auto-generated)
├── ArithmeticMutator
├── ConditionalMutator
└── LogicalMutator
```

The scaffolding tool generates mutators following the same patterns as existing ones, ensuring consistency and reducing boilerplate code.