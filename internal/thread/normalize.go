package thread

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var htmlTag = regexp.MustCompile(`<[^>]+>`)

type treePage struct {
	post          Post
	comments      []Comment
	totalPages    int
	totalComments int64
	totalRoots    int64
	hasMore       bool
}

type repliesPage struct {
	comments []Comment
	cursor   string
	hasMore  bool
}

func parseTree(body []byte) treePage {
	root := decode(body)
	page := treePage{
		post:       normalizePost(object(root["link"])),
		totalPages: int(number(root, "total_page")),
		totalRoots: number(root, "total_floor_num"),
		hasMore:    boolean(root, "has_more_floors"),
	}
	page.totalComments = page.post.Stats.Comments
	for _, groupValue := range array(root["comments"]) {
		group := object(groupValue)
		items := array(group["comment"])
		if len(items) == 0 {
			continue
		}
		rootComment := normalizeComment(object(items[0]))
		if rootComment.ID == "" {
			continue
		}
		rootComment.RootID = rootComment.ID
		rootComment.Replies = make([]Comment, 0, max(0, len(items)-1))
		for _, replyValue := range items[1:] {
			reply := normalizeComment(object(replyValue))
			if reply.ID == "" {
				continue
			}
			reply.RootID = rootComment.ID
			rootComment.Replies = append(rootComment.Replies, reply)
		}
		page.comments = append(page.comments, rootComment)
	}
	return page
}

func parseReplies(body []byte, rootID string) repliesPage {
	root := decode(body)
	page := repliesPage{cursor: text(root, "lastval"), hasMore: boolean(root, "has_more")}
	for _, value := range array(root["comments"]) {
		comment := normalizeComment(object(value))
		if comment.ID == "" {
			continue
		}
		comment.RootID = first(comment.RootID, rootID)
		page.comments = append(page.comments, comment)
	}
	return page
}

func normalizePost(item map[string]any) Post {
	post := Post{
		ID:          text(item, "linkid", "link_id", "id"),
		Title:       clean(text(item, "title"), false),
		Summary:     clean(text(item, "description", "desc"), false),
		PublishedAt: timestamp(item, "create_at", "created_at"),
		UpdatedAt:   timestamp(item, "modify_at", "updated_at"),
		URL:         text(item, "share_url", "url"),
		Stats: PostStats{
			Views:     number(item, "click", "view_num", "views"),
			Likes:     number(item, "up", "like_num"),
			Comments:  number(item, "comment_num"),
			Favorites: number(item, "favour_count", "favorite_count"),
			Awards:    number(item, "link_award_num", "award_num"),
		},
	}
	user := object(item["user"])
	post.Author = text(user, "username", "nickname", "name")
	post.AuthorID = text(user, "userid", "heybox_id", "id")
	post.Content = contentText(item["text"])
	for _, value := range array(item["topics"]) {
		name := clean(text(object(value), "name", "title"), false)
		if name != "" {
			post.Topics = append(post.Topics, name)
		}
	}
	return post
}

func normalizeComment(item map[string]any) Comment {
	user := object(item["user"])
	replyUser := object(item["replyuser"])
	comment := Comment{
		ID:            text(item, "commentid", "comment_id", "id"),
		RootID:        text(item, "root_comment_id"),
		ReplyToID:     text(item, "replyid", "reply_id"),
		ReplyToAuthor: text(replyUser, "username", "nickname", "name"),
		Author:        text(user, "username", "nickname", "name"),
		AuthorID:      first(text(user, "userid", "heybox_id", "id"), text(item, "userid", "user_id")),
		Text:          clean(text(item, "text", "content"), true),
		PublishedAt:   timestamp(item, "create_at", "created_at"),
		IPLocation:    clean(text(item, "ip_location"), false),
		Likes:         number(item, "up", "like_num"),
		Floor:         number(item, "floor_num", "floor"),
		ReplyCount:    number(item, "child_num", "reply_num"),
		IsPostAuthor:  boolean(item, "is_link_owner"),
		IsTop:         boolean(item, "is_top"),
		Replies:       make([]Comment, 0),
	}
	for _, value := range array(item["images"]) {
		image := object(value)
		url := first(text(image, "url", "original", "src"), scalar(value))
		if url != "" {
			comment.Images = append(comment.Images, url)
		}
	}
	return comment
}

func contentText(value any) string {
	if raw, ok := value.(string); ok {
		var decoded any
		if json.Unmarshal([]byte(raw), &decoded) != nil {
			return clean(raw, true)
		}
		value = decoded
	}
	var parts []string
	collectContent(value, &parts)
	return clean(strings.Join(parts, "\n"), true)
}

func collectContent(value any, parts *[]string) {
	switch typed := value.(type) {
	case []any:
		for _, child := range typed {
			collectContent(child, parts)
		}
	case map[string]any:
		for _, key := range []string{"text", "content"} {
			if child, exists := typed[key]; exists {
				collectContent(child, parts)
				return
			}
		}
		for _, child := range typed {
			switch child.(type) {
			case []any, map[string]any:
				collectContent(child, parts)
			}
		}
	case string:
		if strings.TrimSpace(typed) != "" && !strings.HasPrefix(typed, "http://") && !strings.HasPrefix(typed, "https://") {
			*parts = append(*parts, typed)
		}
	}
}

func decode(body []byte) map[string]any {
	var root map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	_ = decoder.Decode(&root)
	return root
}

func object(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func array(value any) []any {
	result, _ := value.([]any)
	return result
}

func text(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := scalar(item[key]); value != "" {
			return value
		}
	}
	return ""
}

func scalar(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
}

func number(item map[string]any, keys ...string) int64 {
	value := text(item, keys...)
	parsed, _ := strconv.ParseFloat(value, 64)
	return int64(parsed)
}

func boolean(item map[string]any, keys ...string) bool {
	for _, key := range keys {
		switch value := item[key].(type) {
		case bool:
			return value
		case json.Number:
			return value.String() != "0"
		case string:
			return value == "1" || strings.EqualFold(value, "true")
		}
	}
	return false
}

func timestamp(item map[string]any, keys ...string) string {
	value := text(item, keys...)
	if value == "" {
		return ""
	}
	if epoch, err := strconv.ParseInt(value, 10, 64); err == nil {
		if epoch > 1_000_000_000_000 {
			epoch /= 1000
		}
		return time.Unix(epoch, 0).Local().Format(time.RFC3339)
	}
	return value
}

func clean(value string, preserveLines bool) string {
	value = htmlTag.ReplaceAllString(value, "")
	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\r", "")
	if !preserveLines {
		return strings.Join(strings.Fields(value), " ")
	}
	lines := strings.Split(value, "\n")
	cleaned := lines[:0]
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
