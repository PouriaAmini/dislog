package loadbalance

import (
	"strings"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

var _ base.PickerBuilder = (*Picker)(nil)

// Picker is a struct that implements the balancer.Picker interface.
// It picks a subconnection to send an RPC.
type Picker struct {
	// A mutex to synchronize access to the picker's internal state.
	mu sync.RWMutex
	// The leader subconnection
	leader balancer.SubConn
	// The list of follower subconnections
	followers []balancer.SubConn
	// The index of the current follower for the next "Consume" request.
	current uint64
}

// Build creates a new Picker based on the given buildInfo and returns it.
// Implements the base.PickerBuilder interface.
func (p *Picker) Build(buildInfo base.PickerBuildInfo) balancer.Picker {
	p.mu.Lock()
	defer p.mu.Unlock()
	var followers []balancer.SubConn
	for sc, scInfo := range buildInfo.ReadySCs {
		isLeader := scInfo.
			Address.
			Attributes.
			Value("is_leader").(bool)
		if isLeader {
			p.leader = sc
			continue
		}
		followers = append(followers, sc)
	}
	p.followers = followers
	return p
}

var _ balancer.Picker = (*Picker)(nil)

// Pick picks a subconnection using the leader-follower algorithm.
// The leader subconnection is chosen for requests containing "Produce" in
// the full method name.
// The next available follower subconnection is chosen for requests
// containing "Consume" in the full method name.
// An error is returned if no subconnections are available.
func (p *Picker) Pick(info balancer.PickInfo) (
	balancer.PickResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result balancer.PickResult
	if strings.Contains(info.FullMethodName, "Produce") ||
		len(p.followers) == 0 {
		result.SubConn = p.leader
	} else if strings.Contains(info.FullMethodName, "Consume") {
		result.SubConn = p.nextFollower()
	}
	if result.SubConn == nil {
		return result, balancer.ErrNoSubConnAvailable
	}
	return result, nil
}

// nextFollower returns the next follower subconnection based on the index of
// the current
func (p *Picker) nextFollower() balancer.SubConn {
	cur := atomic.AddUint64(&p.current, uint64(1))
	len := uint64(len(p.followers))
	idx := int(cur % len)
	return p.followers[idx]
}

// init registers the Picker builder with the grpc balancer module.
func init() {
	balancer.Register(
		base.NewBalancerBuilder(Name, &Picker{}, base.Config{}),
	)
}
