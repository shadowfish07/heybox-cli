package search

import "context"

type Result struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Summary     string `json:"summary,omitempty"`
	Author      string `json:"author,omitempty"`
	Topic       string `json:"topic,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	Stats       Stats  `json:"stats,omitempty"`
	URL         string `json:"url,omitempty"`
}

type Stats struct {
	Views     int64 `json:"views,omitempty"`
	Likes     int64 `json:"likes,omitempty"`
	Comments  int64 `json:"comments,omitempty"`
	Followers int64 `json:"followers,omitempty"`
	Hot       int64 `json:"hot,omitempty"`
}

type Options struct {
	Keyword  string
	Type     string
	Sort     string
	Page     int
	Limit    int
	All      bool
	MaxPages int
}

type Page struct {
	Query    string   `json:"query"`
	Type     string   `json:"type"`
	Page     int      `json:"page"`
	Limit    int      `json:"limit"`
	Partial  bool     `json:"partial"`
	Warnings []string `json:"warnings,omitempty"`
	Results  []Result `json:"results"`
}

type API interface {
	SearchGeneral(context.Context, APIQuery) ([]byte, error)
	SearchTopics(context.Context, APIQuery) ([]byte, error)
	SearchGames(context.Context, APIQuery) ([]byte, error)
}

type APIQuery struct {
	Keyword string
	Type    string
	Sort    string
	Offset  int
	Limit   int
}
