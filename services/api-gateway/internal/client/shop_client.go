package client

import (
	"github.com/Fulim13/nearbuy/gen/golang/v1/shop"
	"github.com/Fulim13/nearbuy/pkg/grpx"
	"google.golang.org/grpc"
)

type ShopClient struct {
	shop.ShopClient
	conn *grpc.ClientConn
}

func NewShopClient(target string) (*ShopClient, error) {
	conn, err := grpx.Dial(target)
	if err != nil {
		return nil, err
	}
	return &ShopClient{ShopClient: shop.NewShopClient(conn), conn: conn}, nil
}

func (c *ShopClient) Close() error {
	return c.conn.Close()
}
