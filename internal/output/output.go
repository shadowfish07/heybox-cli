package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"

	"github.com/shadowfish07/heybox-cli/internal/search"
	"github.com/shadowfish07/heybox-cli/internal/thread"
)

func JSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func Post(writer io.Writer, post thread.Post) error {
	fmt.Fprintln(writer, post.Title)
	meta := compactValues(post.Author, shortTime(post.PublishedAt), strings.Join(post.Topics, " / "))
	if len(meta) > 0 {
		fmt.Fprintln(writer, strings.Join(meta, " · "))
	}
	stats := compactValues(
		formatCount("浏览", post.Stats.Views),
		formatCount("点赞", post.Stats.Likes),
		formatCount("评论", post.Stats.Comments),
		formatCount("收藏", post.Stats.Favorites),
		formatCount("充电", post.Stats.Awards),
	)
	if len(stats) > 0 {
		fmt.Fprintln(writer, strings.Join(stats, " · "))
	}
	if post.URL != "" {
		fmt.Fprintln(writer, post.URL)
	}
	if post.Summary != "" && post.Summary != post.Content {
		fmt.Fprintf(writer, "\n%s\n", post.Summary)
	}
	if post.Content != "" {
		fmt.Fprintf(writer, "\n%s\n", post.Content)
	}
	return nil
}

func Comments(writer io.Writer, page thread.CommentsPage) error {
	for index, comment := range page.Comments {
		if index > 0 {
			fmt.Fprintln(writer)
		}
		label := fmt.Sprintf("#%d", index+1)
		if comment.Floor > 0 {
			label = fmt.Sprintf("#%d楼", comment.Floor)
		}
		flags := compactValues(
			label,
			comment.Author,
			shortTime(comment.PublishedAt),
			comment.IPLocation,
			formatCount("赞", comment.Likes),
		)
		if comment.IsTop {
			flags = append(flags, "置顶")
		}
		if comment.IsPostAuthor {
			flags = append(flags, "楼主")
		}
		fmt.Fprintln(writer, strings.Join(flags, " · "))
		fmt.Fprintln(writer, indentLines(comment.Text, "  "))
		for _, reply := range comment.Replies {
			replyBy := reply.Author
			if reply.ReplyToAuthor != "" {
				replyBy += " → " + reply.ReplyToAuthor
			}
			meta := compactValues(replyBy, shortTime(reply.PublishedAt), reply.IPLocation, formatCount("赞", reply.Likes))
			fmt.Fprintf(writer, "  ↳ %s\n", strings.Join(meta, " · "))
			fmt.Fprintln(writer, indentLines(reply.Text, "    "))
		}
	}
	if len(page.Comments) == 0 {
		fmt.Fprintln(writer, "没有找到评论。")
	}
	return nil
}

func Table(writer io.Writer, page search.Page) error {
	width := terminalWidth(writer)
	if width < 72 {
		return compact(writer, page)
	}

	typeWidth := 7
	idWidth := 10
	authorWidth := 14
	timeWidth := 16
	statsWidth := 16
	urlWidth := 32
	fixed := typeWidth + idWidth + authorWidth + timeWidth + statsWidth + urlWidth + 18
	titleWidth := width - fixed
	if titleWidth < 24 {
		urlWidth = 20
		titleWidth = width - (typeWidth + idWidth + authorWidth + timeWidth + statsWidth + urlWidth + 18)
	}
	if titleWidth < 18 {
		return compact(writer, page)
	}

	fmt.Fprintf(writer, "%s  %s  %s  %s  %s  %s  %s\n",
		pad("TYPE", typeWidth), pad("ID", idWidth), pad("TITLE", titleWidth), pad("AUTHOR/TOPIC", authorWidth),
		pad("TIME", timeWidth), pad("STATS", statsWidth), pad("URL", urlWidth))
	fmt.Fprintln(writer, strings.Repeat("-", width))
	for _, result := range page.Results {
		title := result.Title
		if result.Summary != "" {
			title += " — " + result.Summary
		}
		author := firstNonEmpty(result.Author, result.Topic)
		fmt.Fprintf(writer, "%s  %s  %s  %s  %s  %s  %s\n",
			pad(result.Type, typeWidth), pad(result.ID, idWidth), pad(title, titleWidth), pad(author, authorWidth),
			pad(shortTime(result.PublishedAt), timeWidth), pad(formatStats(result), statsWidth), pad(result.URL, urlWidth))
	}
	if len(page.Results) == 0 {
		fmt.Fprintln(writer, "没有找到结果。")
	}
	return nil
}

func compact(writer io.Writer, page search.Page) error {
	for index, result := range page.Results {
		label := result.Type
		if result.ID != "" {
			label += ":" + result.ID
		}
		fmt.Fprintf(writer, "%d. [%s] %s\n", index+1, label, result.Title)
		if result.Summary != "" {
			fmt.Fprintf(writer, "   %s\n", truncate(result.Summary, 100))
		}
		meta := []string{firstNonEmpty(result.Author, result.Topic), shortTime(result.PublishedAt), formatStats(result)}
		filtered := meta[:0]
		for _, value := range meta {
			if value != "" {
				filtered = append(filtered, value)
			}
		}
		if len(filtered) > 0 {
			fmt.Fprintf(writer, "   %s\n", strings.Join(filtered, " · "))
		}
		if result.URL != "" {
			fmt.Fprintf(writer, "   %s\n", result.URL)
		}
	}
	if len(page.Results) == 0 {
		fmt.Fprintln(writer, "没有找到结果。")
	}
	return nil
}

func terminalWidth(writer io.Writer) int {
	if columns, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && columns > 0 {
		return columns
	}
	if file, ok := writer.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			return width
		}
	}
	return 120
}

func pad(value string, width int) string {
	value = truncate(value, width)
	return value + strings.Repeat(" ", max(0, width-runewidth.StringWidth(value)))
}

func truncate(value string, width int) string {
	value = strings.TrimSpace(value)
	if runewidth.StringWidth(value) <= width {
		return value
	}
	if width <= 1 {
		return runewidth.Truncate(value, width, "")
	}
	return runewidth.Truncate(value, width, "…")
}

func shortTime(value string) string {
	if len(value) >= 16 {
		return strings.ReplaceAll(value[:16], "T", " ")
	}
	return value
}

func formatStats(result search.Result) string {
	var parts []string
	if result.Stats.Views > 0 {
		parts = append(parts, fmt.Sprintf("看%d", result.Stats.Views))
	}
	if result.Stats.Likes > 0 {
		parts = append(parts, fmt.Sprintf("赞%d", result.Stats.Likes))
	}
	if result.Stats.Comments > 0 {
		parts = append(parts, fmt.Sprintf("评%d", result.Stats.Comments))
	}
	if result.Stats.Followers > 0 {
		parts = append(parts, fmt.Sprintf("关%d", result.Stats.Followers))
	}
	if result.Stats.Hot > 0 {
		parts = append(parts, fmt.Sprintf("热%d", result.Stats.Hot))
	}
	return strings.Join(parts, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func compactValues(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}

func formatCount(label string, value int64) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%s%d", label, value)
}

func indentLines(value, prefix string) string {
	if value == "" {
		return prefix + "（无文字内容）"
	}
	return prefix + strings.ReplaceAll(value, "\n", "\n"+prefix)
}
