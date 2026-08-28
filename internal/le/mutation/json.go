package mutation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

type jsonKind uint8

const (
	jsonNull jsonKind = iota
	jsonBool
	jsonNumber
	jsonString
	jsonArray
	jsonObject
)

type jsonMember struct {
	name  string
	value jsonValue
}

type jsonValue struct {
	kind    jsonKind
	text    string
	boolean bool
	values  []jsonValue
	members []jsonMember
}

func decodeJSON(data []byte) (jsonValue, error) {
	if !utf8.Valid(data) {
		return jsonValue{}, fmt.Errorf("invalid UTF-8 in JSON document")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := decodeValue(decoder)
	if err != nil {
		return jsonValue{}, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return jsonValue{}, fmt.Errorf("unexpected data after JSON value")
		}
		return jsonValue{}, err
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder) (jsonValue, error) {
	token, err := decoder.Token()
	if err != nil {
		return jsonValue{}, err
	}
	switch token := token.(type) {
	case nil:
		return jsonValue{kind: jsonNull}, nil
	case bool:
		return jsonValue{kind: jsonBool, boolean: token}, nil
	case string:
		return jsonValue{kind: jsonString, text: token}, nil
	case json.Number:
		return jsonValue{kind: jsonNumber, text: normalizeNumber(token.String())}, nil
	case json.Delim:
		return decodeCollection(decoder, token)
	default:
		return jsonValue{}, fmt.Errorf("unsupported JSON token %T", token)
	}
}

func decodeCollection(decoder *json.Decoder, opening json.Delim) (jsonValue, error) {
	if opening == '[' {
		value := jsonValue{kind: jsonArray, values: []jsonValue{}}
		for decoder.More() {
			item, err := decodeValue(decoder)
			if err != nil {
				return jsonValue{}, err
			}
			value.values = append(value.values, item)
		}
		if _, err := decoder.Token(); err != nil {
			return jsonValue{}, err
		}
		return value, nil
	}
	if opening != '{' {
		return jsonValue{}, fmt.Errorf("unexpected JSON delimiter %q", opening)
	}

	value := jsonValue{kind: jsonObject, members: []jsonMember{}}
	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return jsonValue{}, err
		}
		name, ok := nameToken.(string)
		if !ok {
			return jsonValue{}, fmt.Errorf("JSON object key is %T, not a string", nameToken)
		}
		member, err := decodeValue(decoder)
		if err != nil {
			return jsonValue{}, err
		}
		value.setMember(name, member)
	}
	if _, err := decoder.Token(); err != nil {
		return jsonValue{}, err
	}
	return value, nil
}

// setMember retains the first key position but the last value, matching the
// ordered dictionary json.load builds when an object repeats a key.
func (v *jsonValue) setMember(name string, value jsonValue) {
	for index := range v.members {
		if v.members[index].name == name {
			v.members[index].value = value
			return
		}
	}
	v.members = append(v.members, jsonMember{name: name, value: value})
}

func (v jsonValue) member(name string) (jsonValue, bool) {
	for _, member := range v.members {
		if member.name == name {
			return member.value, true
		}
	}
	return jsonValue{}, false
}

func normalizeNumber(number string) string {
	if !strings.ContainsAny(number, ".eE") {
		integer := new(big.Int)
		if _, ok := integer.SetString(number, 10); ok {
			return integer.String()
		}
		return number
	}
	value, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return number
	}
	normalized := strconv.FormatFloat(value, 'g', -1, 64)
	if !strings.ContainsAny(normalized, ".eE") {
		normalized += ".0"
	}
	return normalized
}

func pythonScore(killed, total int) string {
	if total == 0 {
		return "0"
	}
	score := math.RoundToEven((100*float64(killed)/float64(total))*10) / 10
	return strconv.FormatFloat(score, 'f', 1, 64)
}

func marshalIndented(value jsonValue) []byte {
	var output bytes.Buffer
	appendIndented(&output, value, 0)
	return output.Bytes()
}

func appendIndented(output *bytes.Buffer, value jsonValue, depth int) {
	switch value.kind {
	case jsonNull:
		output.WriteString("null")
	case jsonBool:
		output.WriteString(strconv.FormatBool(value.boolean))
	case jsonNumber:
		output.WriteString(value.text)
	case jsonString:
		appendPythonString(output, value.text)
	case jsonArray:
		appendArray(output, value.values, depth)
	case jsonObject:
		appendObject(output, value.members, depth)
	}
}

func appendArray(output *bytes.Buffer, values []jsonValue, depth int) {
	if len(values) == 0 {
		output.WriteString("[]")
		return
	}
	output.WriteString("[\n")
	for index, value := range values {
		writeIndent(output, depth+1)
		appendIndented(output, value, depth+1)
		if index+1 < len(values) {
			output.WriteByte(',')
		}
		output.WriteByte('\n')
	}
	writeIndent(output, depth)
	output.WriteByte(']')
}

func appendObject(output *bytes.Buffer, members []jsonMember, depth int) {
	if len(members) == 0 {
		output.WriteString("{}")
		return
	}
	output.WriteString("{\n")
	for index, member := range members {
		writeIndent(output, depth+1)
		appendPythonString(output, member.name)
		output.WriteString(": ")
		appendIndented(output, member.value, depth+1)
		if index+1 < len(members) {
			output.WriteByte(',')
		}
		output.WriteByte('\n')
	}
	writeIndent(output, depth)
	output.WriteByte('}')
}

func writeIndent(output *bytes.Buffer, depth int) {
	for range depth * 2 {
		output.WriteByte(' ')
	}
}

// appendPythonString is json.dumps' ensure_ascii=True spelling. Go's JSON
// encoder intentionally leaves Unicode as UTF-8 and escapes HTML characters;
// the producer did neither, so report bytes need this boundary encoder.
func appendPythonString(output *bytes.Buffer, value string) {
	output.WriteByte('"')
	for _, char := range value {
		switch char {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteRune(char)
		case '\b':
			output.WriteString("\\b")
		case '\f':
			output.WriteString("\\f")
		case '\n':
			output.WriteString("\\n")
		case '\r':
			output.WriteString("\\r")
		case '\t':
			output.WriteString("\\t")
		default:
			appendPythonRune(output, char)
		}
	}
	output.WriteByte('"')
}

func appendPythonRune(output *bytes.Buffer, char rune) {
	if char >= 0x20 && char <= 0x7e {
		output.WriteRune(char)
		return
	}
	if char <= 0xffff {
		fmt.Fprintf(output, "\\u%04x", char)
		return
	}
	char -= 0x10000
	high := rune(0xd800) + char/0x400
	low := rune(0xdc00) + char%0x400
	fmt.Fprintf(output, "\\u%04x\\u%04x", high, low)
}
