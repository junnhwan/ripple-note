package cache

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"ripple-note/internal/note"
)

func TestNoteServiceCacheCachesAnonymousDetail(t *testing.T) {
	t.Parallel()

	store := newMemoryJSONStore()
	source := &fakeNoteSource{
		detail: note.NoteDTO{
			ID:             1001,
			Title:          "cached detail",
			Body:           "body",
			Status:         note.StatusPublished,
			Visibility:     note.VisibilityPublic,
			LikesCount:     3,
			FavoritesCount: 2,
			CommentsCount:  1,
		},
	}
	cached := NewNoteServiceCache(store, source)

	first, err := cached.Detail(context.Background(), 1001, 0)
	if err != nil {
		t.Fatalf("first detail: %v", err)
	}
	if first.Title != "cached detail" {
		t.Fatalf("expected source detail title, got %q", first.Title)
	}

	source.detail.Title = "changed in source"
	second, err := cached.Detail(context.Background(), 1001, 0)
	if err != nil {
		t.Fatalf("second detail: %v", err)
	}
	if second.Title != "cached detail" {
		t.Fatalf("expected cached title, got %q", second.Title)
	}
	if source.detailCalls != 1 {
		t.Fatalf("expected source called once, got %d", source.detailCalls)
	}
}

func TestNoteServiceCacheStoresCountsSnapshot(t *testing.T) {
	t.Parallel()

	store := newMemoryJSONStore()
	source := &fakeNoteSource{
		detail: note.NoteDTO{
			ID:             1002,
			Title:          "counts",
			Status:         note.StatusPublished,
			Visibility:     note.VisibilityPublic,
			LikesCount:     11,
			FavoritesCount: 7,
			CommentsCount:  5,
		},
	}
	cached := NewNoteServiceCache(store, source)

	if _, err := cached.Detail(context.Background(), 1002, 0); err != nil {
		t.Fatalf("detail: %v", err)
	}

	var counts NoteCountsSnapshot
	if err := store.Get(context.Background(), NoteCountsKey(1002), &counts); err != nil {
		t.Fatalf("expected counts cache entry: %v", err)
	}
	if counts.LikesCount != 11 || counts.FavoritesCount != 7 || counts.CommentsCount != 5 {
		t.Fatalf("unexpected counts snapshot: %#v", counts)
	}
}

func TestNoteServiceCacheBypassesCacheForViewerDetail(t *testing.T) {
	t.Parallel()

	store := newMemoryJSONStore()
	source := &fakeNoteSource{detail: note.NoteDTO{ID: 1003, Title: "first"}}
	cached := NewNoteServiceCache(store, source)

	if _, err := cached.Detail(context.Background(), 1003, 42); err != nil {
		t.Fatalf("first detail: %v", err)
	}
	source.detail.Title = "second"
	result, err := cached.Detail(context.Background(), 1003, 42)
	if err != nil {
		t.Fatalf("second detail: %v", err)
	}
	if result.Title != "second" {
		t.Fatalf("expected viewer detail to bypass cache, got %q", result.Title)
	}
	if source.detailCalls != 2 {
		t.Fatalf("expected source called twice, got %d", source.detailCalls)
	}
}

func TestNoteServiceCacheFallsBackWhenStoreFails(t *testing.T) {
	t.Parallel()

	store := newMemoryJSONStore()
	store.getErr = assertErr("redis unavailable")
	source := &fakeNoteSource{detail: note.NoteDTO{ID: 1004, Title: "from mysql"}}
	cached := NewNoteServiceCache(store, source)

	result, err := cached.Detail(context.Background(), 1004, 0)
	if err != nil {
		t.Fatalf("detail should fall back to source: %v", err)
	}
	if result.Title != "from mysql" {
		t.Fatalf("expected source result, got %#v", result)
	}
}

type fakeNoteSource struct {
	detail      note.NoteDTO
	detailCalls int
}

func (s *fakeNoteSource) Publish(context.Context, note.PublishInput, uint64) (note.NoteDTO, error) {
	return note.NoteDTO{}, nil
}

func (s *fakeNoteSource) Detail(context.Context, uint64, uint64) (note.NoteDTO, error) {
	s.detailCalls++
	return s.detail, nil
}

func (s *fakeNoteSource) MyNotes(context.Context, uint64, int, int) (note.NoteListDTO, error) {
	return note.NoteListDTO{}, nil
}

type memoryJSONStore struct {
	mu     sync.Mutex
	values map[string][]byte
	getErr error
}

func newMemoryJSONStore() *memoryJSONStore {
	return &memoryJSONStore{values: map[string][]byte{}}
}

func (s *memoryJSONStore) Get(_ context.Context, key string, dest any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.getErr != nil {
		return s.getErr
	}
	data, ok := s.values[key]
	if !ok {
		return ErrCacheMiss
	}
	return json.Unmarshal(data, dest)
}

func (s *memoryJSONStore) Set(_ context.Context, key string, value any, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.values[key] = data
	return nil
}

func (s *memoryJSONStore) Delete(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range keys {
		delete(s.values, key)
	}
	return nil
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
