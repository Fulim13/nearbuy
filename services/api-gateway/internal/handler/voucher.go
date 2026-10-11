package handler

import (
	"net/http"
	"strconv"

	"github.com/Fulim13/nearbuy/gen/golang/v1/voucher"
	"github.com/Fulim13/nearbuy/services/api-gateway/internal/client"
	mw "github.com/Fulim13/nearbuy/services/api-gateway/internal/middleware"
	"github.com/labstack/echo/v5"
)

type VoucherHandler struct {
	voucher *client.VoucherClient
}

func NewVoucherHandler(v *client.VoucherClient) *VoucherHandler {
	return &VoucherHandler{voucher: v}
}

// VoucherResponse is the gateway's wire type. Pointers for the seckill fields so
// a normal voucher omits them entirely rather than reporting stock 0.
type VoucherResponse struct {
	ID          int64   `json:"id"`
	ShopID      int64   `json:"shop_id"`
	Title       string  `json:"title"`
	SubTitle    string  `json:"sub_title"`
	Rules       string  `json:"rules"`
	PayValue    int64   `json:"pay_value"`
	ActualValue int64   `json:"actual_value"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	Stock       *int32  `json:"stock,omitempty"`
	BeginTime   *string `json:"begin_time,omitempty"`
	EndTime     *string `json:"end_time,omitempty"`
}

func voucherOut(v *voucher.VoucherInfo) VoucherResponse {
	return VoucherResponse{
		ID: v.GetId(), ShopID: v.GetShopId(), Title: v.GetTitle(),
		SubTitle: v.GetSubTitle(), Rules: v.GetRules(),
		PayValue: v.GetPayValue(), ActualValue: v.GetActualValue(),
		Type: v.GetType(), Status: v.GetStatus(),
		// Pointer fields, not getters: GetStock() returns 0 for an absent
		// field, which would report a normal voucher as a sold-out seckill.
		Stock: v.Stock, BeginTime: v.BeginTime, EndTime: v.EndTime,
	}
}

func (h *VoucherHandler) getVoucher(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid voucher id")
	}
	resp, err := h.voucher.GetVoucher(c.Request().Context(), &voucher.GetVoucherRequest{Id: id})
	if err != nil {
		return grpcError(err)
	}
	return c.JSON(http.StatusOK, voucherOut(resp.GetVoucher()))
}

func (h *VoucherHandler) listByShop(c *echo.Context) error {
	shopID, err := strconv.ParseInt(c.Param("shopId"), 10, 64)
	if err != nil || shopID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid shop id")
	}
	resp, err := h.voucher.ListVouchersByShop(c.Request().Context(),
		&voucher.ListVouchersByShopRequest{ShopId: shopID})
	if err != nil {
		return grpcError(err)
	}
	out := make([]VoucherResponse, 0, len(resp.GetVouchers()))
	for _, v := range resp.GetVouchers() {
		out = append(out, voucherOut(v))
	}
	return c.JSON(http.StatusOK, out)
}

type createVoucherRequest struct {
	ShopID      int64  `json:"shop_id"`
	Title       string `json:"title"`
	SubTitle    string `json:"sub_title"`
	Rules       string `json:"rules"`
	PayValue    int64  `json:"pay_value"`
	ActualValue int64  `json:"actual_value"`
}

func (r createVoucherRequest) toPB() *voucher.CreateVoucherRequest {
	return &voucher.CreateVoucherRequest{
		ShopId: r.ShopID, Title: r.Title, SubTitle: r.SubTitle, Rules: r.Rules,
		PayValue: r.PayValue, ActualValue: r.ActualValue,
	}
}

func (h *VoucherHandler) createVoucher(c *echo.Context) error {
	var req createVoucherRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.voucher.CreateVoucher(c.Request().Context(), req.toPB())
	if err != nil {
		return grpcError(err)
	}
	return c.JSON(http.StatusCreated, voucherOut(resp.GetVoucher()))
}

type createSeckillRequest struct {
	createVoucherRequest
	Stock     int32  `json:"stock"`
	BeginTime string `json:"begin_time"`
	EndTime   string `json:"end_time"`
}

func (h *VoucherHandler) createSeckillVoucher(c *echo.Context) error {
	var req createSeckillRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.voucher.CreateSeckillVoucher(c.Request().Context(),
		&voucher.CreateSeckillVoucherRequest{
			Voucher:   req.createVoucherRequest.toPB(),
			Stock:     req.Stock,
			BeginTime: req.BeginTime,
			EndTime:   req.EndTime,
		})
	if err != nil {
		return grpcError(err)
	}
	return c.JSON(http.StatusCreated, voucherOut(resp.GetVoucher()))
}

// seckillResponse is what a caller gets from POST /api/voucher-order/seckill/:id.
//
// 202 Accepted, not 201 Created: no order row exists yet. The id is a PROMISE
// that one will, redeemable by polling GET /api/voucher-order/:id once
// order-service exists.
type seckillResponse struct {
	OrderID int64  `json:"order_id"`
	Status  string `json:"status"`
}

// seckillStatus maps the admission code onto an HTTP status.
//
// All the rejections are 4xx, and which 4xx matters:
//   - out of stock  -> 409 Conflict: the request was fine, the world changed
//   - already yours -> 409 Conflict: same, and idempotent-ish from the client's view
//   - not started   -> 425 Too Early, the status literally meant for this
//   - finished      -> 410 Gone, the resource existed and no longer does
//   - unknown       -> 404
//
// None of them is a 500. A sold-out seckill is the system working.
var seckillStatus = map[voucher.SeckillCode]int{
	voucher.SeckillCode_SECKILL_CODE_OUT_OF_STOCK:    http.StatusConflict,
	voucher.SeckillCode_SECKILL_CODE_ALREADY_ORDERED: http.StatusConflict,
	voucher.SeckillCode_SECKILL_CODE_NOT_STARTED:     http.StatusTooEarly,
	voucher.SeckillCode_SECKILL_CODE_FINISHED:        http.StatusGone,
	voucher.SeckillCode_SECKILL_CODE_UNKNOWN_VOUCHER: http.StatusNotFound,
}

func (h *VoucherHandler) seckill(c *echo.Context) error {
	voucherID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || voucherID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid voucher id")
	}
	u, ok := mw.SessionUser(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	resp, err := h.voucher.Seckill(c.Request().Context(), &voucher.SeckillRequest{
		VoucherId: voucherID,
		UserId:    u.ID,
	})
	if err != nil {
		return grpcError(err)
	}

	if !resp.GetAdmitted() {
		code, ok := seckillStatus[resp.GetCode()]
		if !ok {
			code = http.StatusConflict
		}
		return echo.NewHTTPError(code, resp.GetReason())
	}

	return c.JSON(http.StatusAccepted, seckillResponse{
		OrderID: resp.GetOrderId(),
		Status:  "pending",
	})
}
