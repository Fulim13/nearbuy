package grpx

import (
	"context"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	UserIDKey   = "x-user-id"
	UserRoleKey = "x-user-role"
)

// RoleAdmin is the only role a service checks for; everyone else is a plain user.
const RoleAdmin = "admin"

type userIDCtxKey struct{}
type userRoleCtxKey struct{}

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

func ContextWithUserRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, userRoleCtxKey{}, role)
}

// UserRoleFromContext returns "" when no role was propagated, which every
// check must treat as "not an admin".
func UserRoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(userRoleCtxKey{}).(string)
	return role
}

func IsAdmin(ctx context.Context) bool {
	return UserRoleFromContext(ctx) == RoleAdmin
}

func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if id, ok := UserIDFromContext(ctx); ok {
			ctx = metadata.AppendToOutgoingContext(ctx, UserIDKey, strconv.FormatInt(id, 10))
		}
		if role := UserRoleFromContext(ctx); role != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, UserRoleKey, role)
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
			if vals := md.Get(UserRoleKey); len(vals) > 0 {
				ctx = ContextWithUserRole(ctx, vals[0])
			}
		}
		return handler(ctx, req)
	}
}
