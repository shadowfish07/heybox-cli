package search

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var searchHighlightTag = regexp.MustCompile(`(?i)</?em(?:\s[^>]*)?>`)

func normalizeTopics(body []byte) []Result {
	root := decodeObject(body)
	items := findArray(root, "topics", "items", "list")
	results := make([]Result, 0, len(items))
	for _, item := range items {
		id := stringValue(item, "topic_id", "id")
		name := stringValue(item, "name", "title")
		if name == "" {
			continue
		}
		result := Result{
			Type:    "topic",
			ID:      id,
			Title:   cleanText(name),
			Summary: cleanText(stringValue(item, "description", "desc", "show_desc")),
			URL:     stringValue(item, "url", "share_url", "web_url"),
			Stats: Stats{
				Hot: nestedInt(item, []string{"hot", "raw_hot_value"}, "raw_value"),
			},
		}
		results = append(results, result)
	}
	return results
}

func normalizeGames(body []byte) []Result {
	root := decodeObject(body)
	items := findArray(root, "games", "items", "list")
	results := make([]Result, 0, len(items))
	for _, item := range items {
		id := stringValue(item, "steam_appid", "appid", "app_id", "id")
		title := stringValue(item, "name", "title", "game_name")
		if title == "" {
			continue
		}
		result := Result{
			Type:    "game",
			ID:      id,
			Title:   cleanText(title),
			Summary: cleanText(firstNonEmpty(stringValue(item, "complete_tags_str", "description", "desc"), joinStrings(item["platforms"]))),
			URL:     stringValue(item, "url", "share_url", "web_url"),
			Stats: Stats{
				Followers: intValue(item, "follow_num", "followers"),
			},
		}
		results = append(results, result)
	}
	return results
}

func normalizeGeneral(body []byte, requestedType string) []Result {
	root := decodeObject(body)
	items := findArray(root, "items", "list", "results", "links")
	flat := flattenItems(items, "")
	results := make([]Result, 0, len(flat))
	for _, entry := range flat {
		item := unwrap(entry.item)
		resultType := inferType(item, firstNonEmpty(entry.inheritedType, stringValue(item, "search_type", "type", "tab_type")))
		if requestedType != "all" && resultType != requestedType {
			continue
		}
		result := normalizeItem(item, resultType)
		if result.Title == "" {
			continue
		}
		results = append(results, result)
	}
	return results
}

type flatItem struct {
	item          map[string]any
	inheritedType string
}

func flattenItems(items []map[string]any, inheritedType string) []flatItem {
	var out []flatItem
	for _, item := range items {
		itemType := firstNonEmpty(stringValue(item, "search_type", "type", "tab_type"), inheritedType)
		children := findArray(item, "items", "list", "results", "links")
		if len(children) > 0 && !hasIdentity(item) {
			out = append(out, flattenItems(children, itemType)...)
			continue
		}
		out = append(out, flatItem{item: item, inheritedType: itemType})
	}
	return out
}

func unwrap(item map[string]any) map[string]any {
	for _, key := range []string{"info", "link", "post", "article", "topic", "user", "game", "data", "item"} {
		if nested, ok := item[key].(map[string]any); ok {
			for outerKey, outerValue := range item {
				if outerKey == key {
					continue
				}
				if _, exists := nested[outerKey]; !exists {
					nested[outerKey] = outerValue
				}
			}
			return nested
		}
	}
	return item
}

func normalizeItem(item map[string]any, resultType string) Result {
	result := Result{Type: resultType}
	switch resultType {
	case "post":
		result.ID = stringValue(item, "linkid", "link_id", "post_id", "id")
		result.Title = cleanText(stringValue(item, "title", "link_title", "subject", "name"))
		result.Summary = cleanText(stringValue(item, "description", "summary", "content", "text", "abstract"))
		result.Author = nestedString(item, []string{"user", "nickname"}, []string{"user", "username"}, []string{"author", "nickname"}, []string{"author", "username"})
		if result.Author == "" {
			result.Author = stringValue(item, "nickname", "author_name", "username")
		}
		result.Topic = nestedString(item, []string{"topic", "name"}, []string{"topics", "name"})
		result.PublishedAt = timeValue(item, "create_at", "create_time", "created_at", "publish_time", "timestamp")
		result.Stats.Views = intValue(item, "view_num", "views", "read_num")
		result.Stats.Likes = intValue(item, "up", "thumbs_up_num", "like_num", "likes")
		result.Stats.Comments = intValue(item, "comment_num", "comments")
		result.URL = stringValue(item, "url", "share_url", "link_url")
	case "user":
		result.ID = stringValue(item, "heybox_id", "userid", "user_id", "id")
		result.Title = cleanText(stringValue(item, "nickname", "username", "name", "title"))
		result.Summary = cleanText(stringValue(item, "signature", "description", "desc"))
		result.Stats.Followers = intValue(item, "fans_num", "follower_num", "followers")
	case "topic":
		return firstResult(normalizeTopics(mustJSON(map[string]any{"topics": []any{item}})))
	case "game":
		return firstResult(normalizeGames(mustJSON(map[string]any{"games": []any{item}})))
	default:
		result.Type = "unknown"
		result.ID = stringValue(item, "id")
		result.Title = cleanText(stringValue(item, "title", "name"))
		result.Summary = cleanText(stringValue(item, "description", "summary"))
	}
	return result
}

