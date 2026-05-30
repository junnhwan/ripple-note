package cache

import (
	"context"
	"fmt"
	"time"

	"ripple-note/internal/feed"
)

const (
	LatestFeedFirstPageKey = "feed:latest:first-page"
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

type FeedCache struct {
	client  *Client
	service *feed.Service
}

func NewFeedCache(client *Client, service *feed.Service) *FeedCache {
	return &FeedCache{client: client, service: service}
}

func (fc *FeedCache) Latest(ctx context.Context, cursor string, limit int) (feed.FeedResult, error) {
	// Only cache the first page (no cursor).
	if cursor == "" {
		var cached feed.FeedResult
		if err := fc.client.Get(ctx, LatestFeedFirstPageKey, &cached); err == nil {
			return cached, nil
		}
	}

	result, err := fc.service.Latest(ctx, cursor, limit)
	if err != nil {
		return result, err
	}

	if cursor == "" {
		_ = fc.client.Set(ctx, LatestFeedFirstPageKey, result, FeedTTL)
	}

	return result, nil
}

func (fc *FeedCache) Hot(ctx context.Context, cursor string, limit int) (feed.FeedResult, error) {
	return fc.service.Hot(ctx, cursor, limit)
}

func (fc *FeedCache) Following(ctx context.Context, userID uint64, cursor string, limit int) (feed.FeedResult, error) {
	return fc.service.Following(ctx, userID, cursor, limit)
}

func (fc *FeedCache) ByTag(ctx context.Context, tagName, cursor string, limit int) (feed.FeedResult, error) {
	return fc.service.ByTag(ctx, tagName, cursor, limit)
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

func (fc *FeedCache) InvalidateFeedCache(ctx context.Context) {
	_ = fc.client.Delete(ctx, LatestFeedFirstPageKey)
}

func (fc *FeedCache) InvalidateNoteCache(ctx context.Context, noteID uint64) {
	_ = fc.client.Delete(ctx, NoteDetailKey(noteID), NoteCountsKey(noteID))
}
