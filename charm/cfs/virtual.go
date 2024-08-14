package cfs

import (
	"bytes"
	"io/fs"
	"time"
)

// VirtualFile implements fs.File interface for virtual files in memory.
type VirtualFile struct {
	*bytes.Reader
	name string
}

// Close implements fs.File.Close
func (f *VirtualFile) Close() error {
	return nil
}

// Stat implements fs.File.Stat
func (f *VirtualFile) Stat() (fs.FileInfo, error) {
	return &VirtualFileInfo{
		name: f.name,
		size: int64(f.Len()),
	}, nil
}

// VirtualFileInfo implements fs.FileInfo interface
type VirtualFileInfo struct {
	name string
	size int64
}

func (fi *VirtualFileInfo) Name() string       { return fi.name }
func (fi *VirtualFileInfo) Size() int64        { return fi.size }
func (fi *VirtualFileInfo) Mode() fs.FileMode  { return 0444 } // read-only
func (fi *VirtualFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *VirtualFileInfo) IsDir() bool        { return false }
func (fi *VirtualFileInfo) Sys() interface{}   { return nil }

// CreateVirtualFile creates a virtual fs.File from bytes
func CreateVirtualFile(name string, data []byte) fs.File {
	return &VirtualFile{
		Reader: bytes.NewReader(data),
		name:   name,
	}
}
