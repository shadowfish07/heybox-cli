package search

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
	api      API
	pageWait time.Duration
	sleep    func(context.Context, time.Duration) error
}

func NewService(client API) *Service {
	return &Service{
		api:      client,
		pageWait: 300 * time.Millisecond,
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

func (s *Service) Search(ctx context.Context, options Options) (Page, error) {
	page := Page{
		Query:   options.Keyword,
		Type:    options.Type,
		Page:    options.Page,
		Limit:   options.Limit,
		Results: make([]Result, 0),
	}

	pages := 1
	if options.All {
		pages = options.MaxPages
	}
	seen := make(map[string]struct{})
	for index := 0; index < pages; index++ {
		if index > 0 {
			if err := s.sleep(ctx, s.pageWait); err != nil {
				return page, err
			}
		}
		pageNumber := options.Page + index
		results, warnings, partial, err := s.searchPage(ctx, options, pageNumber)
		if err != nil {
			return page, err
		}
		page.Partial = page.Partial || partial
		page.Warnings = appendUnique(page.Warnings, warnings...)

		newCount := 0
		for _, result := range results {
			key := result.Type + ":" + result.ID
			if result.ID == "" {
				key = result.Type + ":" + result.Title + ":" + result.Author
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			page.Results = append(page.Results, result)
			newCount++
		}
		if !options.All || len(results) < options.Limit || newCount == 0 {
			break
		}
	}
	return page, nil
}

func (s *Service) searchPage(ctx context.Context, options Options, page int) ([]Result, []string, bool, error) {
	query := APIQuery{
		Keyword: options.Keyword,
		Type:    options.Type,
		Sort:    options.Sort,
		Offset:  (page - 1) * options.Limit,
		Limit:   options.Limit,
	}

	switch options.Type {
	case "topic":
		body, err := s.api.SearchTopics(ctx, query)
		if err != nil {
			return nil, nil, false, err
		}
		return normalizeTopics(body), nil, false, nil
	case "game":
		body, err := s.api.SearchGames(ctx, query)
		if err != nil {
			return nil, nil, false, err
		}
		return normalizeGames(body), nil, false, nil
	case "post", "user":
		body, err := s.api.SearchGeneral(ctx, query)
		if err != nil {
			return nil, nil, false, err
		}
		return normalizeGeneral(body, options.Type), nil, false, nil
	case "all":
		return s.searchAll(ctx, query)
	default:
		return nil, nil, false, fmt.Errorf("unsupported search type %q", options.Type)
	}
}

func (s *Service) searchAll(ctx context.Context, query APIQuery) ([]Result, []string, bool, error) {
	var warnings []string
	var firstErr error
	failedSources := 0

	postQuery := query
	postQuery.Type = "post"
	postBody, postErr := s.api.SearchGeneral(ctx, postQuery)
	var posts []Result
	if postErr != nil {
		firstErr = postErr
		failedSources++
		warnings = append(warnings, "帖子搜索失败："+postErr.Error())
	} else {
		posts = normalizeGeneral(postBody, "post")
	}

	userQuery := query
	userQuery.Type = "user"
	userBody, userErr := s.api.SearchGeneral(ctx, userQuery)
	var users []Result
	if userErr != nil {
		if firstErr == nil {
			firstErr = userErr
		}
		failedSources++
		warnings = append(warnings, "用户搜索失败："+userErr.Error())
	} else {
		users = normalizeGeneral(userBody, "user")
	}

	var topics []Result
	topicBody, topicErr := s.api.SearchTopics(ctx, query)
	if topicErr != nil {
		if firstErr == nil {
			firstErr = topicErr
		}
		failedSources++
		warnings = append(warnings, "话题搜索失败："+topicErr.Error())
	} else {
		topics = normalizeTopics(topicBody)
	}

	var games []Result
	gameBody, gameErr := s.api.SearchGames(ctx, query)
	if gameErr != nil {
		if firstErr == nil {
			firstErr = gameErr
		}
		failedSources++
		warnings = append(warnings, "游戏搜索失败："+gameErr.Error())
	} else {
		games = normalizeGames(gameBody)
	}

	results := interleave(query.Limit, posts, topics, users, games)
	if len(results) == 0 && failedSources == 4 {
		return nil, warnings, true, firstErr
	}
	return results, warnings, failedSources > 0, nil
}

func interleave(limit int, groups ...[]Result) []Result {
	if limit <= 0 {
		return nil
	}
	results := make([]Result, 0, limit)
	for index := 0; len(results) < limit; index++ {
		added := false
		for _, group := range groups {
			if index >= len(group) {
				continue
			}
			results = append(results, group[index])
			added = true
			if len(results) == limit {
				break
			}
		}
		if !added {
			break
		}
	}
	return results
}

func appendUnique(existing []string, values ...string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, value := range existing {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		existing = append(existing, value)
	}
	return existing
}
