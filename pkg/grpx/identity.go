package grpx

import (
	"context"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const UserIDKey = "x-user-id"

type userIDCtxKey struct{}

func ContextWithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userIDCtxKey{}, id)
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDCtxKey{}).(int64)
	if !ok || id <= 0 {
		return 0, false
	}
	return id, true
}

func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if id, ok := UserIDFromContext(ctx); ok {
			ctx = metadata.AppendToOutgoingContext(ctx, UserIDKey, strconv.FormatInt(id, 10))
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get(UserIDKey); len(vals) > 0 {
				if id, err := strconv.ParseInt(vals[0], 10, 64); err == nil && id > 0 {
					ctx = ContextWithUserID(ctx, id)
				}
			}
		}
		return handler(ctx, req)
	}
}
