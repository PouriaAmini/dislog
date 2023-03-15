package log

import (
	"bufio"
	"encoding/binary"
	"os"
	"sync"
)

var (
	// enc is a global variable of type binary.ByteOrder that specifies the byte
	// order to use when encoding binary data. It is set to binary.BigEndian by
	// default.
	enc = binary.BigEndian
)

const (
	// lenWidth is a constant that represents the width (in bytes) of the length
	// prefix used to encode the length of data in the log file.
	lenWidth = 8
)

// store is a type that represents an append-only log file store.
//
// It embeds an *os.File object, a mutex (mu), a buffered writer (buf),
// and a size field that represents the current size of the file.
// The store type provides methods for appending data to the file,
// reading data from the file, and closing the file.
type store struct {
	*os.File
	mu   sync.Mutex
	buf  *bufio.Writer
	size uint64
}

// newStore is a function that creates a new store object for the given file.
//
// It takes an *os.File object as an argument and returns a new store object
// and any errors encountered during initialization.
func newStore(f *os.File) (*store, error) {
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}
	size := uint64(fi.Size())
	return &store{
		File: f,
		size: size,
		buf:  bufio.NewWriter(f),
	}, nil
}

// Append is a method of the store type that appends a byte slice to the end
// of the log file.
//
// It takes a byte slice p as an argument and returns the  number of bytes
// written to the file, the position of the appended data within the file,
// and any errors encountered during the write operation.
func (s *store) Append(p []byte) (n uint64, pos uint64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pos = s.size
	if err := binary.Write(s.buf, enc, uint64(len(p))); err != nil {
		return 0, 0, err
	}
	w, err := s.buf.Write(p)
	if err != nil {
		return 0, 0, err
	}
	w += lenWidth
	s.size += uint64(w)
	return uint64(w), pos, nil
}

// Read is a method of the store type that reads a byte slice from the log file
// at the given position.
//
// It takes the position within the file as an argument and returns the byte
// slice and any errors encountered during the read operation.
func (s *store) Read(pos uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.buf.Flush(); err != nil {
		return nil, err
	}
	size := make([]byte, lenWidth)
	if _, err := s.File.ReadAt(size, int64(pos)); err != nil {
		return nil, err
	}
	b := make([]byte, enc.Uint64(size))
	if _, err := s.File.ReadAt(b, int64(pos+lenWidth)); err != nil {
		return nil, err
	}
	return b, nil
}

// ReadAt is a method of the store type that reads a byte slice from the log
// file at the given offset.
//
// It takes a byte slice p and an offset within the file as arguments and
// returns the number of bytes read and any errors encountered during the
// read operation.
func (s *store) ReadAt(p []byte, off int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.buf.Flush(); err != nil {
		return 0, err
	}
	return s.File.ReadAt(p, off)
}

// Close is a method of the store type that closes the log file and releases
// any associated resources.
//
// It returns any errors encountered during the close operation.
func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.buf.Flush()
	if err != nil {
		return err
	}
	return s.File.Close()
}
