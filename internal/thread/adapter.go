package thread

import (
	"context"
	"time"

	"github.com/shadowfish07/heybox-cli/internal/api"
)

type ClientAdapter struct{ client *api.Client }

func NewClientAdapter(cookie string, timeout time.Duration, options ...api.ClientOption) *ClientAdapter {
	return &ClientAdapter{client: api.NewClient(cookie, timeout, options...)}
}

func (a *ClientAdapter) GetPostThread(ctx context.Context, query ThreadQuery) ([]byte, error) {
	return a.client.GetPostThread(ctx, api.ThreadQuery{LinkID: query.PostID, Offset: query.Offset, Limit: query.Limit, Sort: query.Sort})
}

func (a *ClientAdapter) GetCommentReplies(ctx context.Context, query RepliesQuery) ([]byte, error) {
	return a.client.GetCommentReplies(ctx, api.RepliesQuery{LinkID: query.PostID, RootCommentID: query.RootID, Cursor: query.Cursor, Limit: query.Limit})
}

var _ API = (*ClientAdapter)(nil)
