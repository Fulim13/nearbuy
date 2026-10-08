package handler

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Fulim13/nearbuy/gen/golang/v1/user"
	"github.com/Fulim13/nearbuy/services/user-service/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	user.UnimplementedUserServer
	service *service.UserService
}

func NewGRPCHandler(s *grpc.Server, svc *service.UserService) {
	user.RegisterUserServer(s, &GRPCHandler{service: svc})
}

func (h *GRPCHandler) GetUser(ctx context.Context, in *user.GetUserRequest) (*user.GetUserResponse, error) {
	if in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be positive")
	}

	u, err := h.service.GetUser(ctx, in.GetId())
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
	}, nil
}
