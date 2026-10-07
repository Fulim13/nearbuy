package handler

import (
	"net/http"
	"strconv"

	"github.com/Fulim13/nearbuy/gen/golang/v1/user"
	"github.com/Fulim13/nearbuy/services/api-gateway/internal/client"
	"github.com/labstack/echo/v5"
)

type UserHandler struct {
	user *client.UserClient
}

func NewUserHandler(user *client.UserClient) *UserHandler {
	return &UserHandler{user: user}
}

type PublicUserResponse struct {
	ID       int64  `json:"id"`
	NickName string `json:"nickname"`
	Icon     string `json:"icon"`
}

func (h *UserHandler) getUser(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	// Request context, so a client disconnect cancels the gRPC call too.
	resp, err := h.user.GetUser(c.Request().Context(), &user.GetUserRequest{Id: id})
	if err != nil {
		return grpcError(err)
	}

	u := resp.GetUser()
	return c.JSON(http.StatusOK, PublicUserResponse{
		ID:       u.GetId(),
		NickName: u.GetNickName(),
		Icon:     u.GetIcon(),
	})
}
