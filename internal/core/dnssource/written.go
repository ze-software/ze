// Design: docs/features/dns-resolver.md -- the dns-source list and the learned record
// Related: select.go -- NewWrittenList, which validates what ReadWritten reads

package dnssource

import (
	"errors"
	"strings"
)

// ReadWritten reads one dns-source container from a plugin's config JSON:
// body is the container's object, which a plugin receives with every value a
// string and a leaf-list as an array of strings (a one-entry leaf-list may
// arrive as a bare string). path is the container's config path, used in the
// refusals. It answers the container as written; NewWrittenList then
// validates it. A word the YANG enumerations would refuse is refused here in
// the operator's terms, because a plugin's offline verifier can meet it
// before the YANG walk does.
func ReadWritten(path string, body map[string]any) (*Written, error) {
	sources, err := stringList(path, "source", body["source"])
	if err != nil {
		return nil, err
	}
	interfaces, err := stringList(path, "received-interface", body["received-interface"])
	if err != nil {
		return nil, err
	}
	written := &Written{ReceivedInterfaces: interfaces}
	for _, word := range sources {
		class, ok := ParseClass(word)
		if !ok {
			return nil, unknownClassError(path, word)
		}
		written.Sources = append(written.Sources, class)
	}
	raw, present := body["mode"]
	if !present {
		return written, nil
	}
	word, isString := raw.(string)
	if !isString {
		return nil, errors.New(path + " mode: not a word, so Ze cannot tell how the sources combine\n" +
			"set " + path + " mode first-usable, or set " + path + " mode merge")
	}
	mode, known := ParseMode(word)
	if !known {
		return nil, errors.New(path + " mode " + word + ": not a mode, so Ze cannot tell how the sources combine\n" +
			"set " + path + " mode first-usable, or set " + path + " mode merge")
	}
	written.Mode = mode
	return written, nil
}

// stringList reads a leaf-list value: absent is empty, a bare string is one
// entry, an array must hold strings only.
func stringList(path, leaf string, value any) ([]string, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		return []string{v}, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			word, ok := item.(string)
			if !ok {
				return nil, errors.New(path + " " + leaf + ": an entry is not a word, so Ze cannot read the list\n" +
					"delete " + path + " " + leaf + " and write each entry again")
			}
			out = append(out, word)
		}
		return out, nil
	}
	return nil, errors.New(path + " " + leaf + ": not a list of words, so Ze cannot read it\n" +
		"delete " + path + " " + leaf + " and write each entry again")
}

// unknownClassError refuses a source word that names no class, listing the
// classes there are.
func unknownClassError(path, word string) error {
	known := Classes()
	names := make([]string, len(known))
	for i, class := range known {
		names[i] = string(class)
	}
	return errors.New(path + " source " + word + ": not a source class, so Ze cannot tell where to take resolvers from\n" +
		"delete " + path + " source " + word + ", and name one of " + strings.Join(names, ", "))
}
