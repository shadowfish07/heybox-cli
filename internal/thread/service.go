package thread

import (
	"context"
	"fmt"
	"time"
)

const replyPageSize = 50

type Service struct {
	api      API
	pageWait time.Duration
	sleep    func(context.Context, time.Duration) error
}

func NewService(client API) *Service {
	return &Service{
		api:      client,
		pageWait: 200 * time.Millisecond,
		sleep: func(ctx context.Context, duration time.Duration) error {
			timer := time.NewTimer(duration)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (s *Service) GetPost(ctx context.Context, postID string) (Post, error) {
	body, err := s.api.GetPostThread(ctx, ThreadQuery{PostID: postID, Limit: 1, Sort: "hot"})
	if err != nil {
		return Post{}, err
	}
	post := parseTree(body).post
	if post.ID == "" {
		return Post{}, fmt.Errorf("帖子 %s 不存在，或小黑盒未返回详情", postID)
	}
	return post, nil
}

func (s *Service) GetComments(ctx context.Context, options Options) (CommentsPage, error) {
	result := CommentsPage{
		PostID:   options.PostID,
		Sort:     options.Sort,
		Page:     options.Page,
		Limit:    options.Limit,
		Comments: make([]Comment, 0),
	}
	pageCount := 1
	if options.All {
		pageCount = options.MaxPages
	}
	seenRoots := make(map[string]struct{})
	for index := 0; index < pageCount; index++ {
		if index > 0 {
			if err := s.sleep(ctx, s.pageWait); err != nil {
				return result, err
			}
		}
		pageNumber := options.Page + index
		body, err := s.api.GetPostThread(ctx, ThreadQuery{
			PostID: options.PostID,
			Offset: (pageNumber - 1) * options.Limit,
			Limit:  options.Limit,
			Sort:   upstreamSort(options.Sort),
		})
		if err != nil {
			return result, err
		}
		page := parseTree(body)
		result.TotalPages = page.totalPages
		result.TotalComments = page.totalComments
		result.TotalRootComments = page.totalRoots
		added := 0
		for _, comment := range page.comments {
			if _, exists := seenRoots[comment.ID]; exists {
				continue
			}
			seenRoots[comment.ID] = struct{}{}
			if options.Replies && comment.ReplyCount > 0 {
				replies, warnings, partial, err := s.getReplies(ctx, options, comment)
				if err != nil {
					return result, err
				}
				comment.Replies = replies
				result.Warnings = appendUnique(result.Warnings, warnings...)
				result.Partial = result.Partial || partial
			}
			result.Comments = append(result.Comments, comment)
			added++
		}
		if !options.All || !page.hasMore || pageNumber >= page.totalPages || added == 0 {
			break
		}
	}
	return result, nil
}

func (s *Service) getReplies(ctx context.Context, options Options, root Comment) ([]Comment, []string, bool, error) {
	replies := append(make([]Comment, 0, len(root.Replies)), root.Replies...)
	seen := make(map[string]struct{}, len(replies))
	for _, reply := range replies {
		seen[reply.ID] = struct{}{}
	}
	cursor := ""
	for pageNumber := 1; pageNumber <= options.MaxReplyPages; pageNumber++ {
		if pageNumber > 1 {
			if err := s.sleep(ctx, s.pageWait); err != nil {
				return replies, nil, true, err
			}
		}
		body, err := s.api.GetCommentReplies(ctx, RepliesQuery{
			PostID: options.PostID,
			RootID: root.ID,
			Cursor: cursor,
			Limit:  replyPageSize,
		})
		if err != nil {
			warning := fmt.Sprintf("楼层 %s 的楼中楼获取失败：%v", root.ID, err)
			return replies, []string{warning}, true, nil
		}
		page := parseReplies(body, root.ID)
		for _, reply := range page.comments {
			if _, exists := seen[reply.ID]; exists {
				continue
			}
			seen[reply.ID] = struct{}{}
			replies = append(replies, reply)
		}
		if !page.hasMore {
			return replies, nil, false, nil
		}
		if page.cursor == "" || page.cursor == cursor {
			return replies, []string{fmt.Sprintf("楼层 %s 的楼中楼游标未推进，已停止", root.ID)}, true, nil
		}
		cursor = page.cursor
	}
	return replies, []string{fmt.Sprintf("楼层 %s 的楼中楼超过 --max-reply-pages=%d，结果已截断", root.ID, options.MaxReplyPages)}, true, nil
}

func upstreamSort(sort string) string {
	switch sort {
	case "oldest":
		return "time_aes"
	case "latest":
		return "time_desc"
	default:
		return "hot"
	}
}

func appendUnique(existing []string, values ...string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, value := range existing {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		existing = append(existing, value)
	}
	return existing
}
