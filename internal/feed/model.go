package feed

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const DefaultLimit = 20
const MaxLimit = 50

type Cursor struct {
	PublishedAt *time.Time `json:"pat,omitempty"`
	HotScore    *float64   `json:"hs,omitempty"`
	ID          uint64     `json:"id"`
}

func EncodeCursor(c Cursor) (string, error) {
	if c.ID == 0 && c.PublishedAt == nil && c.HotScore == nil {
		return "", nil
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal cursor: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func DecodeCursor(encoded string) (Cursor, error) {
	if encoded == "" {
		return Cursor{}, nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Cursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	var c Cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return Cursor{}, fmt.Errorf("unmarshal cursor: %w", err)
	}
	return c, nil
}

type FeedItem struct {
	ID             uint64     `json:"id"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	AuthorID       uint64     `json:"author_id"`
	AuthorNickname string     `json:"author_nickname"`
	AuthorAvatar   string     `json:"author_avatar"`
	LikesCount     uint64     `json:"likes_count"`
	FavoritesCount uint64     `json:"favorites_count"`
	CommentsCount  uint64     `json:"comments_count"`
	Tags           []string   `json:"tags"`
	ImageURLs      []string   `json:"image_urls"`
	PublishedAt    *time.Time `json:"published_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type FeedResult struct {
	Items      []FeedItem `json:"items"`
	NextCursor string     `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
}

func parseLimit(limit int) int {
	if limit <= 0 || limit > MaxLimit {
		return DefaultLimit
	}
	return limit
}

func applyLatestCursor(query *gorm.DB, c Cursor) *gorm.DB {
	if c.ID == 0 || c.PublishedAt == nil {
		return query
	}
	return query.Where(
		"(published_at, id) < (?, ?)",
		*c.PublishedAt, c.ID,
	)
}

func applyHotCursor(query *gorm.DB, c Cursor) *gorm.DB {
	if c.ID == 0 || c.HotScore == nil {
		return query
	}
	return query.Where(
		"(hot_score, id) < (?, ?)",
		*c.HotScore, c.ID,
	)
}
