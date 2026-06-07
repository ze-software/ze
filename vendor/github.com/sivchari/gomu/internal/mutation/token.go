package mutation

import "go/token"

// stringToToken converts a string representation of an operator to its token.Token.
//
//nolint:gocyclo,cyclop
func stringToToken(s string) token.Token {
	switch s {
	// Arithmetic
	case "+":
		return token.ADD
	case "-":
		return token.SUB
	case "*":
		return token.MUL
	case "/":
		return token.QUO
	case "%":
		return token.REM
	case "++":
		return token.INC
	case "--":
		return token.DEC

	// Arithmetic assignment
	case "+=":
		return token.ADD_ASSIGN
	case "-=":
		return token.SUB_ASSIGN
	case "*=":
		return token.MUL_ASSIGN
	case "/=":
		return token.QUO_ASSIGN

	// Conditional
	case "==":
		return token.EQL
	case "!=":
		return token.NEQ
	case "<":
		return token.LSS
	case "<=":
		return token.LEQ
	case ">":
		return token.GTR
	case ">=":
		return token.GEQ

	// Logical
	case "&&":
		return token.LAND
	case "||":
		return token.LOR

	// Bitwise
	case "&":
		return token.AND
	case "|":
		return token.OR
	case "^":
		return token.XOR
	case "&^":
		return token.AND_NOT
	case "<<":
		return token.SHL
	case ">>":
		return token.SHR

	// Bitwise assignment
	case "&=":
		return token.AND_ASSIGN
	case "|=":
		return token.OR_ASSIGN
	case "^=":
		return token.XOR_ASSIGN
	case "<<=":
		return token.SHL_ASSIGN
	case ">>=":
		return token.SHR_ASSIGN

	default:
		return token.ILLEGAL
	}
}
