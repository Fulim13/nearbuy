package handler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Fulim13/nearbuy/gen/golang/v1/voucher"
	"github.com/Fulim13/nearbuy/services/voucher-service/internal/repository"
	"github.com/Fulim13/nearbuy/services/voucher-service/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	voucher.UnimplementedVoucherServer
	service *service.VoucherService
}

func NewGRPCHandler(s *grpc.Server, svc *service.VoucherService) {
	voucher.RegisterVoucherServer(s, &GRPCHandler{service: svc})
}

func (h *GRPCHandler) GetVoucher(ctx context.Context, in *voucher.GetVoucherRequest) (*voucher.GetVoucherResponse, error) {
	if in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be positive")
	}
	v, err := h.service.GetVoucher(ctx, in.GetId())
	if errors.Is(err, service.ErrVoucherNotFound) {
		return nil, status.Error(codes.NotFound, "voucher not found")
	}
	if err != nil {
		slog.Error("get voucher", "id", in.GetId(), "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &voucher.GetVoucherResponse{Voucher: rowToPB(v)}, nil
}

func (h *GRPCHandler) ListVouchersByShop(ctx context.Context, in *voucher.ListVouchersByShopRequest) (*voucher.ListVouchersByShopResponse, error) {
	if in.GetShopId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "shop_id must be positive")
	}
	rows, err := h.service.ListVouchersByShop(ctx, in.GetShopId())
	if err != nil {
		slog.Error("list vouchers", "shop_id", in.GetShopId(), "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	out := make([]*voucher.VoucherInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, listRowToPB(r))
	}
	return &voucher.ListVouchersByShopResponse{Vouchers: out}, nil
}

func (h *GRPCHandler) CreateVoucher(ctx context.Context, in *voucher.CreateVoucherRequest) (*voucher.CreateVoucherResponse, error) {
	if err := validateCreate(in); err != nil {
		return nil, err
	}
	v, err := h.service.CreateVoucher(ctx, repository.CreateVoucherParams{
		ShopID: in.GetShopId(), Title: in.GetTitle(), SubTitle: in.GetSubTitle(),
		Rules: in.GetRules(), PayValue: in.GetPayValue(), ActualValue: in.GetActualValue(),
	})
	if err != nil {
		slog.Error("create voucher", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &voucher.CreateVoucherResponse{Voucher: voucherToPB(v, nil, nil, nil)}, nil
}

func (h *GRPCHandler) CreateSeckillVoucher(ctx context.Context, in *voucher.CreateSeckillVoucherRequest) (*voucher.CreateVoucherResponse, error) {
	base := in.GetVoucher()
	if err := validateCreate(base); err != nil {
		return nil, err
	}
	if in.GetStock() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "stock must be positive")
	}
	begin, err := time.Parse(time.RFC3339, in.GetBeginTime())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "begin_time must be RFC3339")
	}
	end, err := time.Parse(time.RFC3339, in.GetEndTime())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "end_time must be RFC3339")
	}

	v, err := h.service.CreateSeckillVoucher(ctx, repository.CreateSeckillVoucherRowParams{
		ShopID: base.GetShopId(), Title: base.GetTitle(), SubTitle: base.GetSubTitle(),
		Rules: base.GetRules(), PayValue: base.GetPayValue(), ActualValue: base.GetActualValue(),
	}, in.GetStock(), begin, end)
	if errors.Is(err, service.ErrInvalidWindow) {
		return nil, status.Error(codes.InvalidArgument, "end_time must be after begin_time")
	}
	if err != nil {
		slog.Error("create seckill voucher", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	stock := in.GetStock()
	bs, es := begin.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)
	return &voucher.CreateVoucherResponse{Voucher: voucherToPB(v, &stock, &bs, &es)}, nil
}

func (h *GRPCHandler) NextOrderID(ctx context.Context, _ *voucher.NextOrderIDRequest) (*voucher.NextOrderIDResponse, error) {
	id, err := h.service.NextOrderID(ctx)
	if err != nil {
		slog.Error("next order id", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &voucher.NextOrderIDResponse{OrderId: id}, nil
}

func validateCreate(in *voucher.CreateVoucherRequest) error {
	switch {
	case in == nil:
		return status.Error(codes.InvalidArgument, "voucher is required")
	case in.GetShopId() <= 0:
		return status.Error(codes.InvalidArgument, "shop_id must be positive")
	case in.GetTitle() == "":
		return status.Error(codes.InvalidArgument, "title is required")
	case in.GetPayValue() < 0 || in.GetActualValue() < 0:
		return status.Error(codes.InvalidArgument, "amounts must not be negative")
	}
	return nil
}

func voucherToPB(v repository.Voucher, stock *int32, begin, end *string) *voucher.VoucherInfo {
	return &voucher.VoucherInfo{
		Id: v.ID, ShopId: v.ShopID, Title: v.Title, SubTitle: v.SubTitle,
		Rules: v.Rules, PayValue: v.PayValue, ActualValue: v.ActualValue,
		Type: string(v.Type), Status: string(v.Status),
		Stock: stock, BeginTime: begin, EndTime: end,
	}
}

func rowToPB(r repository.GetVoucherRow) *voucher.VoucherInfo {
	return &voucher.VoucherInfo{
		Id: r.ID, ShopId: r.ShopID, Title: r.Title, SubTitle: r.SubTitle,
		Rules: r.Rules, PayValue: r.PayValue, ActualValue: r.ActualValue,
		Type: string(r.Type), Status: string(r.Status),
		Stock: r.Stock, BeginTime: rfc3339(r.BeginTime), EndTime: rfc3339(r.EndTime),
	}
}

func listRowToPB(r repository.ListVouchersByShopRow) *voucher.VoucherInfo {
	return &voucher.VoucherInfo{
		Id: r.ID, ShopId: r.ShopID, Title: r.Title, SubTitle: r.SubTitle,
		Rules: r.Rules, PayValue: r.PayValue, ActualValue: r.ActualValue,
		Type: string(r.Type), Status: string(r.Status),
		Stock: r.Stock, BeginTime: rfc3339(r.BeginTime), EndTime: rfc3339(r.EndTime),
	}
}

// rfc3339 keeps nil as nil, so a normal voucher's absent window stays absent
// rather than becoming the zero time.
//
// Always .UTC(): pgx returns timestamptz in the SERVER's local zone, so without
// this the same instant is rendered "2026-10-09T16:05:24Z" by the create
// response (which formats the parsed input) and "2026-10-10T00:05:24+08:00" by
// the read path. Both are correct and the inconsistency is still a bug -- a
// client diffing two responses sees a change that did not happen.
func rfc3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
