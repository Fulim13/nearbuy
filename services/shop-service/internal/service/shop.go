package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Fulim13/nearbuy/pkg/cache"
	"github.com/Fulim13/nearbuy/services/shop-service/internal/repository"
	"github.com/jackc/pgx/v5"
)

const (
	shopKeyPrefix   = "cache:shop:"
	shopTypesKey    = "cache:shop-types"
	shopTTL         = 30 * time.Minute
	shopNilTTL      = 2 * time.Minute
	shopTypesTTL    = 24 * time.Hour
	maxPageSize     = 50
	defaultPageSize = 10
)

var ErrShopNotFound = fmt.Errorf("shop not found: %w", cache.ErrNotFound)

type DB interface {
	repository.DBTX
}

type ShopService struct {
	repo  *repository.Queries
	cache *cache.Cache
}

func NewShopService(db DB, c *cache.Cache) *ShopService {
	return &ShopService{
		repo:  repository.New(db),
		cache: c,
	}
}

func (s *ShopService) shopKey(id int64) string {
	return fmt.Sprintf("%s%d", shopKeyPrefix, id)
}

func (s *ShopService) GetShop(ctx context.Context, id int64) (repository.Shop, error) {
	shop, err := cache.GetOrLoad(ctx, s.cache, s.shopKey(id), shopTTL, shopNilTTL, func(ctx context.Context) (repository.Shop, error) {
		sh, err := s.repo.GetShop(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.Shop{}, cache.ErrNotFound
		}
		return sh, err
	})
	if errors.Is(err, cache.ErrNotFound) {
		return repository.Shop{}, ErrShopNotFound
	}
	return shop, err
}

func (s *ShopService) ListShopTypes(ctx context.Context) ([]repository.ShopType, error) {
	return cache.GetOrLoad(ctx, s.cache, shopTypesKey, shopTypesTTL, shopNilTTL,
		func(ctx context.Context) ([]repository.ShopType, error) {
			return s.repo.ListShopTypes(ctx)
		})
}

// ListShopsByType is NOT cached.
func (s *ShopService) ListShopsByType(ctx context.Context, typeID int64, page, pageSize int32) ([]repository.Shop, int64, error) {
	limit, offset := paginate(page, pageSize)
	shops, err := s.repo.ListShopsByType(ctx, repository.ListShopsByTypeParams{
		TypeID: typeID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountShopsByType(ctx, typeID)
	if err != nil {
		return nil, 0, err
	}
	if shops == nil {
		shops = []repository.Shop{}
	}
	return shops, total, nil
}

func (s *ShopService) SearchShopsByName(ctx context.Context, name string, page, pageSize int32) ([]repository.Shop, error) {
	limit, offset := paginate(page, pageSize)
	shops, err := s.repo.SearchShopsByName(ctx, repository.SearchShopsByNameParams{
		Name: name, Lim: limit, Off: offset,
	})
	if err != nil {
		return nil, err
	}
	if shops == nil {
		shops = []repository.Shop{}
	}
	return shops, nil
}

func (s *ShopService) CreateShop(ctx context.Context, arg repository.CreateShopParams) (repository.Shop, error) {
	shop, err := s.repo.CreateShop(ctx, arg)
	if err != nil {
		return repository.Shop{}, err
	}
	// A brand-new id may already hold a NEGATIVE cache entry: someone asked for
	// it during the two minutes before it existed. Drop it, or the shop is
	// invisible until that sentinel expires.
	// This also ensure the cache-aside pattern, ensure cache and db consistent
	// -> Create DB then delete cache
	// Cache Invalidation on Write
	if delErr := s.cache.Del(ctx, s.shopKey(shop.ID)); delErr != nil {
		slog.Warn("cache del after create", "shop_id", shop.ID, "error", delErr)
	}
	return shop, nil
}

// This also ensure the cache-aside pattern, ensure cache and db consistent
// -> Create DB then delete cache
// Cache Invalidation on Write
func (s *ShopService) UpdateShop(ctx context.Context, arg repository.UpdateShopParams) (bool, error) {
	rows, err := s.repo.UpdateShop(ctx, arg)
	if err != nil {
		return false, err
	}
	if rows == 0 {
		return false, nil
	}
	if delErr := s.cache.Del(ctx, s.shopKey(arg.ID)); delErr != nil {
		slog.Warn("cache del after update", "shop_id", arg.ID, "error", delErr)
	}
	return true, nil
}

func paginate(page, pageSize int32) (limit, offset int32) {
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	if page <= 0 {
		page = 1
	}
	return pageSize, (page - 1) * pageSize
}
