package search

import (
	"context"
	"time"

	"github.com/shadowfish07/heybox-cli/internal/api"
)

type ClientAdapter struct{ client *api.Client }

func NewClientAdapter(cookie string, timeout time.Duration, options ...api.ClientOption) *ClientAdapter {
	return &ClientAdapter{client: api.NewClient(cookie, timeout, options...)}
}

func (a *ClientAdapter) SearchGeneral(ctx context.Context, query APIQuery) ([]byte, error) {
	return a.client.SearchGeneral(ctx, toAPIQuery(query))
}

func (a *ClientAdapter) SearchTopics(ctx context.Context, query APIQuery) ([]byte, error) {
	return a.client.SearchTopics(ctx, toAPIQuery(query))
}

func (a *ClientAdapter) SearchGames(ctx context.Context, query APIQuery) ([]byte, error) {
	return a.client.SearchGames(ctx, toAPIQuery(query))
}

func toAPIQuery(query APIQuery) api.Query {
	return api.Query{
		Keyword: query.Keyword,
		Type:    query.Type,
		Sort:    query.Sort,
		Offset:  query.Offset,
		Limit:   query.Limit,
	}
}

var _ API = (*ClientAdapter)(nil)
