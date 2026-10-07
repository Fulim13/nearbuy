package grpx

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// roundRobin tells gRPC to spread requests across every address the resolver
// returns. Without it a channel picks ONE address and keeps it for its lifetime.
const roundRobin = `{"loadBalancingConfig":[{"round_robin":{}}]}`

// The "dns:///" prefix is load-bearing. It selects gRPC's DNS resolver, which
// re-resolves and keeps a subconnection per returned address. Paired with a
// HEADLESS Service -- whose DNS record lists every pod IP rather than one virtual IP
func Dial(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	base := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(roundRobin),
	}
	return grpc.NewClient("dns:///"+target, append(base, opts...)...)
}
