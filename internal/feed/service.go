package feed

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"ripple-note/internal/note"
)

var ErrTagNotFound = errors.New("tag not found")

type FollowProvider interface {
	FollowingIDs(ctx context.Context, userID uint64) ([]uint64, error)
}

type AuthorProvider interface {
	FindByID(ctx context.Context, id uint64) (note.AuthorDTO, error)
}

type Service struct {
	repo    *Repository
	notes   *note.Repository
	authors AuthorProvider
	follows FollowProvider
	db      *gorm.DB
}

func NewService(db *gorm.DB, repo *Repository, notes *note.Repository, authors AuthorProvider, follows FollowProvider) *Service {
	return &Service{db: db, repo: repo, notes: notes, authors: authors, follows: follows}
}

func (s *Service) Latest(ctx context.Context, encodedCursor string, limit int) (FeedResult, error) {
	limit = parseLimit(limit)
	cursor, err := DecodeCursor(encodedCursor)
	if err != nil {
		return FeedResult{}, err
	}

	notes, err := s.repo.ListLatest(ctx, cursor, limit)
	if err != nil {
		return FeedResult{}, err
	}

	return s.buildResult(ctx, notes, limit, func(n *note.Note) Cursor {
		return Cursor{PublishedAt: n.PublishedAt, ID: n.ID}
	})
}

func (s *Service) Hot(ctx context.Context, encodedCursor string, limit int) (FeedResult, error) {
	limit = parseLimit(limit)
	cursor, err := DecodeCursor(encodedCursor)
	if err != nil {
		return FeedResult{}, err
	}

	notes, err := s.repo.ListHot(ctx, cursor, limit)
	if err != nil {
		return FeedResult{}, err
	}

	return s.buildResult(ctx, notes, limit, func(n *note.Note) Cursor {
		return Cursor{HotScore: &n.HotScore, ID: n.ID}
	})
}

func (s *Service) Following(ctx context.Context, userID uint64, encodedCursor string, limit int) (FeedResult, error) {
	limit = parseLimit(limit)
	cursor, err := DecodeCursor(encodedCursor)
	if err != nil {
		return FeedResult{}, err
	}

	ids, err := s.follows.FollowingIDs(ctx, userID)
	if err != nil {
		return FeedResult{}, err
	}

	notes, err := s.repo.ListByAuthorIDs(ctx, ids, cursor, limit)
	if err != nil {
		return FeedResult{}, err
	}

	return s.buildResult(ctx, notes, limit, func(n *note.Note) Cursor {
		return Cursor{PublishedAt: n.PublishedAt, ID: n.ID}
	})
}

func (s *Service) ByTag(ctx context.Context, tagName, encodedCursor string, limit int) (FeedResult, error) {
	limit = parseLimit(limit)
	cursor, err := DecodeCursor(encodedCursor)
	if err != nil {
		return FeedResult{}, err
	}

	tag, err := s.repo.FindTagByName(ctx, tagName)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return FeedResult{Items: []FeedItem{}, HasMore: false}, nil
		}
		return FeedResult{}, err
	}

	notes, err := s.repo.ListByTagID(ctx, tag.ID, cursor, limit)
	if err != nil {
		return FeedResult{}, err
	}

	return s.buildResult(ctx, notes, limit, func(n *note.Note) Cursor {
		return Cursor{PublishedAt: n.PublishedAt, ID: n.ID}
	})
}

func (s *Service) buildResult(ctx context.Context, notes []*note.Note, limit int, cursorFn func(*note.Note) Cursor) (FeedResult, error) {
	hasMore := len(notes) > limit
	if hasMore {
		notes = notes[:limit]
	}

	items := make([]FeedItem, 0, len(notes))
	for _, n := range notes {
		item, err := s.toFeedItem(ctx, n)
		if err != nil {
			return FeedResult{}, err
		}
		items = append(items, *item)
	}

	var nextCursor string
	if hasMore && len(notes) > 0 {
		c := cursorFn(notes[len(notes)-1])
		encoded, err := EncodeCursor(c)
		if err != nil {
			return FeedResult{}, err
		}
		nextCursor = encoded
	}

	return FeedResult{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *Service) toFeedItem(ctx context.Context, n *note.Note) (*FeedItem, error) {
	author, err := s.authors.FindByID(ctx, n.AuthorID)
	if err != nil {
		return nil, err
	}

	tags, _ := s.notes.FindTagNamesByNoteID(ctx, n.ID)
	images, _ := s.notes.FindImagesByNoteID(ctx, n.ID)

	var imageURLs []string
	for _, img := range images {
		imageURLs = append(imageURLs, img.URL)
	}

	return &FeedItem{
		ID:             n.ID,
		Title:          n.Title,
		Body:           n.Body,
		AuthorID:       n.AuthorID,
		AuthorNickname: author.Nickname,
		AuthorAvatar:   author.AvatarURL,
		LikesCount:     n.LikesCount,
		FavoritesCount: n.FavoritesCount,
		CommentsCount:  n.CommentsCount,
		Tags:           tags,
		ImageURLs:      imageURLs,
		PublishedAt:    n.PublishedAt,
		CreatedAt:      n.CreatedAt,
	}, nil
}
