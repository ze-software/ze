// Package fat implements writing FAT16B file system images, which is
// useful when generating images for embedded devices such as the
// Raspberry Pi. With regards to reading, getting file offsets and
// lengths is implemented.
//
// The resulting images use a cluster size of 4 sectors and a sector
// size of 512 bytes, i.e. their size is limited to about 127 MB.
//
// Filenames are restricted to 8 characters + 3 characters for the
// file extension.
package fat
