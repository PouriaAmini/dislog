// Package agent implements the Proglog agent. It provides a distributed log
// service using the Raft consensus algorithm for log replication and a
// gRPC-based API for clients to interact with the distributed log.
package agent

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/hashicorp/raft"
	"io"
	"net"
	"sync"
	"time"

	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/pouriaamini/proglog/internal/auth"
	"github.com/pouriaamini/proglog/internal/discovery"
	"github.com/pouriaamini/proglog/internal/log"
	"github.com/pouriaamini/proglog/internal/server"
)

// Agent is a struct that implements the Proglog agent.
// It provides a distributed log service using the Raft consensus algorithm
// for log replication and a gRPC-based API for clients to interact with the
// distributed log.
type Agent struct {
	Config Config

	mux        cmux.CMux
	log        *log.DistributedLog
	server     *grpc.Server
	membership *discovery.Membership

	shutdown     bool
	shutdowns    chan struct{}
	shutdownLock sync.Mutex
}

// Config is a struct that represents the configuration options for the agent.
type Config struct {
	// ServerTLSConfig is the server's TLS configuration.
	ServerTLSConfig *tls.Config
	// PeerTLSConfig is the peer's TLS configuration.
	PeerTLSConfig *tls.Config
	// DataDir is the directory where the log data will be stored.
	DataDir string
	// BindAddr is the address the server will listen on.
	BindAddr string
	// RPCPort is the port the server will listen on.
	RPCPort int
	// NodeName is the unique identifier for the node.
	NodeName string
	// StartJoinAddrs is the initial list of nodes to join.
	StartJoinAddrs []string
	// ACLModelFile is the path to the model file for ACL.
	ACLModelFile string
	// ACLPolicyFile is the path to the policy file for ACL.
	ACLPolicyFile string
	// Bootstrap is a flag to bootstrap the Raft cluster.
	Bootstrap bool
}

// RPCAddr returns the address of the RPC endpoint.
func (c Config) RPCAddr() (string, error) {
	host, _, err := net.SplitHostPort(c.BindAddr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", host, c.RPCPort), nil
}

// New creates a new instance of the agent with the given configuration.
func New(config Config) (*Agent, error) {
	a := &Agent{
		Config:    config,
		shutdowns: make(chan struct{}),
	}
	setup := []func() error{
		a.setupLogger,
		a.setupMux,
		a.setupLog,
		a.setupServer,
		a.setupMembership,
	}
	for _, fn := range setup {
		if err := fn(); err != nil {
			return nil, err
		}
	}
	go a.serve()
	return a, nil
}

// setupLogger sets up the logger for the agent.
func (a *Agent) setupLogger() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	zap.ReplaceGlobals(logger)
	return nil
}

// setupMux sets up the multiplexer for the agent.
func (a *Agent) setupMux() error {
	rpcAddr := fmt.Sprintf(
		":%d",
		a.Config.RPCPort,
	)
	ln, err := net.Listen("tcp", rpcAddr)
	if err != nil {
		return err
	}
	a.mux = cmux.New(ln)
	return nil
}

// setupLog function sets up the logging configuration for the agent by
// creating a new instance of distributed log and initializing it with the
// agent's configuration. It also waits for a leader if bootstrap is set to
// true in the configuration.
func (a *Agent) setupLog() error {
	raftLn := a.mux.Match(func(reader io.Reader) bool {
		b := make([]byte, 1)
		if _, err := reader.Read(b); err != nil {
			return false
		}
		return bytes.Compare(b, []byte{byte(log.RaftRPC)}) == 0
	})
	logConfig := log.Config{}
	logConfig.Raft.StreamLayer = log.NewStreamLayer(
		raftLn,
		a.Config.ServerTLSConfig,
		a.Config.PeerTLSConfig,
	)
	logConfig.Raft.LocalID = raft.ServerID(a.Config.NodeName)
	logConfig.Raft.Bootstrap = a.Config.Bootstrap
	var err error
	a.log, err = log.NewDistributedLog(
		a.Config.DataDir,
		logConfig,
	)
	if err != nil {
		return err
	}
	if a.Config.Bootstrap {
		err = a.log.WaitForLeader(3 * time.Second)
	}
	return err
}

// setupServer function sets up the gRPC server for the agent by creating a
// new instance of gRPC server and initializing it with the agent's
// configuration.
func (a *Agent) setupServer() error {
	authorizer := auth.New(
		a.Config.ACLModelFile,
		a.Config.ACLPolicyFile,
	)
	serverConfig := &server.Config{
		CommitLog:   a.log,
		Authorizer:  authorizer,
		GetServerer: a.log,
	}
	var opts []grpc.ServerOption
	if a.Config.ServerTLSConfig != nil {
		creds := credentials.NewTLS(a.Config.ServerTLSConfig)
		opts = append(opts, grpc.Creds(creds))
	}
	var err error
	a.server, err = server.NewGRPCServer(serverConfig, opts...)
	if err != nil {
		return err
	}
	grpcLn := a.mux.Match(cmux.Any())
	go func() {
		if err := a.server.Serve(grpcLn); err != nil {
			_ = a.Shutdown()
		}
	}()
	return err
}

// setupMembership function sets up the membership for the agent by creating
// a new instance of discovery and initializing it with the agent's
// configuration.
func (a *Agent) setupMembership() error {
	rpcAddr, err := a.Config.RPCAddr()
	if err != nil {
		return err
	}
	a.membership, err = discovery.New(a.log, discovery.Config{
		NodeName: a.Config.NodeName,
		BindAddr: a.Config.BindAddr,
		Tags: map[string]string{
			"rpc_addr": rpcAddr,
		},
		StartJoinAddrs: a.Config.StartJoinAddrs,
	})
	return err
}

// Shutdown function is responsible for shutting down the agent by closing
// all the connections and shutting down the servers.
func (a *Agent) Shutdown() error {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()
	if a.shutdown {
		return nil
	}
	a.shutdown = true
	close(a.shutdowns)

	shutdown := []func() error{
		a.membership.Leave,
		func() error {
			a.server.GracefulStop()
			return nil
		},
		a.log.Close,
	}
	for _, fn := range shutdown {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) serve() error {
	if err := a.mux.Serve(); err != nil {
		_ = a.Shutdown()
		return err
	}
	return nil
}
