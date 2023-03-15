package log

import (
	"io"
	"os"

	"github.com/tysonmote/gommap"
)

var (
	// offWidth is the number of bytes used to represent the offset of a log
	// entry.
	offWidth uint64 = 4
	// posWidth is the number of bytes used to represent the position of a
	// log entry.
	posWidth uint64 = 8
	// entWidth is the total number of bytes used to represent a log entry (
	//offset and position).
	entWidth = offWidth + posWidth
)

// index represents a file-based index of a log.
type index struct {
	file *os.File
	mmap gommap.MMap
	size uint64
}

// newIndex is a function that creates a new instance of the index struct, which
// represents an index file that tracks the offset and position of messages
// in a log.
//
// The function takes a file pointer and a Config struct as input parameters
// and returns a pointer to an index struct and an error.
// The Config struct contains configuration parameters for the log,
// including the maximum index file size.
func newIndex(f *os.File, c Config) (*index, error) {
	idx := &index{
		file: f,
	}
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}
	idx.size = uint64(fi.Size())
	if err = os.Truncate(
		f.Name(), int64(c.Segment.MaxIndexBytes),
	); err != nil {
		return nil, err
	}
	if idx.mmap, err = gommap.Map(
		idx.file.Fd(),
		gommap.PROT_READ|gommap.PROT_WRITE,
		gommap.MAP_SHARED,
	); err != nil {
		return nil, err
	}
	return idx, nil
}

// Close releases the resources held by the index.
// It synchronizes any changes to the memory-mapped file and truncates it to
// the size of the last valid entry, and then closes the file.
//
// Returns any error encountered during these operations.
func (i *index) Close() error {
	if err := i.mmap.Sync(gommap.MS_SYNC); err != nil {
		return err
	}
	if err := i.file.Sync(); err != nil {
		return err
	}
	if err := i.file.Truncate(int64(i.size)); err != nil {
		return err
	}
	return i.file.Close()
}

// Read reads the index entry at the given position in the index file.
// If in is -1, the last entry is read.
// Returns the offset and position of the entry and an error, if any.
// If the index is empty or the given position is out of bounds, returns io.EOF.
func (i *index) Read(in int64) (out uint32, pos uint64, err error) {
	if i.size == 0 {
		return 0, 0, io.EOF
	}
	if in == -1 {
		out = uint32((i.size / entWidth) - 1)
	} else {
		out = uint32(in)
	}
	pos = uint64(out) * entWidth
	if i.size < pos+entWidth {
		return 0, 0, io.EOF
	}
	out = enc.Uint32(i.mmap[pos : pos+offWidth])
	pos = enc.Uint64(i.mmap[pos+offWidth : pos+entWidth])
	return out, pos, nil
}

// Write writes the given offset and position to the index file's memory map.
// If the memory map does not have enough space for the new entry,
// an io.EOF error is returned.
func (i *index) Write(off uint32, pos uint64) error {
	if uint64(len(i.mmap)) < i.size+entWidth {
		return io.EOF
	}
	enc.PutUint32(i.mmap[i.size:i.size+offWidth], off)
	enc.PutUint64(i.mmap[i.size+offWidth:i.size+entWidth], pos)
	i.size += uint64(entWidth)
	return nil
}

// Name returns the name of file used for index
func (i *index) Name() string {
	return i.file.Name()
}
