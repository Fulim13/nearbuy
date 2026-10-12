package handler

import (
	"net/http"
	"strconv"

	orderpb "github.com/Fulim13/nearbuy/gen/golang/v1/order"
	"github.com/Fulim13/nearbuy/services/api-gateway/internal/client"
	mw "github.com/Fulim13/nearbuy/services/api-gateway/internal/middleware"
	"github.com/labstack/echo/v5"
)

type OrderHandler struct {
	order *client.OrderClient
}

func NewOrderHandler(o *client.OrderClient) *OrderHandler {
	return &OrderHandler{order: o}
}

// OrderResponse is the gateway's wire type. int64 as a Go int64 rather than
// protobuf's JSON string, protobuf renders int64 as a
// quoted string and a browser client would have to parse it back.
type OrderResponse struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	VoucherID  int64   `json:"voucher_id"`
	PayValue   int64   `json:"pay_value"`
	PayType    string  `json:"pay_type"`
	Status     string  `json:"status"`
	CreateTime string  `json:"create_time"`
	PayTime    *string `json:"pay_time,omitempty"`
	UseTime    *string `json:"use_time,omitempty"`
	RefundTime *string `json:"refund_time,omitempty"`
}

func orderOut(o *orderpb.OrderInfo) OrderResponse {
	return OrderResponse{
		ID: o.GetId(), UserID: o.GetUserId(), VoucherID: o.GetVoucherId(),
		PayValue: o.GetPayValue(), PayType: o.GetPayType(), Status: o.GetStatus(),
		CreateTime: o.GetCreateTime(),
		// Pointers, not getters: GetPayTime() returns "" for an unpaid order,
		// and "" is not the same claim as "this field does not apply yet".
		PayTime: o.PayTime, UseTime: o.UseTime, RefundTime: o.RefundTime,
	}
}

// getOrder redeems the 202 that the seckill handed out.
//
// 404 is a NORMAL answer here, not an error to be smoothed over: between the
// 202 and the Saga's first branch committing, the id is a promise with no row
// behind it. The client polls. What the gateway must NOT do is invent a
// {"status":"pending"} body for a 200 -- that makes "the Saga has not reached
// order_db yet" indistinguishable from "this id was never issued".
func (h *OrderHandler) getOrder(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid order id")
	}
	u, ok := mw.SessionUser(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	resp, err := h.order.GetOrder(c.Request().Context(), &orderpb.GetOrderRequest{Id: id})
	if err != nil {
		return grpcError(err)
	}
	// An order is private. order-service does not filter by caller -- it answers
	// what it is asked -- so the gateway owns this check. 404 rather than 403:
	// telling a stranger "that order exists but is not yours" leaks the fact
	// that the id is live.
	if resp.GetOrder().GetUserId() != u.ID {
		return echo.NewHTTPError(http.StatusNotFound, "order not found")
	}
	return c.JSON(http.StatusOK, orderOut(resp.GetOrder()))
}

// listMyOrders takes the user from the SESSION, never from a query parameter.
// with /api/voucher-orders?user_id=7, any user could change 7 to another
// number and read someone else's order history.
func (h *OrderHandler) listMyOrders(c *echo.Context) error {
	u, ok := mw.SessionUser(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	resp, err := h.order.ListOrdersByUser(c.Request().Context(), &orderpb.ListOrdersByUserRequest{
		UserId: u.ID, Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		return grpcError(err)
	}
	out := make([]OrderResponse, 0, len(resp.GetOrders()))
	for _, o := range resp.GetOrders() {
		out = append(out, orderOut(o))
	}
	return c.JSON(http.StatusOK, out)
}

// pay starts Saga S2 and waits for it, so this handler is the one place in the
// gateway that holds a request open across a distributed transaction.
//
// POST, not PATCH: paying is not a partial update of an order, it is an action
// with effects in two other databases.
func (h *OrderHandler) pay(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid order id")
	}
	u, ok := mw.SessionUser(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	resp, err := h.order.PayOrder(c.Request().Context(), &orderpb.PayOrderRequest{
		OrderId: id,
		// From the SESSION. A user_id in the body would let anyone spend anyone
		// else's balance.
		UserId: u.ID,
	})
	if err != nil {
		return grpcError(err)
	}
	return c.JSON(http.StatusOK, orderOut(resp.GetOrder()))
}

