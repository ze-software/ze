// Design: (none -- predates documentation)
// Overview: store.go -- BlobStore uses tree for in-memory indexing

package zefs

import (
	"fmt"
	"strings"
)

// node is a tree node. Leaves hold data, interior nodes hold children.
type node struct {
	data     []byte           // non-nil for leaf nodes (files)
	children map[string]*node // non-nil for interior nodes (directories)
}

func newDirNode() *node {
	return &node{children: make(map[string]*node)}
}

// set inserts or updates a key in the tree. Intermediate directory nodes
// are created as needed. Returns an error if the path conflicts with
// existing entries (e.g., writing "a/b" when "a" is a file, or writing
// "a" when "a/b" exists).
func (n *node) set(key string, data []byte) error {
	parts := strings.Split(key, "/")
	cur := n
	for _, p := range parts[:len(parts)-1] {
		child, ok := cur.children[p]
		if !ok {
			child = newDirNode()
			cur.children[p] = child
		} else if child.children == nil {
			return fmt.Errorf("zefs: path conflict: segment %q in %q is a file", p, key)
		}
		cur = child
	}
	leaf := parts[len(parts)-1]
	if existing, ok := cur.children[leaf]; ok && existing.children != nil {
		return fmt.Errorf("zefs: path conflict: %q is a directory", key)
	}
	cur.children[leaf] = &node{data: data}
	return nil
}

// get returns the data for a key, or nil if not found.
func (n *node) get(key string) ([]byte, bool) {
	nd := n.walk(key)
	if nd == nil || nd.data == nil {
		return nil, false
	}
	return nd.data, true
}

// remove deletes a key. Returns true if the key existed.
// Prunes empty parent directories.
func (n *node) remove(key string) bool {
	parts := strings.Split(key, "/")
	return n.removeRecursive(parts)
}

func (n *node) removeRecursive(parts []string) bool {
	if len(parts) == 1 {
		_, existed := n.children[parts[0]]
		delete(n.children, parts[0])
		return existed
	}
	child, ok := n.children[parts[0]]
	if !ok {
		return false
	}
	removed := child.removeRecursive(parts[1:])
	// Prune empty directory
	if removed && child.children != nil && len(child.children) == 0 {
		delete(n.children, parts[0])
	}
	return removed
}

// has returns true if the key exists as a leaf.
func (n *node) has(key string) bool {
	nd := n.walk(key)
	return nd != nil && nd.data != nil
}

// walk follows path segments to a node. Returns nil if not found.
// "." or "" returns the root node itself.
func (n *node) walk(path string) *node {
	if path == "." || path == "" {
		return n
	}
	parts := strings.Split(path, "/")
	cur := n
	for _, p := range parts {
		if cur.children == nil {
			return nil
		}
		child, ok := cur.children[p]
		if !ok {
			return nil
		}
		cur = child
	}
	return cur
}

// collect gathers all leaf keys under this node.
func (n *node) collect(prefix string, out *[]string) {
	if n.data != nil {
		*out = append(*out, prefix)
		return
	}
	for name, child := range n.children {
		p := name
		if prefix != "" {
			p = prefix + "/" + name
		}
		child.collect(p, out)
	}
}
