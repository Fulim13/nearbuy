package client

import (
	orderpb "github.com/Fulim13/nearbuy/gen/golang/v1/order"
	"github.com/Fulim13/nearbuy/pkg/grpx"
	"google.golang.org/grpc"
)

type OrderClient struct {
	orderpb.OrderClient
	conn *grpc.ClientConn
}

func NewOrderClient(target string) (*OrderClient, error) {
	conn, err := grpx.Dial(target)
	if err != nil {
		return nil, err
	}
	return &OrderClient{OrderClient: orderpb.NewOrderClient(conn), conn: conn}, nil
}

func (c *OrderClient) Close() error {
	return c.conn.Close()
}
