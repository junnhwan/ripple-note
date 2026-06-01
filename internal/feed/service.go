package feed

import (
	"context"
	"errors"
	"fmt"

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

type BatchAuthorProvider interface {
	FindByIDs(ctx context.Context, ids []uint64) (map[uint64]note.AuthorDTO, error)
}

type BatchViewerStateProvider interface {
	LikedNoteIDs(ctx context.Context, userID uint64, noteIDs []uint64) (map[uint64]bool, error)
	FavoritedNoteIDs(ctx context.Context, userID uint64, noteIDs []uint64) (map[uint64]bool, error)
	FollowingAuthorIDs(ctx context.Context, followerID uint64, authorIDs []uint64) (map[uint64]bool, error)
}

type Service struct {
	repo    *Repository
	notes   *note.Repository
	authors AuthorProvider
	follows FollowProvider
	viewer  ViewerStateProvider
	db      *gorm.DB
}

func NewService(db *gorm.DB, repo *Repository, notes *note.Repository, authors AuthorProvider, follows FollowProvider, viewer ViewerStateProvider) *Service {
	return &Service{db: db, repo: repo, notes: notes, authors: authors, follows: follows, viewer: viewer}
}

func (s *Service) Latest(ctx context.Context, viewerID uint64, encodedCursor string, limit int) (FeedResult, error) {
	limit = parseLimit(limit)
	cursor, err := DecodeCursor(encodedCursor)
	if err != nil {
		return FeedResult{}, err
	}

	notes, err := s.repo.ListLatest(ctx, cursor, limit)
	if err != nil {
		return FeedResult{}, err
	}

	return s.buildResult(ctx, notes, limit, viewerID, func(n *note.Note) Cursor {
		return Cursor{PublishedAt: n.PublishedAt, ID: n.ID}
	})
}

func (s *Service) Hot(ctx context.Context, viewerID uint64, encodedCursor string, limit int) (FeedResult, error) {
	limit = parseLimit(limit)
	cursor, err := DecodeCursor(encodedCursor)
	if err != nil {
		return FeedResult{}, err
	}

	notes, err := s.repo.ListHot(ctx, cursor, limit)
	if err != nil {
		return FeedResult{}, err
	}

	return s.buildResult(ctx, notes, limit, viewerID, func(n *note.Note) Cursor {
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

	return s.buildResult(ctx, notes, limit, userID, func(n *note.Note) Cursor {
		return Cursor{PublishedAt: n.PublishedAt, ID: n.ID}
	})
}

func (s *Service) ByTag(ctx context.Context, viewerID uint64, tagName, encodedCursor string, limit int) (FeedResult, error) {
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

	return s.buildResult(ctx, notes, limit, viewerID, func(n *note.Note) Cursor {
		return Cursor{PublishedAt: n.PublishedAt, ID: n.ID}
	})
}

func (s *Service) buildResult(ctx context.Context, notes []*note.Note, limit int, viewerID uint64, cursorFn func(*note.Note) Cursor) (FeedResult, error) {
	hasMore := len(notes) > limit
	if hasMore {
		notes = notes[:limit]
	}

	noteIDs, authorIDs := collectFeedIDs(notes)

	authors, err := s.findAuthors(ctx, authorIDs)
	if err != nil {
		return FeedResult{}, err
	}

	tagNames, err := s.notes.FindTagNamesByNoteIDs(ctx, noteIDs)
	if err != nil {
		return FeedResult{}, err
	}

	noteImages, err := s.notes.FindImagesByNoteIDs(ctx, noteIDs)
	if err != nil {
		return FeedResult{}, err
	}

	viewerState, err := s.findViewerState(ctx, viewerID, noteIDs, authorIDs)
	if err != nil {
		return FeedResult{}, err
	}

	items := make([]FeedItem, 0, len(notes))
	for _, n := range notes {
		author, ok := authors[n.AuthorID]
		if !ok {
			return FeedResult{}, fmt.Errorf("author %d not found", n.AuthorID)
		}
		item := toFeedItem(n, author, tagNames[n.ID], noteImages[n.ID], viewerState, viewerID)
		items = append(items, item)
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

func (s *Service) findAuthors(ctx context.Context, authorIDs []uint64) (map[uint64]note.AuthorDTO, error) {
	if len(authorIDs) == 0 {
		return map[uint64]note.AuthorDTO{}, nil
	}

	if batch, ok := s.authors.(BatchAuthorProvider); ok {
		authors, err := batch.FindByIDs(ctx, authorIDs)
		if err != nil {
			return nil, err
		}
		return authors, nil
	}

	authors := make(map[uint64]note.AuthorDTO, len(authorIDs))
	for _, id := range authorIDs {
		author, err := s.authors.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		authors[id] = author
	}
	return authors, nil
}

type viewerStateMaps struct {
	likedNoteIDs     map[uint64]bool
	favoritedNoteIDs map[uint64]bool
	followingUserIDs map[uint64]bool
}

func (s *Service) findViewerState(ctx context.Context, viewerID uint64, noteIDs, authorIDs []uint64) (viewerStateMaps, error) {
	state := viewerStateMaps{
		likedNoteIDs:     map[uint64]bool{},
		favoritedNoteIDs: map[uint64]bool{},
		followingUserIDs: map[uint64]bool{},
	}
	if viewerID == 0 || s.viewer == nil {
		return state, nil
	}

	if batch, ok := s.viewer.(BatchViewerStateProvider); ok {
		liked, err := batch.LikedNoteIDs(ctx, viewerID, noteIDs)
		if err != nil {
			return viewerStateMaps{}, err
		}
		favorited, err := batch.FavoritedNoteIDs(ctx, viewerID, noteIDs)
		if err != nil {
			return viewerStateMaps{}, err
		}
		following, err := batch.FollowingAuthorIDs(ctx, viewerID, authorIDs)
		if err != nil {
			return viewerStateMaps{}, err
		}
		state.likedNoteIDs = liked
		state.favoritedNoteIDs = favorited
		state.followingUserIDs = following
		return state, nil
	}

	for _, noteID := range noteIDs {
		liked, err := s.viewer.HasLiked(ctx, viewerID, noteID)
		if err != nil {
			return viewerStateMaps{}, err
		}
		state.likedNoteIDs[noteID] = liked

		favorited, err := s.viewer.HasFavorited(ctx, viewerID, noteID)
		if err != nil {
			return viewerStateMaps{}, err
		}
		state.favoritedNoteIDs[noteID] = favorited
	}
	for _, authorID := range authorIDs {
		following, err := s.viewer.IsFollowing(ctx, viewerID, authorID)
		if err != nil {
			return viewerStateMaps{}, err
		}
		state.followingUserIDs[authorID] = following
	}
	return state, nil
}

func toFeedItem(n *note.Note, author note.AuthorDTO, tags []string, images []*note.NoteImage, viewerState viewerStateMaps, viewerID uint64) FeedItem {
	var imageURLs []string
	for _, img := range images {
		imageURLs = append(imageURLs, img.URL)
	}

	item := FeedItem{
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
	}

	if viewerID > 0 {
		liked := viewerState.likedNoteIDs[n.ID]
		favorited := viewerState.favoritedNoteIDs[n.ID]
		following := viewerState.followingUserIDs[n.AuthorID]
		item.ViewerLiked = &liked
		item.ViewerFavorited = &favorited
		item.ViewerFollowing = &following
	}

	return item
}

func collectFeedIDs(notes []*note.Note) ([]uint64, []uint64) {
	noteIDs := make([]uint64, 0, len(notes))
	authorIDs := make([]uint64, 0, len(notes))
	seenNoteIDs := make(map[uint64]struct{}, len(notes))
	seenAuthorIDs := make(map[uint64]struct{}, len(notes))

	for _, n := range notes {
		if n == nil {
			continue
		}
		if _, ok := seenNoteIDs[n.ID]; !ok {
			seenNoteIDs[n.ID] = struct{}{}
			noteIDs = append(noteIDs, n.ID)
		}
		if _, ok := seenAuthorIDs[n.AuthorID]; !ok {
			seenAuthorIDs[n.AuthorID] = struct{}{}
			authorIDs = append(authorIDs, n.AuthorID)
		}
	}

	return noteIDs, authorIDs
}
