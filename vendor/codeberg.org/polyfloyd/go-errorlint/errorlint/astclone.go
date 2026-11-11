package errorlint

import (
	"go/ast"
	"reflect"
)

var (
	astObjectType = reflect.TypeOf((*ast.Object)(nil))
	astScopeType  = reflect.TypeOf((*ast.Scope)(nil))
)

// cloneStmts returns a deep copy of the given statements. Identifiers present in rename are given
// the mapped name in the copy. The original AST is left untouched, which matters because it is
// shared with other analyzers.
func cloneStmts(stmts []ast.Stmt, rename map[*ast.Ident]string) []ast.Stmt {
	out := make([]ast.Stmt, len(stmts))
	for i, stmt := range stmts {
		out[i] = cloneValue(reflect.ValueOf(stmt), rename).Interface().(ast.Stmt)
	}
	return out
}

func cloneValue(v reflect.Value, rename map[*ast.Ident]string) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		// Deprecated object resolution data is shared rather than copied: it is not used for
		// printing and contains back-references into the tree.
		if v.IsNil() || v.Type() == astObjectType || v.Type() == astScopeType {
			return v
		}
		c := reflect.New(v.Type().Elem())
		c.Elem().Set(cloneValue(v.Elem(), rename))
		if ident, ok := v.Interface().(*ast.Ident); ok {
			if name, ok := rename[ident]; ok {
				c.Interface().(*ast.Ident).Name = name
			}
		}
		return c
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		c := reflect.New(v.Type()).Elem()
		c.Set(cloneValue(v.Elem(), rename))
		return c
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		c := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			c.Index(i).Set(cloneValue(v.Index(i), rename))
		}
		return c
	case reflect.Struct:
		c := reflect.New(v.Type()).Elem()
		c.Set(v)
		for i := range v.NumField() {
			if c.Field(i).CanSet() {
				c.Field(i).Set(cloneValue(v.Field(i), rename))
			}
		}
		return c
	default:
		return v
	}
}
