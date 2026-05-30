package cache

import (
	"context"
	"fmt"
	"time"

	"ripple-note/internal/feed"
)

const (
	LatestFeedFirstPageKey = "feed:latest:first-page"
	HotFeedFirstPageKey    = "feed:hot:first-page"
	NoteDetailKeyPrefix    = "note:detail:"
	NoteCountsKeyPrefix    = "note:counts:"
	UserProfileKeyPrefix   = "user:profile:"
)

var (
	FeedTTL    = 30 * time.Second
	DetailTTL  = 2 * time.Minute
	CountsTTL  = 1 * time.Minute
	ProfileTTL = 5 * time.Minute
)

// CacheInvalidator provides methods to invalidate specific cache entries.
type CacheInvalidator interface {
	InvalidateFeedCache(ctx context.Context)
	InvalidateNoteCache(ctx context.Context, noteID uint64)
}

type FeedCache struct {
	client  *Client
	service feed.FeedService
}

func NewFeedCache(client *Client, service feed.FeedService) *FeedCache {
	return &FeedCache{client: client, service: service}
}

func (fc *FeedCache) Latest(ctx context.Context, viewerID uint64, cursor string, limit int) (feed.FeedResult, error) {
	// Only cache the first page (no cursor) for anonymous users (viewerID == 0).
	if cursor == "" && viewerID == 0 {
		var cached feed.FeedResult
		if err := fc.client.Get(ctx, LatestFeedFirstPageKey, &cached); err == nil {
			return cached, nil
		}
	}

	result, err := fc.service.Latest(ctx, viewerID, cursor, limit)
	if err != nil {
		return result, err
	}

	if cursor == "" && viewerID == 0 {
		_ = fc.client.Set(ctx, LatestFeedFirstPageKey, result, FeedTTL)
	}

	return result, nil
}

func (fc *FeedCache) Hot(ctx context.Context, viewerID uint64, cursor string, limit int) (feed.FeedResult, error) {
	// Only cache the first page for anonymous users.
	if cursor == "" && viewerID == 0 {
		var cached feed.FeedResult
		if err := fc.client.Get(ctx, HotFeedFirstPageKey, &cached); err == nil {
			return cached, nil
		}
	}

	result, err := fc.service.Hot(ctx, viewerID, cursor, limit)
	if err != nil {
		return result, err
	}

	if cursor == "" && viewerID == 0 {
		_ = fc.client.Set(ctx, HotFeedFirstPageKey, result, FeedTTL)
	}

	return result, nil
}

func (fc *FeedCache) Following(ctx context.Context, userID uint64, cursor string, limit int) (feed.FeedResult, error) {
	// Following feed is user-specific, don't cache globally.
	return fc.service.Following(ctx, userID, cursor, limit)
}

func (fc *FeedCache) ByTag(ctx context.Context, viewerID uint64, tagName, cursor string, limit int) (feed.FeedResult, error) {
	// Tag feed has too many combinations, pass through.
	return fc.service.ByTag(ctx, viewerID, tagName, cursor, limit)
}

func NoteDetailKey(id uint64) string {
	return fmt.Sprintf("%s%d", NoteDetailKeyPrefix, id)
}

func NoteCountsKey(id uint64) string {
	return fmt.Sprintf("%s%d", NoteCountsKeyPrefix, id)
}

func UserProfileKey(id uint64) string {
	return fmt.Sprintf("%s%d", UserProfileKeyPrefix, id)
}

// InvalidateFeedCache clears the latest and hot feed first-page caches.
func (fc *FeedCache) InvalidateFeedCache(ctx context.Context) {
	_ = fc.client.Delete(ctx, LatestFeedFirstPageKey, HotFeedFirstPageKey)
}

// InvalidateNoteCache clears the detail and counts caches for a specific note.
func (fc *FeedCache) InvalidateNoteCache(ctx context.Context, noteID uint64) {
	_ = fc.client.Delete(ctx, NoteDetailKey(noteID), NoteCountsKey(noteID))
}
