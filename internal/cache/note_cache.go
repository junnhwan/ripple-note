package cache

import (
	"context"
	"time"

	"ripple-note/internal/note"
)

type JSONCache interface {
	Get(ctx context.Context, key string, dest any) error
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

type NoteService interface {
	Publish(ctx context.Context, input note.PublishInput, authorID uint64) (note.NoteDTO, error)
	Detail(ctx context.Context, noteID uint64, viewerID uint64) (note.NoteDTO, error)
	MyNotes(ctx context.Context, authorID uint64, limit, offset int) (note.NoteListDTO, error)
	PublicNotes(ctx context.Context, authorID uint64, limit, offset int) (note.NoteListDTO, error)
	DeleteOwn(ctx context.Context, noteID uint64, authorID uint64) (bool, error)
}

type NoteCountsSnapshot struct {
	LikesCount     uint64 `json:"likes_count"`
	FavoritesCount uint64 `json:"favorites_count"`
	CommentsCount  uint64 `json:"comments_count"`
}

type NoteServiceCache struct {
	cache  JSONCache
	source NoteService
}

func NewNoteServiceCache(cache JSONCache, source NoteService) *NoteServiceCache {
	return &NoteServiceCache{cache: cache, source: source}
}

func (c *NoteServiceCache) Publish(ctx context.Context, input note.PublishInput, authorID uint64) (note.NoteDTO, error) {
	return c.source.Publish(ctx, input, authorID)
}

func (c *NoteServiceCache) Detail(ctx context.Context, noteID uint64, viewerID uint64) (note.NoteDTO, error) {
	if c == nil || c.cache == nil || c.source == nil || viewerID != 0 {
		return c.source.Detail(ctx, noteID, viewerID)
	}

	var cached note.NoteDTO
	if err := c.cache.Get(ctx, NoteDetailKey(noteID), &cached); err == nil {
		return cached, nil
	}

	detail, err := c.source.Detail(ctx, noteID, viewerID)
	if err != nil {
		return detail, err
	}
	if detail.Status == note.StatusPublished && detail.Visibility == note.VisibilityPublic {
		_ = c.cache.Set(ctx, NoteDetailKey(noteID), detail, DetailTTL)
		_ = c.cache.Set(ctx, NoteCountsKey(noteID), NoteCountsSnapshot{
			LikesCount:     detail.LikesCount,
			FavoritesCount: detail.FavoritesCount,
			CommentsCount:  detail.CommentsCount,
		}, CountsTTL)
	}
	return detail, nil
}

func (c *NoteServiceCache) MyNotes(ctx context.Context, authorID uint64, limit, offset int) (note.NoteListDTO, error) {
	return c.source.MyNotes(ctx, authorID, limit, offset)
}

func (c *NoteServiceCache) PublicNotes(ctx context.Context, authorID uint64, limit, offset int) (note.NoteListDTO, error) {
	return c.source.PublicNotes(ctx, authorID, limit, offset)
}

func (c *NoteServiceCache) DeleteOwn(ctx context.Context, noteID uint64, authorID uint64) (bool, error) {
	return c.source.DeleteOwn(ctx, noteID, authorID)
}