func inferType(item map[string]any, hint string) string {
	hint = strings.ToLower(hint)
	switch {
	case strings.Contains(hint, "link"), strings.Contains(hint, "post"), strings.Contains(hint, "article"):
		return "post"
	case strings.Contains(hint, "topic"):
		return "topic"
	case strings.Contains(hint, "user"), strings.Contains(hint, "account"):
		return "user"
	case strings.Contains(hint, "game"):
		return "game"
	case hasAny(item, "linkid", "link_id", "post_id"):
		return "post"
	case hasAny(item, "steam_appid", "appid", "game_name"):
		return "game"
	case hasAny(item, "topic_id"):
		return "topic"
	case hasAny(item, "heybox_id", "userid", "user_id"):
		return "user"
	default:
		return "unknown"
	}
}

func decodeObject(body []byte) map[string]any {
	var root map[string]any
	_ = json.Unmarshal(body, &root)
	return root
}

func findArray(root map[string]any, keys ...string) []map[string]any {
	for _, key := range keys {
		value, ok := root[key]
		if !ok {
			continue
		}
		if array, ok := value.([]any); ok {
			out := make([]map[string]any, 0, len(array))
			for _, element := range array {
				if item, ok := element.(map[string]any); ok {
					out = append(out, item)
				}
			}
			return out
		}
	}
	for _, value := range root {
		if nested, ok := value.(map[string]any); ok {
			if result := findArray(nested, keys...); len(result) > 0 {
				return result
			}
		}
	}
	return nil
}

func stringValue(item map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := item[key].(type) {
		case string:
			if value != "" {
				return value
			}
		case json.Number:
			return value.String()
		case float64:
			return strconv.FormatInt(int64(value), 10)
		case int:
			return strconv.Itoa(value)
		case int64:
			return strconv.FormatInt(value, 10)
		}
	}
	return ""
}

func intValue(item map[string]any, keys ...string) int64 {
	for _, key := range keys {
		switch value := item[key].(type) {
		case float64:
			return int64(value)
		case json.Number:
			parsed, _ := value.Int64()
			return parsed
		case string:
			parsed, _ := strconv.ParseFloat(value, 64)
			return int64(parsed)
		case int64:
			return value
		case int:
			return int64(value)
		}
	}
	return 0
}

func nestedInt(item map[string]any, path []string, fallbacks ...string) int64 {
	current := any(item)
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return intValue(item, fallbacks...)
		}
		current = object[key]
	}
	switch value := current.(type) {
	case float64:
		return int64(value)
	case string:
		parsed, _ := strconv.ParseFloat(value, 64)
		return int64(parsed)
	default:
		return intValue(item, fallbacks...)
	}
}

func nestedString(item map[string]any, paths ...[]string) string {
	for _, path := range paths {
		current := any(item)
		for _, key := range path {
			if array, ok := current.([]any); ok {
				if len(array) == 0 {
					current = nil
					break
				}
				current = array[0]
			}
			object, ok := current.(map[string]any)
			if !ok {
				current = nil
				break
			}
			current = object[key]
		}
		if value, ok := current.(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func timeValue(item map[string]any, keys ...string) string {
	value := stringValue(item, keys...)
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

func cleanText(value string) string {
	value = searchHighlightTag.ReplaceAllString(value, "")
	value = html.UnescapeString(value)
	value = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func joinStrings(value any) string {
	array, ok := value.([]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(array))
	for _, element := range array {
		if text, ok := element.(string); ok {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " / ")
}

func hasIdentity(item map[string]any) bool {
	return hasAny(item, "id", "linkid", "link_id", "topic_id", "heybox_id", "userid", "steam_appid", "appid")
}

func hasAny(item map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := item[key]; ok {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func mustJSON(value any) []byte {
	body, _ := json.Marshal(value)
	return body
}

func firstResult(results []Result) Result {
	if len(results) > 0 {
		return results[0]
	}
	return Result{}
}
