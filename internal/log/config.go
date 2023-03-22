package log

import "github.com/hashicorp/raft"

// Config defines the configuration for the log
type Config struct {
	// Raft struct represents a Raft node with configuration options
	Raft struct {
		// Embedded type Config for Raft configuration
		raft.Config
		// BindAddr is the domain name to bind the Raft node to
		BindAddr string
		// StreamLayer is the stream layer used by the Raft node
		StreamLayer *StreamLayer
		// Bootstrap checks whether the node should bootstrap a new cluster
		Bootstrap bool
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
