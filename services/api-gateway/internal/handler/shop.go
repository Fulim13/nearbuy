package handler

import (
	"net/http"
	"strconv"

	"github.com/Fulim13/nearbuy/gen/golang/v1/shop"
	"github.com/Fulim13/nearbuy/services/api-gateway/internal/client"
	"github.com/labstack/echo/v5"
)

type ShopHandler struct {
	shop *client.ShopClient
}

func NewShopHandler(s *client.ShopClient) *ShopHandler { return &ShopHandler{shop: s} }

type ShopResponse struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	TypeID    int64   `json:"type_id"`
	Images    string  `json:"images"`
	Area      string  `json:"area"`
	Address   string  `json:"address"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	AvgPrice  int64   `json:"avg_price"`
	Sold      int32   `json:"sold"`
	Comments  int32   `json:"comments"`
	Score     int32   `json:"score"`
	OpenHours string  `json:"open_hours"`
}

type ShopTypeResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder int32  `json:"sort_order"`
}

type ShopListResponse struct {
	Shops []ShopResponse `json:"shops"`
	Total int64          `json:"total"`
}

func shopOut(s *shop.ShopInfo) ShopResponse {
	return ShopResponse{
		ID: s.GetId(), Name: s.GetName(), TypeID: s.GetTypeId(),
		Images: s.GetImages(), Area: s.GetArea(), Address: s.GetAddress(),
		X: s.GetX(), Y: s.GetY(), AvgPrice: s.GetAvgPrice(),
		Sold: s.GetSold(), Comments: s.GetComments(), Score: s.GetScore(),
		OpenHours: s.GetOpenHours(),
	}
}

func (h *ShopHandler) getShop(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid shop id")
	}
	resp, err := h.shop.GetShop(c.Request().Context(), &shop.GetShopRequest{Id: id})
	if err != nil {
		return grpcError(err)
	}
	return c.JSON(http.StatusOK, shopOut(resp.GetShop()))
}

// listShops takes either ?type_id= or ?name=. One endpoint, two filters
func (h *ShopHandler) listShops(c *echo.Context) error {
	typeID, _ := strconv.ParseInt(c.QueryParam("type_id"), 10, 64)
	name := c.QueryParam("name")
	if typeID <= 0 && name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "type_id or name is required")
	}
	page, _ := strconv.Atoi(c.QueryParamOr("page", "1"))
	pageSize, _ := strconv.Atoi(c.QueryParamOr("page_size", "10"))

	resp, err := h.shop.ListShops(c.Request().Context(), &shop.ListShopsRequest{
		TypeId: typeID, Name: name, Page: int32(page), PageSize: int32(pageSize),
	})
	if err != nil {
		return grpcError(err)
	}
	out := ShopListResponse{Shops: make([]ShopResponse, 0, len(resp.GetShops())), Total: resp.GetTotal()}
	for _, s := range resp.GetShops() {
		out.Shops = append(out.Shops, shopOut(s))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *ShopHandler) listShopTypes(c *echo.Context) error {
	resp, err := h.shop.ListShopTypes(c.Request().Context(), &shop.ListShopTypesRequest{})
	if err != nil {
		return grpcError(err)
	}
	out := make([]ShopTypeResponse, 0, len(resp.GetTypes()))
	for _, t := range resp.GetTypes() {
		out = append(out, ShopTypeResponse{
			ID: t.GetId(), Name: t.GetName(), Icon: t.GetIcon(), SortOrder: t.GetSortOrder(),
		})
	}
	return c.JSON(http.StatusOK, out)
}

type createShopRequest struct {
	Name      string  `json:"name"`
	TypeID    int64   `json:"type_id"`
	Images    string  `json:"images"`
	Area      string  `json:"area"`
	Address   string  `json:"address"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	AvgPrice  int64   `json:"avg_price"`
	OpenHours string  `json:"open_hours"`
}

func (h *ShopHandler) createShop(c *echo.Context) error {
	var req createShopRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.shop.CreateShop(c.Request().Context(), &shop.CreateShopRequest{
		Name: req.Name, TypeId: req.TypeID, Images: req.Images, Area: req.Area,
		Address: req.Address, X: req.X, Y: req.Y, AvgPrice: req.AvgPrice,
		OpenHours: req.OpenHours,
	})
	if err != nil {
		return grpcError(err)
	}
	return c.JSON(http.StatusCreated, shopOut(resp.GetShop()))
}

// updateShopRequest uses POINTERS so JSON-absent and JSON-null are both nil,
// which is what the proto's optional fields and sqlc's narg both want. A plain
// string field would make every PATCH overwrite every column with "".
type updateShopRequest struct {
	Name      *string `json:"name"`
	TypeID    *int64  `json:"type_id"`
	Images    *string `json:"images"`
	Area      *string `json:"area"`
	Address   *string `json:"address"`
	AvgPrice  *int64  `json:"avg_price"`
	OpenHours *string `json:"open_hours"`
}

func (h *ShopHandler) updateShop(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid shop id")
	}
	var req updateShopRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if _, err := h.shop.UpdateShop(c.Request().Context(), &shop.UpdateShopRequest{
		Id: id, Name: req.Name, TypeId: req.TypeID, Images: req.Images,
		Area: req.Area, Address: req.Address, AvgPrice: req.AvgPrice,
		OpenHours: req.OpenHours,
	}); err != nil {
		return grpcError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
