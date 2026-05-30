package interaction

import "time"

type CommentDTO struct {
	ID        uint64    `json:"id"`
	NoteID    uint64    `json:"note_id"`
	AuthorID  uint64    `json:"author_id"`
	Author    string    `json:"author_nickname"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type CommentListDTO struct {
	Items []CommentDTO `json:"items"`
	Total int64        `json:"total"`
}
