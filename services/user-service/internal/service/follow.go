package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/Fulim13/nearbuy/services/user-service/internal/repository"
	"github.com/redis/go-redis/v9"
)

var (
	ErrSelfFollow  = errors.New("cannot follow yourself")
	ErrInvalidUser = errors.New("invalid user id")
)

func followsKey(userID int64) string {
	return "follows:" + strconv.FormatInt(userID, 10)
}

type FollowService struct {
	repo *repository.Queries
	rdb  *redis.Client
}

func NewFollowService(db DB, rdb *redis.Client) *FollowService {
	return &FollowService{repo: repository.New(db), rdb: rdb}
}

// Follow records the relation and mirrors it into Redis.
//
// Postgres is written FIRST and is the source of truth. If the Redis mirror write
// fails we log and carry on: a missing SET member degrades CommonFollowing to its
// SQL fallback, which is correct but slower. The reverse order would be worse --
// a Redis entry for a relation Postgres never accepted is a lie with no way to
// detect it.
func (s *FollowService) Follow(ctx context.Context, userID, targetID int64) error {
	if userID <= 0 || targetID <= 0 {
		return ErrInvalidUser
	}
	if userID == targetID {
		return ErrSelfFollow
	}

	if _, err := s.repo.Follow(ctx, repository.FollowParams{
		UserID:       userID,
		FollowUserID: targetID,
	}); err != nil {
		return fmt.Errorf("insert follow: %w", err)
	}

	if err := s.rdb.SAdd(ctx, followsKey(userID), targetID).Err(); err != nil {
		slog.Warn("redis follows mirror write failed", "user_id", userID, "target_id", targetID, "error", err)
	}
	return nil
}

func (s *FollowService) Unfollow(ctx context.Context, userID, targetID int64) error {
	if userID <= 0 || targetID <= 0 {
		return ErrInvalidUser
	}
	if _, err := s.repo.Unfollow(ctx, repository.UnfollowParams{
		UserID:       userID,
		FollowUserID: targetID,
	}); err != nil {
		return fmt.Errorf("delete follow: %w", err)
	}
	if err := s.rdb.SRem(ctx, followsKey(userID), targetID).Err(); err != nil {
		slog.Warn("redis follows mirror remove failed", "user_id", userID, "target_id", targetID, "error", err)
	}
	return nil
}

func (s *FollowService) IsFollowing(ctx context.Context, userID, targetID int64) (bool, error) {
	return s.repo.IsFollowing(ctx, repository.IsFollowingParams{
		UserID:       userID,
		FollowUserID: targetID,
	})
}

// CommonFollowing returns the users both a and b follow.
//
// The Redis path is one SINTER: Redis computes the intersection server-side and
// returns only the answer. The SQL path is a self-join.
func (s *FollowService) CommonFollowing(ctx context.Context, a, b int64) ([]int64, error) {
	keyA, keyB := followsKey(a), followsKey(b)

	existsA, err := s.rdb.Exists(ctx, keyA).Result()
	if err == nil {
		var existsB int64
		existsB, err = s.rdb.Exists(ctx, keyB).Result()
		if err == nil && existsA == 1 && existsB == 1 {
			members, sErr := s.rdb.SInter(ctx, keyA, keyB).Result()
			if sErr == nil {
				return parseIDs(members), nil
			}
			err = sErr
		}
	}
	// Two different reasons to be here, and only one is a problem:
	//   err != nil   -> Redis is broken. Worth a warning.
	//   both absent  -> the mirror is simply cold (new user, or a FLUSHALL).
	//                   Expected, not an error, so it is logged at Debug.
	if err != nil {
		slog.Warn("redis SINTER unavailable, falling back to sql", "error", err)
	} else {
		slog.Info("follows mirror cold, using sql", "a", a, "b", b)
	}

	return s.repo.CommonFollowing(ctx, repository.CommonFollowingParams{A: a, B: b})
}

// RebuildMirror repopulates follows:{userID} from Postgres.
func (s *FollowService) RebuildMirror(ctx context.Context, userID int64) (int, error) {
	ids, err := s.repo.ListFollowing(ctx, userID)
	if err != nil {
		return 0, err
	}
	key := followsKey(userID)
	if len(ids) == 0 {
		return 0, s.rdb.Del(ctx, key).Err()
	}

	members := make([]any, 0, len(ids))
	for _, id := range ids {
		members = append(members, id)
	}
	// Replace, not merge: DEL then SADD in a pipeline so a stale member cannot survive a rebuild.
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, key)
	pipe.SAdd(ctx, key, members...)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (s *FollowService) Counts(ctx context.Context, userID int64) (following, followers int64, err error) {
	if following, err = s.repo.CountFollowing(ctx, userID); err != nil {
		return 0, 0, err
	}
	followers, err = s.repo.CountFollowers(ctx, userID)
	return following, followers, err
}

func parseIDs(ss []string) []int64 {
	out := make([]int64, 0, len(ss))
	for _, s := range ss {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			out = append(out, n)
		}
	}
	return out
}
