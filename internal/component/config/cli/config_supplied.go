// Design: docs/architecture/api/commands.md -- caller-supplied configuration inspection.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	suppliedBefore = "before"
	suppliedAfter  = "after"
	suppliedName   = "supplied.conf"
)

type suppliedProducer func(command.ValidatedArgs, []byte, string) command.LocalAnswer

// suppliedHandler admits bytes through the normal bounded command argument.
// It opens no file, store or candidate, and keeps the original caller context.
func suppliedHandler(path string, produce suppliedProducer) command.LocalDataHandler {
	return func(args command.ValidatedArgs) command.LocalAnswer {
		if err := args.Context().Err(); err != nil {
			return suppliedRefusal(path, "request canceled", err)
		}
		if len(args.PositionalTokens()) != 0 {
			return suppliedRefusal(path, "unexpected arguments", nil)
		}
		encoded, found := args.Keyed("content")
		if !found {
			return suppliedRefusal(path, "content is missing", nil)
		}
		data, err := command.DecodeContent(encoded)
		if err != nil {
			return suppliedRefusal(path, "content could not be decoded", err)
		}
		if len(data) == 0 {
			return suppliedRefusal(path, "content is empty", nil)
		}
		name := suppliedName
		if label, found := args.Keyed("name"); found {
			if !storage.PlainConfigName(label) || label == "" {
				return suppliedRefusal(path, "name must be a filename without a directory", nil)
			}
			for _, r := range label {
				if unicode.IsControl(r) {
					return suppliedRefusal(path, "name contains a control character", nil)
				}
			}
			name = label
		}
		return suppliedAnswer(path, produce, args, data, name)
	}
}

// suppliedAnswer bounds the produced bytes as well as the request. A compact
// nested source can expand substantially when serialized as repeated set paths.
func suppliedAnswer(path string, produce suppliedProducer, args command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	if err := args.Context().Err(); err != nil {
		return suppliedRefusal(path, "request canceled", err)
	}
	answer := produce(args, data, name)
	if err := args.Context().Err(); err != nil {
		return suppliedRefusal(path, "request canceled", err)
	}
	if !args.Route().CallerLocal() {
		if blob, text := answer.Payload.(command.Blob); text {
			if len(blob) > command.BlobRemoteOctetsMax {
				return suppliedRefusal(path, "the produced text exceeds the daemon response limit; use the local ze config command on the host holding the source", nil)
			}
		}
	}
	return answer
}

func suppliedRefusal(path, problem string, cause error) command.LocalAnswer {
	var message, fix textbuf.Buffer
	message.Str(path).Str(": ").Str(problem)
	fix.Str(path).Str(" help")
	return command.LocalRefusal(exitError, command.LocalErrorLine(opererr.Wrap(cause,
		message.String(),
		"This command inspects supplied bytes only; give padded base64 content and the declared keywords.",
		fix.String())))
}

func suppliedDump(args command.ValidatedArgs, data []byte, _ string) command.LocalAnswer {
	var diag textbuf.Buffer
	result, err := resolveDumpBytes(data, args.Flag(leafStripPrivate), &diag)
	if err != nil {
		return command.LocalRefusal(exitError, diag.String()+command.LocalErrorLine(err))
	}
	return command.LocalAnswer{Payload: result.dumpMap, Diagnostic: diag.String()}
}

func suppliedValidation(_ command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	return validationBytesAnswer(data, name)
}

func suppliedFmt(args command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	return fmtBytesAnswer(name, data, args.Flag("check"), args.Flag("diff"), false)
}

func suppliedGraph(_ command.ValidatedArgs, data []byte, _ string) command.LocalAnswer {
	return graphBytesAnswer(data)
}

func suppliedDiff(_ command.ValidatedArgs, data []byte, _ string) command.LocalAnswer {
	const path = "show config supplied diff"
	before, after, err := suppliedPair(data)
	if err != nil {
		return suppliedRefusal(path, "comparison content must contain before and after configuration strings", err)
	}
	schema, declared, err := config.SchemaWithDeclaredPlugins(
		config.ConfigSource{Text: string(before)},
		config.ConfigSource{Text: string(after)},
	)
	if err != nil {
		return suppliedRefusal(path, "configuration schema is unavailable", err)
	}
	first, err := parseAndResolve(schema, before)
	if err != nil {
		if refusal := declaredParseExplanation(declared, err); refusal != nil {
			return command.LocalRefusal(exitError, command.LocalErrorLine(refusal))
		}
		return suppliedRefusal(path, "before configuration could not be parsed or resolved", err)
	}
	second, err := parseAndResolve(schema, after)
	if err != nil {
		if refusal := declaredParseExplanation(declared, err); refusal != nil {
			return command.LocalRefusal(exitError, command.LocalErrorLine(refusal))
		}
		return suppliedRefusal(path, "after configuration could not be parsed or resolved", err)
	}
	return diffAnswer(diffSources(schema, first, second))
}

// suppliedPair consumes exactly one bounded JSON object. Token decoding keeps
// duplicate and null fields visible; ordinary struct unmarshaling loses them.
func suppliedPair(data []byte) ([]byte, []byte, error) {
	return suppliedPairFields(data, suppliedBefore, suppliedAfter)
}

// suppliedPairFields keeps the same strict two-string envelope for comparison
// sources and completion input. The field names are owner-declared constants.
func suppliedPairFields(data []byte, first, second string) ([]byte, []byte, error) {
	if !utf8.Valid(data) {
		return nil, nil, errors.New("the comparison envelope is not UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, nil, errors.New("the comparison envelope must be a JSON object")
	}
	var before, after []byte
	seenBefore, seenAfter := false, false
	for decoder.More() {
		field, err := decoder.Token()
		if err != nil {
			return nil, nil, errors.New("the comparison envelope has an invalid field")
		}
		value, err := decoder.Token()
		if err != nil {
			return nil, nil, errors.New("the comparison envelope has an invalid value")
		}
		text, isString := value.(string)
		if !isString {
			return nil, nil, errors.New("each comparison source must be a string")
		}
		switch field {
		case first:
			if seenBefore {
				return nil, nil, errors.New("the " + first + " source was given more than once")
			}
			seenBefore, before = true, []byte(text)
		case second:
			if seenAfter {
				return nil, nil, errors.New("the " + second + " source was given more than once")
			}
			seenAfter, after = true, []byte(text)
		default:
			return nil, nil, errors.New("only " + first + " and " + second + " fields are accepted")
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || !seenBefore || !seenAfter {
		return nil, nil, errors.New("the comparison envelope needs both " + first + " and " + second + " sources")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, nil, errors.New("nothing may follow the comparison envelope")
	}
	return before, after, nil
}
