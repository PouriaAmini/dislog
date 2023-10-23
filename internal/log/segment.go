package log

import (
	"fmt"
	"os"
	"path"

	"google.golang.org/protobuf/proto"

	api "github.com/pouriaamini/proglog/api/v1"
)

// A Segment represents a single storage unit in the log.
// It contains a set of message records, a corresponding index, and metadata
// such as the base offset and file sizes.
type segment struct {
	store                  *store
	index                  *index
	baseOffset, nextOffset uint64
	config                 Config
}

// NewSegment creates a new segment with the given base offset and config.
// The base offset is the starting offset for this segment, and the config
// determines the segment's properties such as maximum size and retention time.
// The segment will be created with an index file and data file, and will be
// ready to append records to.
//
// If an existing segment with the same base offset and directory already exists,
// an error will be returned.
func newSegment(dir string, baseOffset uint64, c Config) (*segment, error) {
	s := &segment{
		baseOffset: baseOffset,
		config:     c,
	}
	var err error
	storeFile, err := os.OpenFile(
		path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".store")),
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, err
	}
	if s.store, err = newStore(storeFile); err != nil {
		return nil, err
	}
	indexFile, err := os.OpenFile(
		path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".index")),
		os.O_RDWR|os.O_CREATE,
		0644,
	)
	if err != nil {
		return nil, err
	}
	if s.index, err = newIndex(indexFile, c); err != nil {
		return nil, err
	}
	if off, _, err := s.index.Read(-1); err != nil {
		s.nextOffset = baseOffset
	} else {
		s.nextOffset = baseOffset + uint64(off) + 1
	}
	return s, nil
}

// Append appends a key-value pair to the log.
// It encodes the pair into a binary format and writes it to the active
// segment file. If the active segment is too large or too old,
// it closes the segment and creates a new one.
// It returns the offset and position of the record in the log.
func (s *segment) Append(record *api.Record) (offset uint64, err error) {
	cur := s.nextOffset
	record.Offset = cur
	p, err := proto.Marshal(record)
	if err != nil {
		return 0, err
	}
	_, pos, err := s.store.Append(p)
	if err != nil {
		return 0, err
	}
	if err = s.index.Write(
		// index offsets are relative to base offset
		uint32(s.nextOffset-uint64(s.baseOffset)),
		pos,
	); err != nil {
		return 0, err
	}
	s.nextOffset++
	return cur, nil
}

// Read reads a log entry from the segment at the given index. If index is -1,
// it reads the last entry. The returned `out` is the offset of the log entry in
// the store, and `pos` is the position of the log entry in the segment. If the
// requested entry is not found, an `io.EOF` error is returned.
func (s *segment) Read(off uint64) (*api.Record, error) {
	_, pos, err := s.index.Read(int64(off - s.baseOffset))
	if err != nil {
		return nil, err
	}
	p, err := s.store.Read(pos)
	if err != nil {
		return nil, err
	}
	record := &api.Record{}
	err = proto.Unmarshal(p, record)
	return record, err
}

// IsMaxed checks if the current segment has exceeded its maximum
// allowed size limit. It returns true if either the size of the store or index
// file has reached its maximum size, and false otherwise.
func (s *segment) IsMaxed() bool {
	return s.store.size >= s.config.Segment.MaxStoreBytes ||
		s.index.size >= s.config.Segment.MaxIndexBytes ||
		s.index.IsMaxed()
}

// Remove closes the segment and removes its associated store and index files.
func (s *segment) Remove() error {
	if err := s.Close(); err != nil {
		return err
	}
	if err := os.Remove(s.index.Name()); err != nil {
		return err
	}
	if err := os.Remove(s.store.Name()); err != nil {
		return err
	}
	return nil
}

// Close closes the segment by closing its associated store and index files.
func (s *segment) Close() error {
	if err := s.index.Close(); err != nil {
		return err
	}
	if err := s.store.Close(); err != nil {
		return err
	}
	return nil
}

// nearestMultiple returns the nearest multiple of k that is greater than or
// equal to j.
func nearestMultiple(j, k uint64) uint64 {
	if j >= 0 {
		return (j / k) * k
	}
	return ((j - k + 1) / k) * k
}
