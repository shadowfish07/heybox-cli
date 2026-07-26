package thread

import "testing"

func TestParseTreePreservesPostAndReplyRelations(t *testing.T) {
	t.Parallel()
	body := []byte(`{
  "link": {
    "linkid": 123,
    "title": "测试帖子",
    "description": "摘要",
    "text": "[{\"text\":\"第一段\"},{\"text\":\"第二段\"},{\"type\":\"image\",\"url\":\"https://example.test/a.png\"}]",
    "create_at": 1700000000,
    "comment_num": 9,
    "user": {"userid": 7, "username": "楼主"},
    "topics": [{"name": "硬件"}]
  },
  "total_page": 2,
  "total_floor_num": 3,
  "has_more_floors": true,
  "comments": [{"comment": [
    {"commentid": 10, "text": "根评论", "child_num": 2, "floor_num": 1, "user": {"userid": 8, "username": "甲"}},
    {"commentid": 11, "text": "回复", "replyid": 10, "replyuser": {"username": "甲"}, "user": {"userid": 9, "username": "乙"}}
  ]}]
}`)

	page := parseTree(body)
	if page.post.ID != "123" || page.post.Content != "第一段\n第二段" || page.post.Author != "楼主" {
		t.Fatalf("post = %#v", page.post)
	}
	if page.totalComments != 9 || page.totalRoots != 3 || !page.hasMore {
		t.Fatalf("metadata = %#v", page)
	}
	if len(page.comments) != 1 || len(page.comments[0].Replies) != 1 {
		t.Fatalf("comments = %#v", page.comments)
	}
	reply := page.comments[0].Replies[0]
	if reply.RootID != "10" || reply.ReplyToID != "10" || reply.ReplyToAuthor != "甲" {
		t.Fatalf("reply = %#v", reply)
	}
}

func TestParseRepliesUsesCursorAndRoot(t *testing.T) {
	t.Parallel()
	page := parseReplies([]byte(`{"has_more":true,"lastval":99,"comments":[{"commentid":12,"text":"楼中楼","replyid":11,"replyuser":{"username":"乙"},"user":{"username":"丙"}}]}`), "10")
	if !page.hasMore || page.cursor != "99" || len(page.comments) != 1 {
		t.Fatalf("page = %#v", page)
	}
	if page.comments[0].RootID != "10" || page.comments[0].ReplyToID != "11" || page.comments[0].ReplyToAuthor != "乙" {
		t.Fatalf("comment = %#v", page.comments[0])
	}
}
