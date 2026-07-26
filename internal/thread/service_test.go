package thread

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type fakeAPI struct {
	threadQueries  []ThreadQuery
	repliesQueries []RepliesQuery
}

func (f *fakeAPI) GetPostThread(_ context.Context, query ThreadQuery) ([]byte, error) {
	f.threadQueries = append(f.threadQueries, query)
	if query.PostID == "missing" {
		return []byte(`{}`), nil
	}
	if query.Offset == 0 {
		return []byte(`{"link":{"linkid":123,"comment_num":4},"total_page":2,"total_floor_num":2,"has_more_floors":true,"comments":[{"comment":[{"commentid":10,"text":"一楼","child_num":2,"user":{"username":"甲"}},{"commentid":11,"text":"预览","replyid":10,"user":{"username":"乙"}}]}]}`), nil
	}
	return []byte(`{"link":{"linkid":123,"comment_num":4},"total_page":2,"total_floor_num":2,"has_more_floors":false,"comments":[{"comment":[{"commentid":10,"text":"置顶重复","child_num":2}]},{"comment":[{"commentid":20,"text":"二楼","child_num":0}]}]}`), nil
}

func (f *fakeAPI) GetCommentReplies(_ context.Context, query RepliesQuery) ([]byte, error) {
	f.repliesQueries = append(f.repliesQueries, query)
	if query.RootID != "10" {
		return nil, fmt.Errorf("unexpected root %s", query.RootID)
	}
	if query.Cursor == "" {
		return []byte(`{"has_more":true,"lastval":11,"comments":[{"commentid":11,"text":"预览","replyid":10,"user":{"username":"乙"}}]}`), nil
	}
	return []byte(`{"has_more":false,"lastval":12,"comments":[{"commentid":12,"text":"回复预览","replyid":11,"replyuser":{"username":"乙"},"user":{"username":"丙"}}]}`), nil
}

func TestServiceGetsAllRootsAndCompleteReplies(t *testing.T) {
	t.Parallel()
	api := &fakeAPI{}
	service := NewService(api)
	service.pageWait = 0
	service.sleep = func(context.Context, time.Duration) error { return nil }
	page, err := service.GetComments(context.Background(), Options{
		PostID: "123", Sort: "latest", Page: 1, Limit: 2, All: true, MaxPages: 5, Replies: true, MaxReplyPages: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Comments) != 2 || len(page.Comments[0].Replies) != 2 {
		t.Fatalf("page = %#v", page)
	}
	if page.Comments[0].Replies[1].ReplyToID != "11" || page.Comments[0].Replies[1].ReplyToAuthor != "乙" {
		t.Fatalf("nested reply = %#v", page.Comments[0].Replies[1])
	}
	if len(api.threadQueries) != 2 || api.threadQueries[1].Offset != 2 || api.threadQueries[0].Sort != "time_desc" {
		t.Fatalf("thread queries = %#v", api.threadQueries)
	}
	if len(api.repliesQueries) != 2 || api.repliesQueries[1].Cursor != "11" {
		t.Fatalf("reply queries = %#v", api.repliesQueries)
	}
}

func TestGetPostRejectsEmptyUpstreamResult(t *testing.T) {
	t.Parallel()
	_, err := NewService(&fakeAPI{}).GetPost(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
}
