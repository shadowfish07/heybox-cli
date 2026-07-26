package thread

import "context"

type Post struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary,omitempty"`
	Content     string    `json:"content,omitempty"`
	Author      string    `json:"author,omitempty"`
	AuthorID    string    `json:"author_id,omitempty"`
	Topics      []string  `json:"topics,omitempty"`
	PublishedAt string    `json:"published_at,omitempty"`
	UpdatedAt   string    `json:"updated_at,omitempty"`
	URL         string    `json:"url,omitempty"`
	Stats       PostStats `json:"stats"`
}

type PostStats struct {
	Views     int64 `json:"views,omitempty"`
	Likes     int64 `json:"likes,omitempty"`
	Comments  int64 `json:"comments,omitempty"`
	Favorites int64 `json:"favorites,omitempty"`
	Awards    int64 `json:"awards,omitempty"`
}

type Comment struct {
	ID            string    `json:"id"`
	RootID        string    `json:"root_id,omitempty"`
	ReplyToID     string    `json:"reply_to_id,omitempty"`
	ReplyToAuthor string    `json:"reply_to_author,omitempty"`
	Author        string    `json:"author,omitempty"`
	AuthorID      string    `json:"author_id,omitempty"`
	Text          string    `json:"text"`
	PublishedAt   string    `json:"published_at,omitempty"`
	IPLocation    string    `json:"ip_location,omitempty"`
	Likes         int64     `json:"likes,omitempty"`
	Floor         int64     `json:"floor,omitempty"`
	ReplyCount    int64     `json:"reply_count,omitempty"`
	IsPostAuthor  bool      `json:"is_post_author,omitempty"`
	IsTop         bool      `json:"is_top,omitempty"`
	Images        []string  `json:"images,omitempty"`
	Replies       []Comment `json:"replies"`
}

type CommentsPage struct {
	PostID            string    `json:"post_id"`
	Sort              string    `json:"sort"`
	Page              int       `json:"page"`
	Limit             int       `json:"limit"`
	TotalPages        int       `json:"total_pages,omitempty"`
	TotalComments     int64     `json:"total_comments,omitempty"`
	TotalRootComments int64     `json:"total_root_comments,omitempty"`
	Partial           bool      `json:"partial"`
	Warnings          []string  `json:"warnings,omitempty"`
	Comments          []Comment `json:"comments"`
}

type Options struct {
	PostID        string
	Sort          string
	Page          int
	Limit         int
	All           bool
	MaxPages      int
	Replies       bool
	MaxReplyPages int
}

type API interface {
	GetPostThread(context.Context, ThreadQuery) ([]byte, error)
	GetCommentReplies(context.Context, RepliesQuery) ([]byte, error)
}

type ThreadQuery struct {
	PostID string
	Offset int
	Limit  int
	Sort   string
}

type RepliesQuery struct {
	PostID string
	RootID string
	Cursor string
	Limit  int
}
