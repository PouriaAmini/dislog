package log

import "github.com/hashicorp/raft"

// Config defines the configuration for the log
type Config struct {
	Raft struct {
		raft.Config
		StreamLayer *StreamLayer
		Bootstrap   bool
	}
	// Segment contains the configuration options for the log segments
	Segment struct {
		// MaxStoreBytes specifies the maximum size of a segment file for
		// storing log entries
		MaxStoreBytes uint64
		// MaxIndexBytes specifies the maximum size of a segment file for
		// storing index entries
		MaxIndexBytes uint64
		// InitialOffset specifies the initial offset value for the log
		InitialOffset uint64
	}
}
