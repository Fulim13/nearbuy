package client

import (
	"github.com/Fulim13/nearbuy/gen/golang/v1/user"
	"github.com/Fulim13/nearbuy/pkg/grpx"
	"google.golang.org/grpc"
)

type UserClient struct {
	user.UserClient
	conn *grpc.ClientConn
}

// NewUserClient dials user-service.
// grpcx.Dial adds "dns:///" and the round_robin policy.
func NewUserClient(target string) (*UserClient, error) {
	conn, err := grpx.Dial(target)
	if err != nil {
		return nil, err
	}
	return &UserClient{UserClient: user.NewUserClient(conn), conn: conn}, nil
}

func (c *UserClient) Close() error { return c.conn.Close() }
