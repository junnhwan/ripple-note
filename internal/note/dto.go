package note

import "time"

type NoteDTO struct {
	ID             uint64     `json:"id"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	Status         string     `json:"status"`
	Visibility     string     `json:"visibility"`
	Author         AuthorDTO  `json:"author"`
	Images         []ImageDTO `json:"images"`
	Tags           []string   `json:"tags"`
	LikesCount     uint64     `json:"likes_count"`
	FavoritesCount uint64     `json:"favorites_count"`
	CommentsCount  uint64     `json:"comments_count"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type AuthorDTO struct {
	ID        uint64 `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type ImageDTO struct {
	ID     uint64 `json:"id"`
	URL    string `json:"url"`
	Width  *int   `json:"width,omitempty"`
	Height *int   `json:"height,omitempty"`
}

type PublishInput struct {
	Title     string
	Body      string
	ImageURLs []string
	Tags      []string
}

type NoteListDTO struct {
	Items []NoteDTO `json:"items"`
	Total int64     `json:"total"`
}
