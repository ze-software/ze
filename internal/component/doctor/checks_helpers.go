// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Related: doctor.go — readiness check runner and output contract
// Related: checks_config.go — config coherence checks built on these helpers

// Shared config-tree navigation helpers used by the check implementations
// (checks_config.go, checks_listener.go, checks_reach.go, checks_storage.go,
// checks_platform.go, checks_linux.go).

package doctor

import (
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/config"
)

// configTrueValue is the canonical boolean true spelling in config leaves.
const configTrueValue = "true"

// envTypeString is the value env.EnvEntry.Type takes for a string-valued
// variable. env declares the vocabulary in a comment on that field rather than
// as constants, so each caller spells it (internal/core/env/registry.go).
const envTypeString = "string"

func configTimeout(tree *config.Tree, leaf string, def int) time.Duration {
	if v, ok := tree.Get(leaf); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return time.Duration(def) * time.Second
}

func configEnabled(tree *config.Tree, defaultValue bool) bool {
	if tree == nil {
		return false
	}
	if v, ok := tree.Get("enabled"); ok {
		return v == configTrueValue
	}
	return defaultValue
}

func getContainerPath(tree *config.Tree, names ...string) *config.Tree {
	cur := tree
	for _, name := range names {
		if cur == nil {
			return nil
		}
		cur = cur.GetContainer(name)
	}
	return cur
}

func nestedValue(tree *config.Tree, path ...string) (string, bool) {
	if len(path) == 0 {
		return "", false
	}
	containerPath := path[:len(path)-1]
	leaf := path[len(path)-1]
	container := getContainerPath(tree, containerPath...)
	if container == nil {
		return "", false
	}
	return container.Get(leaf)
}

func inheritedValue(parent, node *config.Tree, path ...string) (string, bool) {
	if v, ok := nestedValue(node, path...); ok {
		return v, true
	}
	return nestedValue(parent, path...)
}

func nestedSlice(tree *config.Tree, path ...string) []string {
	if len(path) == 0 {
		return nil
	}
	containerPath := path[:len(path)-1]
	leaf := path[len(path)-1]
	container := getContainerPath(tree, containerPath...)
	if container == nil {
		return nil
	}
	return container.GetSlice(leaf)
}

func valueOrDefault(tree *config.Tree, name, def string) string {
	if tree == nil {
		return def
	}
	if v, ok := tree.Get(name); ok && v != "" {
		return v
	}
	return def
}
