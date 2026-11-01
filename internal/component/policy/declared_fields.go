// Design: docs/architecture/core-design.md -- Policy Filter Chain
// Related: fields.go -- the vocabulary and CheckChain for named instances

package policy

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// CheckFieldNames refuses a declared route field the vocabulary does not hold,
// naming the field and who declared it. An external filter declares its reads
// and sets in vocabulary names at Stage 1, and a name nothing registered can
// be neither sent nor checked.
func CheckFieldNames(declarer string, names []string) error {
	vocab.mu.RLock()
	defer vocab.mu.RUnlock()
	for _, name := range names {
		if _, known := vocab.fields[name]; known {
			continue
		}
		var tb textbuf.Buffer
		tb.Str(declarer).Str(" names route field ").Quoted(name).Str(", which no policy field is called; ").
			Str("the route fields are ").Str(vocab.fieldNames())
		return errors.New(tb.String())
	}
	return nil
}

// CheckDeclaredFilter refuses a chain at an attachment when an external
// filter's declared reads name a field the attachment does not carry, or its
// declared sets one the attachment does not allow. It is the judgement
// CheckChain makes of a named instance, made of what a plugin declared at
// Stage 1 (spec D-10). reference is the chain member as the config spells it.
func CheckDeclaredFilter(attachment, reference string, reads, sets []string) error {
	vocab.mu.RLock()
	defer vocab.mu.RUnlock()
	at, ok := vocab.attachments[attachment]
	if !ok {
		return errors.New("policy: no attachment " + attachment + " is registered, so its chain cannot be checked")
	}
	readFields, err := vocab.fieldsNamed(reference, reads)
	if err != nil {
		return err
	}
	setFields, err := vocab.fieldsNamed(reference, sets)
	if err != nil {
		return err
	}
	var tb textbuf.Buffer
	if missing := absent(readFields, at.first); missing != "" {
		tb.Str(attachment).Str(" filter ").Quoted(reference).Str(" reads ").Str(missing).Str(", which ").
			Str(attachment).Str(" routes do not carry").Str(vocab.synonymHint(attachment, readFields)).
			Str("; remove it from this chain or use a filter that reads only ").Str(joinFields(at.first))
		return errors.New(tb.String())
	}
	if missing := absent(setFields, at.second); missing != "" {
		tb.Str(attachment).Str(" filter ").Quoted(reference).Str(" sets ").Str(missing).
			Str(", which a filter at ").Str(attachment).Str(" may not set; remove it from this chain")
		return errors.New(tb.String())
	}
	return nil
}

// fieldsNamed resolves declared names to fields. Callers MUST hold v.mu.
func (v *vocabulary) fieldsNamed(declarer string, names []string) ([]Field, error) {
	fields := make([]Field, 0, len(names))
	for _, name := range names {
		f, known := v.fields[name]
		if !known {
			var tb textbuf.Buffer
			tb.Str(declarer).Str(" names route field ").Quoted(name).Str(", which no policy field is called")
			return nil, errors.New(tb.String())
		}
		fields = append(fields, f)
	}
	return fields, nil
}

// fieldNames joins every registered field name, sorted. Callers MUST hold
// v.mu.
func (v *vocabulary) fieldNames() string {
	names := make([]Field, 0, len(v.fields))
	for _, f := range v.fields {
		names = append(names, f)
	}
	slices.SortFunc(names, func(a, b Field) int { return strings.Compare(a.name, b.name) })
	return joinFields(names)
}
