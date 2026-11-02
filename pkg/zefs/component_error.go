// Design: docs/architecture/zefs-format.md -- secure framed-tree traversal refusals.
// Related: check_tree_unix.go -- OpenDirectory, the walk that returns it.

package zefs

import "io/fs"

// ComponentError is OpenDirectory refusing one component of the path it
// walks. Parent, Component and Folder are absolute paths the walk built:
// Folder is the whole path it was asked to open, Component the one it could
// not, and Parent the folder holding it. A caller that names the folder in
// other words than the ones the walk used compares Component with Folder to
// tell the folder itself from a folder on the path to it, rather than reading
// the message. It matches fs.ErrPermission and its cause. It is declared on
// every platform so a caller can name it in an untagged file, even where
// OpenDirectory never returns it.
type ComponentError struct {
	Parent    string
	Component string
	Folder    string
	Err       error
}

// Error names the component by the absolute path the walk built.
func (e *ComponentError) Error() string {
	return "zefs: open " + e.Parent + ": unsafe component " + e.Component + ": " + e.Err.Error() + ": " + fs.ErrPermission.Error()
}

// Unwrap answers the cause and fs.ErrPermission, which every refusal of the
// walk matches.
func (e *ComponentError) Unwrap() []error { return []error{e.Err, fs.ErrPermission} }
