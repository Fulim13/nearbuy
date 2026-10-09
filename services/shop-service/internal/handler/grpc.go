package handler

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Fulim13/nearbuy/gen/golang/v1/shop"
	"github.com/Fulim13/nearbuy/services/shop-service/internal/repository"
	"github.com/Fulim13/nearbuy/services/shop-service/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	shop.UnimplementedShopServer
	service *service.ShopService
}

func NewGRPCHandler(s *grpc.Server, svc *service.ShopService) {
	shop.RegisterShopServer(s, &GRPCHandler{service: svc})
}

func (h *GRPCHandler) GetShop(ctx context.Context, in *shop.GetShopRequest) (*shop.GetShopResponse, error) {
	if in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be positive")
	}
	s, err := h.service.GetShop(ctx, in.GetId())
	if errors.Is(err, service.ErrShopNotFound) {
		return nil, status.Error(codes.NotFound, "shop not found")
	}
	if err != nil {
		slog.Error("get shop", "id", in.GetId(), "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &shop.GetShopResponse{Shop: toPB(s)}, nil
}

func (h *GRPCHandler) ListShops(ctx context.Context, in *shop.ListShopsRequest) (*shop.ListShopsResponse, error) {
	switch {
	case in.GetTypeId() > 0:
		shops, total, err := h.service.ListShopsByType(ctx, in.GetTypeId(), in.GetPage(), in.GetPageSize())
		if err != nil {
			slog.Error("list shops by type", "error", err)
			return nil, status.Error(codes.Internal, "internal error")
		}
		return &shop.ListShopsResponse{Shops: toPBs(shops), Total: total}, nil

	case in.GetName() != "":
		shops, err := h.service.SearchShopsByName(ctx, in.GetName(), in.GetPage(), in.GetPageSize())
		if err != nil {
			slog.Error("search shops", "error", err)
			return nil, status.Error(codes.Internal, "internal error")
		}
		return &shop.ListShopsResponse{Shops: toPBs(shops)}, nil

	default:
		return nil, status.Error(codes.InvalidArgument, "type_id or name is required")
	}
}

func (h *GRPCHandler) ListShopTypes(ctx context.Context, _ *shop.ListShopTypesRequest) (*shop.ListShopTypesResponse, error) {
	types, err := h.service.ListShopTypes(ctx)
	if err != nil {
		slog.Error("list shop types", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	out := make([]*shop.ShopType, 0, len(types))
	for _, t := range types {
		out = append(out, &shop.ShopType{
			Id: t.ID, Name: t.Name, Icon: t.Icon, SortOrder: t.SortOrder,
		})
	}
	return &shop.ListShopTypesResponse{Types: out}, nil
}

func (h *GRPCHandler) CreateShop(ctx context.Context, in *shop.CreateShopRequest) (*shop.CreateShopResponse, error) {
	if in.GetName() == "" || in.GetAddress() == "" || in.GetTypeId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "name, address and type_id are required")
	}
	s, err := h.service.CreateShop(ctx, repository.CreateShopParams{
		Name:      in.GetName(),
		TypeID:    in.GetTypeId(),
		Images:    in.GetImages(),
		Area:      strPtr(in.GetArea()),
		Address:   in.GetAddress(),
		X:         in.GetX(),
		Y:         in.GetY(),
		AvgPrice:  in.GetAvgPrice(),
		OpenHours: strPtr(in.GetOpenHours()),
	})
	if err != nil {
		slog.Error("create shop", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &shop.CreateShopResponse{Shop: toPB(s)}, nil
}

// UpdateShop passes the request's optional fields straight through. proto3
// `optional` gives *string / *int64, and sqlc's emit_pointers_for_null_types
// gives narg the same *string / *int64 -- so "absent means leave it alone"
// survives from the wire to the COALESCE with no translation layer.
func (h *GRPCHandler) UpdateShop(ctx context.Context, in *shop.UpdateShopRequest) (*shop.UpdateShopResponse, error) {
	if in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be positive")
	}
	updated, err := h.service.UpdateShop(ctx, repository.UpdateShopParams{
		ID:        in.GetId(),
		Name:      in.Name,
		TypeID:    in.TypeId,
		Images:    in.Images,
		Area:      in.Area,
		Address:   in.Address,
		AvgPrice:  in.AvgPrice,
		OpenHours: in.OpenHours,
	})
	if err != nil {
		slog.Error("update shop", "id", in.GetId(), "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	if !updated {
		return nil, status.Error(codes.NotFound, "shop not found")
	}
	return &shop.UpdateShopResponse{Updated: true}, nil
}

func toPB(s repository.Shop) *shop.ShopInfo {
	out := &shop.ShopInfo{
		Id: s.ID, Name: s.Name, TypeId: s.TypeID, Images: s.Images,
		Address: s.Address, X: s.X, Y: s.Y, AvgPrice: s.AvgPrice,
		Sold: s.Sold, Comments: s.Comments, Score: s.Score,
	}
	if s.Area != nil {
		out.Area = *s.Area
	}
	if s.OpenHours != nil {
		out.OpenHours = *s.OpenHours
	}
	return out
}

func toPBs(in []repository.Shop) []*shop.ShopInfo {
	out := make([]*shop.ShopInfo, 0, len(in))
	for _, s := range in {
		out = append(out, toPB(s))
	}
	return out
}

// strPtr turns "" into nil, so an omitted optional column stays NULL rather than
// becoming an empty string.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
