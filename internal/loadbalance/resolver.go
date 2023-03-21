// Package loadbalance provides custom implementations of a resolver and a
// balancer for gRPC clients to connect to distributed
// systems that require special load balancing algorithms.
package loadbalance

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"

	api "github.com/pouriaamini/proglog/api/v1"
)

// Resolver implements the resolver.Resolver interface.
// It resolves the service endpoint addresses and their attributes (
// isLeader, for example) using the get_servers RPC of a proglog server.
type Resolver struct {
	// A mutex to synchronize access to the resolver's internal state
	mu sync.Mutex
	// The clientConn to update with the resolved service
	clientConn resolver.ClientConn
	// The underlying gRPC client connection to the proglog
	resolverConn *grpc.ClientConn
	// The parsed service configuration
	serviceConfig *serviceconfig.ParseResult
	// A logger instance
	logger *zap.Logger
}

var _ resolver.Builder = (*Resolver)(nil)

// Build builds and returns a new Resolver struct for the given target,
// clientConn, and resolver.BuildOptions.
func (r *Resolver) Build(
	target resolver.Target,
	cc resolver.ClientConn,
	opts resolver.BuildOptions,
) (resolver.Resolver, error) {
	r.logger = zap.L().Named("resolver")
	r.clientConn = cc
	var dialOpts []grpc.DialOption
	if opts.DialCreds != nil {
		dialOpts = append(
			dialOpts,
			grpc.WithTransportCredentials(opts.DialCreds),
		)
	}
	r.serviceConfig = r.clientConn.ParseServiceConfig(
		fmt.Sprintf(`{"loadBalancingConfig":[{"%s":{}}]}`, Name),
	)
	var err error
	r.resolverConn, err = grpc.Dial(target.Endpoint, dialOpts...)
	if err != nil {
		return nil, err
	}
	r.ResolveNow(resolver.ResolveNowOptions{})
	return r, nil
}

// Name is the name of the proglog load balancing mechanism.
const Name = "proglog"

// Scheme returns the name of the load balancing scheme.
func (r *Resolver) Scheme() string {
	return Name
}

// init registers the Resolver with the grpc resolver module.
func init() {
	resolver.Register(&Resolver{})
}

var _ resolver.Resolver = (*Resolver)(nil)

// ResolveNow resolves the addresses of the endpoints and their attributes
// using the get_servers RPC of the proglog server.
func (r *Resolver) ResolveNow(resolver.ResolveNowOptions) {
	r.mu.Lock()
	defer r.mu.Unlock()
	client := api.NewLogClient(r.resolverConn)
	// get cluster and then set on cc attributes
	ctx := context.Background()
	res, err := client.GetServers(ctx, &api.GetServersRequest{})
	if err != nil {
		r.logger.Error(
			"failed to resolve server",
			zap.Error(err),
		)
		return
	}
	var addrs []resolver.Address
	for _, server := range res.Servers {
		addrs = append(addrs, resolver.Address{
			Addr: server.RpcAddr,
			Attributes: attributes.New(
				"is_leader",
				server.IsLeader,
			),
		})
	}
	r.clientConn.UpdateState(resolver.State{
		Addresses:     addrs,
		ServiceConfig: r.serviceConfig,
	})
}

// Close closes the connection to the proglog server.
func (r *Resolver) Close() {
	if err := r.resolverConn.Close(); err != nil {
		r.logger.Error(
			"failed to close conn",
			zap.Error(err),
		)
	}
}
