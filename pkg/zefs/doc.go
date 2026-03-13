// Package zefs provides a netcapstring-framed blob store with hierarchical keys.
//
// Keys use "/" separators to form a virtual directory tree. On disk, entries
// are stored as flat netcapstrings with fixed-width headers. In memory, keys
// are indexed as a tree for ReadDir support.
//
// The store implements io/fs.FS, fs.ReadFileFS, and fs.ReadDirFS for reads.
// Writes use WriteFile/Remove following the Go proposal (#67002) signatures.
//
// Disk format:
//
//	ZeFS:<used>:<capacity>:<entries><padding>
//	  <name_used>:<name_cap>:<name><pad> <data_used>:<data_cap>:<data><pad>
//	  ...
//	  \n (end marker)
//	  \0... (container padding)
package zefs
