package client

import (
	"github.com/Fulim13/nearbuy/gen/golang/v1/voucher"
	"github.com/Fulim13/nearbuy/pkg/grpx"
	"google.golang.org/grpc"
)

type VoucherClient struct {
	voucher.VoucherClient
	conn *grpc.ClientConn
}

func NewVoucherClient(target string) (*VoucherClient, error) {
	conn, err := grpx.Dial(target)
	if err != nil {
		return nil, err
	}
	return &VoucherClient{VoucherClient: voucher.NewVoucherClient(conn), conn: conn}, nil
}

func (c *VoucherClient) Close() error {
	return c.conn.Close()
}
