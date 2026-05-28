// Design: docs/architecture/api/architecture.md -- gNMI path translation
// Related: server.go -- gNMI server core

package gnmi

import (
	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

const maxPathDepth = 64

// pathToSegments converts a gNMI Path to a flat slice of config tree path
// segments. List keys are interleaved: /bgp/neighbor[address=10.0.0.1]/description
// becomes ["bgp", "neighbor", "10.0.0.1", "description"].
//
// Ze's config tree uses positional list keys (the key value is a direct
// child name in the list map), not named keys. When a PathElem has exactly
// one key, its value is used as the list entry identifier. Multiple keys
// are not supported (Ze lists are single-keyed).
func pathToSegments(p *gpb.Path) ([]string, error) {
	if p == nil {
		return nil, nil
	}
	if len(p.Elem) > maxPathDepth {
		return nil, errPathTooDeep
	}
	segments := make([]string, 0, len(p.Elem)*2)
	for _, elem := range p.Elem {
		if elem.Name == "" {
			return nil, errEmptyPathElement
		}
		segments = append(segments, elem.Name)
		if len(elem.Key) == 1 {
			for _, v := range elem.Key {
				segments = append(segments, v)
			}
		} else if len(elem.Key) > 1 {
			return nil, errMultipleKeys
		}
	}
	return segments, nil
}

// segmentsToPath converts config tree path segments back to a gNMI Path.
// Heuristic: if a segment follows a known list name, it is treated as a
// key value rather than a container name. The listNames set provides this
// context from the YANG schema.
func segmentsToPath(segments []string, listNames map[string]bool) *gpb.Path {
	p := &gpb.Path{}
	for i := 0; i < len(segments); i++ {
		elem := &gpb.PathElem{Name: segments[i]}
		if listNames != nil && listNames[segments[i]] && i+1 < len(segments) {
			elem.Key = map[string]string{"name": segments[i+1]}
			i++
		}
		p.Elem = append(p.Elem, elem)
	}
	return p
}
