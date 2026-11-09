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

// ContainerKey is the name of the dns-source container in a consumer's config
// JSON.
const ContainerKey = "dns-source"

// ParseContainer reads the dns-source container of parent, a consumer's
// config object, into the list the consumer selects with. A parent that holds
// no container answers builtin, the consumer's built-in default (D-5), whose
// path names the container in every refusal; a container that is not an
// object is refused rather than read as absent, because the default would
// then stand in for a list the operator wrote. The written list is validated
// by NewWrittenList with family, audience and the classes the consumer
// refuses.
func ParseContainer(parent map[string]any, builtin List, family Family, audience Audience, refused ...Refusal) (List, error) {
	raw, present := parent[ContainerKey]
	if present {
		return parseWrittenContainer(builtin.Path(), raw, family, audience, refused)
	}
	return builtin, nil
}

// parseWrittenContainer reads one written container at path.
func parseWrittenContainer(path string, raw any, family Family, audience Audience, refused []Refusal) (List, error) {
	body, isObject := raw.(map[string]any)
	if !isObject {
		return List{}, errors.New(path + " is not a container; write it as " + path + " { source <class>; }")
	}
	written, err := ReadWritten(path, body)
	if err != nil {
		return List{}, err
	}
	return NewWrittenList(path, family, audience, written, refused...)
}
