package handler

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Fulim13/nearbuy/gen/golang/v1/user"
	"github.com/Fulim13/nearbuy/pkg/grpx"
	"github.com/Fulim13/nearbuy/services/user-service/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	user.UnimplementedUserServer
	service *service.UserService
	follows *service.FollowService
}

func NewGRPCHandler(s *grpc.Server, svc *service.UserService, follows *service.FollowService) {
	user.RegisterUserServer(s, &GRPCHandler{service: svc, follows: follows})
}

func (h *GRPCHandler) GetUser(ctx context.Context, in *user.GetUserRequest) (*user.GetUserResponse, error) {
	if in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be positive")
	}

	u, err := h.service.GetUser(ctx, in.GetId())
	callerID, authed := grpx.UserIDFromContext(ctx)
	slog.Info("GetUser", "target_id", in.GetId(), "caller_id", callerID, "authenticated", authed)
	if errors.Is(err, service.ErrUserNotFound) {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	if err != nil {
		slog.Error("get user", "id", in.GetId(), "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &user.GetUserResponse{User: &user.PublicUser{
		Id:       u.ID,
		NickName: u.NickName,
		Icon:     u.Icon,
	}}, nil
}

func (h *GRPCHandler) GetOrCreateUserByPhone(ctx context.Context, in *user.GetOrCreateUserByPhoneRequest) (*user.GetOrCreateUserByPhoneResponse, error) {
	if in.GetPhone() == "" {
		return nil, status.Error(codes.InvalidArgument, "phone is required")
	}

	u, created, err := h.service.GetOrCreateUserByPhone(ctx, in.GetPhone(), in.GetDefaultNickName())
	if err != nil {
		slog.Error("get or create user", "phone", in.GetPhone(), "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &user.GetOrCreateUserByPhoneResponse{
		User:    &user.PublicUser{Id: u.ID, NickName: u.NickName, Icon: u.Icon},
		Created: created,
		Role:    string(u.Role),
	}, nil
}

func (h *GRPCHandler) Follow(ctx context.Context, in *user.FollowRequest) (*user.FollowResponse, error) {
	if err := h.follows.Follow(ctx, in.GetUserId(), in.GetTargetId()); err != nil {
		return nil, followError(err)
	}
	return &user.FollowResponse{}, nil
}

func (h *GRPCHandler) Unfollow(ctx context.Context, in *user.FollowRequest) (*user.FollowResponse, error) {
	if err := h.follows.Unfollow(ctx, in.GetUserId(), in.GetTargetId()); err != nil {
		return nil, followError(err)
	}
	return &user.FollowResponse{}, nil
}

func (h *GRPCHandler) IsFollowing(ctx context.Context, in *user.FollowRequest) (*user.IsFollowingResponse, error) {
	ok, err := h.follows.IsFollowing(ctx, in.GetUserId(), in.GetTargetId())
	if err != nil {
		slog.Error("is following", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &user.IsFollowingResponse{Following: ok}, nil
}

// CommonFollowing hydrates the intersection into PublicUsers, because an id list is useless to a client that then has to make N more calls.
func (h *GRPCHandler) CommonFollowing(ctx context.Context, in *user.CommonFollowingRequest) (*user.CommonFollowingResponse, error) {
	ids, err := h.follows.CommonFollowing(ctx, in.GetUserId(), in.GetOtherId())
	if err != nil {
		slog.Error("common following", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	out := make([]*user.PublicUser, 0, len(ids))
	for _, id := range ids {
		u, err := h.service.GetUser(ctx, id)
		if err != nil {
			// A follow row pointing at a deleted user: skip it rather than fail
			// the whole response.
			slog.Warn("common following: user missing", "id", id)
			continue
		}
		out = append(out, &user.PublicUser{Id: u.ID, NickName: u.NickName, Icon: u.Icon})
	}
	return &user.CommonFollowingResponse{Users: out}, nil
}

func (h *GRPCHandler) FollowCounts(ctx context.Context, in *user.FollowCountsRequest) (*user.FollowCountsResponse, error) {
	following, followers, err := h.follows.Counts(ctx, in.GetUserId())
	if err != nil {
		slog.Error("follow counts", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &user.FollowCountsResponse{Following: following, Followers: followers}, nil
}

func followError(err error) error {
	switch {
	case errors.Is(err, service.ErrSelfFollow):
		return status.Error(codes.InvalidArgument, "cannot follow yourself")
	case errors.Is(err, service.ErrInvalidUser):
		return status.Error(codes.InvalidArgument, "invalid user id")
	default:
		slog.Error("follow operation failed", "error", err)
		return status.Error(codes.Internal, "internal error")
	}
}
