package handler

import (
	"net/http"
	"strconv"

	"github.com/Fulim13/nearbuy/gen/golang/v1/user"
	"github.com/Fulim13/nearbuy/services/api-gateway/internal/client"
	mw "github.com/Fulim13/nearbuy/services/api-gateway/internal/middleware"
	"github.com/labstack/echo/v5"
)

type FollowHandler struct {
	user *client.UserClient
}

func NewFollowHandler(u *client.UserClient) *FollowHandler {
	return &FollowHandler{user: u}
}

// targetID reads :id and, for the authenticated actor, the session -- never a
// body or query field. The actor is whoever holds the token, so letting a client
// name it would let anyone follow on anyone's behalf.
func (h *FollowHandler) actorAndTarget(c *echo.Context) (actor, target int64, err error) {
	u, ok := mw.SessionUser(c)
	if !ok {
		return 0, 0, echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	target, parseErr := strconv.ParseInt(c.Param("id"), 10, 64)
	if parseErr != nil || target <= 0 {
		return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	return u.ID, target, nil
}

func (h *FollowHandler) follow(c *echo.Context) error {
	actor, target, err := h.actorAndTarget(c)
	if err != nil {
		return err
	}
	if _, err := h.user.Follow(c.Request().Context(), &user.FollowRequest{
		UserId: actor,
		TargetId: target,
	}); err != nil {
		return grpcError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *FollowHandler) unfollow(c *echo.Context) error {
	actor, target, err := h.actorAndTarget(c)
	if err != nil {
		return err
	}
	if _, err := h.user.Unfollow(c.Request().Context(), &user.FollowRequest{
		UserId: actor,
		TargetId: target,
	}); err != nil {
		return grpcError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

type isFollowingResponse struct {
	Following bool `json:"following"`
}

func (h *FollowHandler) isFollowing(c *echo.Context) error {
	actor, target, err := h.actorAndTarget(c)
	if err != nil {
		return err
	}
	resp, err := h.user.IsFollowing(c.Request().Context(), &user.FollowRequest{
		UserId: actor,
		TargetId: target,
	})
	if err != nil {
		return grpcError(err)
	}
	return c.JSON(http.StatusOK, isFollowingResponse{Following: resp.GetFollowing()})
}

func (h *FollowHandler) commonFollowing(c *echo.Context) error {
	actor, other, err := h.actorAndTarget(c)
	if err != nil {
		return err
	}
	resp, err := h.user.CommonFollowing(c.Request().Context(), &user.CommonFollowingRequest{
		UserId: actor,
		OtherId: other,
	})
	if err != nil {
		return grpcError(err)
	}
	out := make([]PublicUserResponse, 0, len(resp.GetUsers()))
	for _, u := range resp.GetUsers() {
		out = append(out, PublicUserResponse{
			ID: u.GetId(), NickName: u.GetNickName(), Icon: u.GetIcon(),
		})
	}
	return c.JSON(http.StatusOK, out)
}

type followCountsResponse struct {
	Following int64 `json:"following"`
	Followers int64 `json:"followers"`
}

// followCounts is PUBLIC: anyone may see how many followers a user has.
func (h *FollowHandler) followCounts(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	resp, err := h.user.FollowCounts(c.Request().Context(), &user.FollowCountsRequest{UserId: id})
	if err != nil {
		return grpcError(err)
	}
	return c.JSON(http.StatusOK, followCountsResponse{
		Following: resp.GetFollowing(), Followers: resp.GetFollowers(),
	})
}
