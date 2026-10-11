package handler

import (
	"net/http"
	"strconv"

	"github.com/Fulim13/nearbuy/pkg/ratelimit"
	"github.com/Fulim13/nearbuy/pkg/util"
	mw "github.com/Fulim13/nearbuy/services/api-gateway/internal/middleware"
	"github.com/Fulim13/nearbuy/services/api-gateway/internal/session"
	"github.com/labstack/echo/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Routes(e *echo.Echo, limiter *ratelimit.Limiter, sessions *session.Store, user *UserHandler, auth *AuthHandler, shop *ShopHandler, follow *FollowHandler, voucher *VoucherHandler) {
	e.GET("/healthz", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	perIP := mw.Tier{
		Name:       "per-ip",
		Key:        func(c *echo.Context) string { return "rl:ip:" + c.RealIP() },
		Capacity:   100,
		RatePerSec: 100.0 / 60.0,
	}

	// This sets up a rate limit for each logged-in user, on each API endpoint. Each user gets their own small allowance per endpoint
	// For anonymous user, perIP already block it
	// Measured: five authenticated requests across five different ids:
	// key:    rl:user:8:GET:/api/user/:id
	// tokens: 5.0056666666666665   <- capacity 10, five spent
	// keys:   1  (one, not five)
	perUserRoute := mw.Tier{
		Name: "per-user-route",
		Key: func(c *echo.Context) string {
			u, ok := mw.SessionUser(c)
			if !ok {
				return ""
			}
			return "rl:user:" + strconv.FormatInt(u.ID, 10) + ":" + c.Request().Method + ":" + c.Path()
		},
		Capacity:   10,
		RatePerSec: 10.0 / 60.0,
	}

	api := e.Group("/api",
		mw.RateLimit(limiter, perIP),
		mw.Session(sessions),
		mw.RateLimit(limiter, perUserRoute),
	)

	// Public
	api.GET("/ping", ping)
	api.GET("/user/:id", user.getUser)
	api.POST("/user/code", auth.sendCode)
	api.POST("/user/login", auth.login)

	// Authenticated
	authed := api.Group("", mw.RequireAuth())
	authed.GET("/user/me", auth.getMe)
	authed.POST("/user/logout", auth.logout)

	// Public
	api.GET("/shops/:id", shop.getShop)
	api.GET("/shops", shop.listShops)
	api.GET("/shop-types", shop.listShopTypes)

	// Authenticated
	authed.POST("/shops", shop.createShop)
	authed.PATCH("/shops/:id", shop.updateShop)

	// Public: anyone may see how many followers a user has.
	api.GET("/users/:id/follow-counts", follow.followCounts)

	// The actor is always the session user, never a path or body field.
	authed.PUT("/users/:id/follow", follow.follow)
	authed.DELETE("/users/:id/follow", follow.unfollow)
	authed.GET("/users/:id/follow", follow.isFollowing)
	authed.GET("/users/:id/common-following", follow.commonFollowing)

	// Public.
	api.GET("/vouchers/:id", voucher.getVoucher)
	api.GET("/shops/:shopId/vouchers", voucher.listByShop)

	// Authenticated.
	authed.POST("/vouchers", voucher.createVoucher)
	authed.POST("/vouchers/seckill", voucher.createSeckillVoucher)
}

func ping(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"message": "pong",
		"pod":     util.Env("POD_NAME", "unknown"),
	})
}

func grpcError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
	switch st.Code() {
	case codes.NotFound:
		return echo.NewHTTPError(http.StatusNotFound, st.Message())
	case codes.InvalidArgument:
		return echo.NewHTTPError(http.StatusBadRequest, st.Message())
	case codes.PermissionDenied:
		return echo.NewHTTPError(http.StatusForbidden, st.Message())
	case codes.Unauthenticated:
		return echo.NewHTTPError(http.StatusUnauthorized, st.Message())
	case codes.DeadlineExceeded:
		return echo.NewHTTPError(http.StatusGatewayTimeout, "upstream timeout")
	case codes.Unavailable:
		return echo.NewHTTPError(http.StatusServiceUnavailable, "upstream unavailable")
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
