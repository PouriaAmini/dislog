// Package log provides functionality for managing and manipulating append-only
// log files.
package log

import (
	api "github.com/pouriaamini/proglog/api/v1"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Log represents a durable, sequentially appended log of records.
// It is composed of a list of segments,
// where each segment contains an index and store file.
// The segments are appended to sequentially and can be read from in order
// to reconstruct the original log. It supports appending, reading,
// truncating, and resetting the log.
type Log struct {
	mu            sync.RWMutex
	Dir           string
	Config        Config
	activeSegment *segment
	segments      []*segment
}

// NewLog creates and returns a new Log instance with the given
// directory path and configuration.
// If the configuration values for MaxStoreBytes or MaxIndexBytes are zero,
// default values of 1024 will be used.
// It also initializes the log by reading existing segment files from the
// directory and setting up new segments as needed.
// The function returns an error if there was a problem setting up the log.
func NewLog(dir string, c Config) (*Log, error) {
	if c.Segment.MaxStoreBytes == 0 {
		c.Segment.MaxStoreBytes = 1024
	}
	if c.Segment.MaxIndexBytes == 0 {
		c.Segment.MaxIndexBytes = 1024
	}
	l := &Log{
		Dir:    dir,
		Config: c,
	}
	return l, l.setup()
}

// setup initializes the log by loading all existing segments from the log directory
// and creates a new segment if none exist. It reads the offsets from the file names
// and sorts them before loading segments. It also sets up the active segment and the
// segments slice.
func (l *Log) setup() error {
	files, err := ioutil.ReadDir(l.Dir)
	if err != nil {
		return err
	}
	var baseOffsets []uint64
	for _, file := range files {
		offStr := strings.TrimSuffix(
			file.Name(),
			path.Ext(file.Name()),
		)
		off, _ := strconv.ParseUint(offStr, 10, 0)
		baseOffsets = append(baseOffsets, off)
	}
	sort.Slice(baseOffsets, func(i, j int) bool {
		return baseOffsets[i] < baseOffsets[j]
	})
	for i := 0; i < len(baseOffsets); i++ {
		if err = l.newSegment(baseOffsets[i]); err != nil {
			return err
		}
		// baseOffset contains dup for index and store so we skip
		// the dup
		i++
	}
	if l.segments == nil {
		if err = l.newSegment(l.Config.Segment.InitialOffset); err != nil {
			return err
		}
	}
	return nil
}

// Append appends a new record to the active segment of the log. If the active
// segment is full after appending the record, it creates a new segment and sets
// it as the active segment.
//
// Append returns the offset of the appended record and an error if any.
func (l *Log) Append(record *api.Record) (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	off, err := l.activeSegment.Append(record)
	if err != nil {
		return 0, err
	}
	if l.activeSegment.IsMaxed() {
		err = l.newSegment(off + 1)
	}
	return off, err
}

// Read reads and returns the record with the given offset from the log. It
// searches for the segment that contains the record with the given offset and
// returns an error if the offset is out of range or the segment is not found.
func (l *Log) Read(off uint64) (*api.Record, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var s *segment
	for _, segment := range l.segments {
		if segment.baseOffset <= off && off < segment.nextOffset {
			s = segment
			break
		}
	}
	if s == nil || s.nextOffset <= off {
		return nil, api.ErrOffsetOutOfRange{Offset: off}
	}
	return s.Read(off)
}

// Close closes all segments in the log and releases all associated resources.
// It returns an error if any of the segments fail to close.
func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, segment := range l.segments {
		if err := segment.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Remove closes and removes all segments in the log and deletes the log directory
// and its contents. It returns an error if any of the segments fail to close or
// if the directory cannot be removed.
func (l *Log) Remove() error {
	if err := l.Close(); err != nil {
		return err
	}
	return os.RemoveAll(l.Dir)
}

// Reset removes the log directory and its contents and sets up a new log with
// the initial segment offset specified in the log configuration. It returns an
// error if any error occurs while removing the directory or setting up the new
// log.
func (l *Log) Reset() error {
	if err := l.Remove(); err != nil {
		return err
	}
	return l.setup()
}

// LowestOffset returns the base offset of the first segment in the log. It is
// safe to call this method concurrently with other log methods.
func (l *Log) LowestOffset() (uint64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.segments[0].baseOffset, nil
}

// HighestOffset returns the highest offset in the log. It is safe to call this
// method concurrently with other log methods.
func (l *Log) HighestOffset() (uint64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	off := l.segments[len(l.segments)-1].nextOffset
	if off == 0 {
		return 0, nil
	}
	return off - 1, nil
}

// Truncate removes all segments in the log whose next offset is less than or
// equal to the specified lowest offset. It returns an error if any of the segments
// fail to remove.
func (l *Log) Truncate(lowest uint64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	var segments []*segment
	for _, s := range l.segments {
		if s.nextOffset <= lowest+1 {
			if err := s.Remove(); err != nil {
				return err
			}
			continue
		}
		segments = append(segments, s)
	}
	l.segments = segments
	return nil
}

// Reader returns a reader that reads all records in the log. It is safe to call
// this method concurrently with other log methods.
func (l *Log) Reader() io.Reader {
	l.mu.RLock()
	defer l.mu.RUnlock()
	readers := make([]io.Reader, len(l.segments))
	for i, segment := range l.segments {
		readers[i] = &originReader{segment.store, 0}
	}
	return io.MultiReader(readers...)
}

// originReader is an implementation of the io.Reader interface that provides a
// read-only view into the log. It is used to construct a MultiReader from all
// the store objects in the segments slice.
type originReader struct {
	*store
	off int64
}

// Read reads up to len(p) bytes into p from the underlying store object, starting
// at the current offset, and advances the offset by the number of bytes read. It
// returns the number of bytes read and any error encountered.
func (o *originReader) Read(p []byte) (int, error) {
	n, err := o.ReadAt(p, o.off)
	o.off += int64(n)
	return n, err
}

// newSegment creates a new segment with the specified base offset and adds it
// to the log segments. It sets the new segment as the active segment of the log.
// It returns an error if any error occurs while creating the new segment.
func (l *Log) newSegment(off uint64) error {
	s, err := newSegment(l.Dir, off, l.Config)
	if err != nil {
		return err
	}
	l.segments = append(l.segments, s)
	l.activeSegment = s
	return nil
}
