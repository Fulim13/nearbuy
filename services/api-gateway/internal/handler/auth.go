package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"regexp"

	"github.com/Fulim13/nearbuy/gen/golang/v1/user"
	"github.com/Fulim13/nearbuy/pkg/ratelimit"
	"github.com/Fulim13/nearbuy/services/api-gateway/internal/client"
	mw "github.com/Fulim13/nearbuy/services/api-gateway/internal/middleware"
	"github.com/Fulim13/nearbuy/services/api-gateway/internal/session"
	"github.com/labstack/echo/v5"
)

// Malaysian mobile, local format: 01X + 7 digits, or 011 + 8 digits.
// 10–11 digits, so it fits a VARCHAR(11) column in db.
var phoneRe = regexp.MustCompile(`^01(?:1\d{8}|[02-46-9]\d{7})$`)

var devEchoSMS = os.Getenv("DEV_ECHO_SMS") == "true"

type AuthHandler struct {
	sessions *session.Store
	user     *client.UserClient
	limiter  *ratelimit.Limiter
}

func NewAuthHandler(s *session.Store, u *client.UserClient, l *ratelimit.Limiter) *AuthHandler {
	return &AuthHandler{sessions: s, user: u, limiter: l}
}

type sendCodeRequest struct {
	Phone string `json:"phone"`
}

type sendCodeResponse struct {
	Sent bool   `json:"sent"`
	Code string `json:"code,omitempty"` // dev only
}

func (h *AuthHandler) sendCode(c *echo.Context) error {
	var req sendCodeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if !phoneRe.MatchString(req.Phone) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid phone number")
	}

	// Tier 2: one code per phone per minute. Keyed on the PHONE, not the IP --
	const smsBurst, smsPerSec = 1, 1.0 / 60.0
	d, err := h.limiter.TokenBucket(c.Request().Context(), "rl:sms:"+req.Phone, smsBurst, smsPerSec)
	if err != nil {
		slog.Warn("sms limiter unavailable, failing open", "error", err)
	} else if !d.Allowed {
		return mw.TooManyRequests(c, d, "per-phone-sms", smsBurst)
	}

	code, err := session.NewCode()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
	if err := h.sessions.SaveCode(c.Request().Context(), req.Phone, code); err != nil {
		slog.Error("save code", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	if devEchoSMS {
		slog.Info("dev sms", "phone", req.Phone, "code", code)
		return c.JSON(http.StatusOK, sendCodeResponse{Sent: true, Code: code})
	}
	return c.JSON(http.StatusOK, sendCodeResponse{Sent: true})
}

type loginRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type loginResponse struct {
	Token string       `json:"token"`
	User  session.User `json:"user"`
}

func (h *AuthHandler) login(c *echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if !phoneRe.MatchString(req.Phone) || req.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "phone and code are required")
	}

	ctx := c.Request().Context()

	// Cap ATTEMPTS, not just code requests. The per-phone limiter in sendCode
	// stops someone asking for 1000 codes; it does nothing about someone holding
	// one code and trying all 10^6 values. 5 tries per 10 minutes per phone.
	const tryBurst, tryPerSec = 5, 5.0 / 600.0
	if d, err := h.limiter.TokenBucket(ctx, "rl:login:"+req.Phone, tryBurst, tryPerSec); err != nil {
		slog.Warn("login limiter unavailable, failing open", "error", err)
	} else if !d.Allowed {
		return mw.TooManyRequests(c, d, "per-phone-login-attempts", tryBurst)
	}

	// One code, one login: VerifyCode deletes on success.
	if err := h.sessions.VerifyCode(ctx, req.Phone, req.Code); err != nil {
		if errors.Is(err, session.ErrCodeMismatch) {
			// Same message for "wrong code" and "no code": distinguishing them
			// tells an attacker which phone numbers have a login in flight.
			return echo.NewHTTPError(http.StatusUnauthorized, "code is wrong or expired")
		}
		slog.Error("verify code", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	nick, err := session.NewNickname()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	resp, err := h.user.GetOrCreateUserByPhone(ctx, &user.GetOrCreateUserByPhoneRequest{
		Phone:           req.Phone,
		DefaultNickName: nick,
	})
	if err != nil {
		return grpcError(err)
	}

	u := session.User{
		ID:       resp.GetUser().GetId(),
		NickName: resp.GetUser().GetNickName(),
		Icon:     resp.GetUser().GetIcon(),
	}
	token, err := h.sessions.Create(ctx, u)
	if err != nil {
		slog.Error("create session", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	slog.Info("login", "user_id", u.ID, "created", resp.GetCreated())
	return c.JSON(http.StatusOK, loginResponse{Token: token, User: u})
}

func (h *AuthHandler) getMe(c *echo.Context) error {
	u, ok := mw.SessionUser(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	return c.JSON(http.StatusOK, u)
}

func (h *AuthHandler) logout(c *echo.Context) error {
	token := mw.SessionToken(c)
	if token == "" {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.sessions.Delete(c.Request().Context(), token); err != nil {
		slog.Error("delete session", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
	return c.NoContent(http.StatusNoContent)
}
