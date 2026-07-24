package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/shadowfish07/heybox-cli/internal/api"
	"github.com/shadowfish07/heybox-cli/internal/auth"
	"github.com/shadowfish07/heybox-cli/internal/output"
	"github.com/shadowfish07/heybox-cli/internal/search"
)

var version = "dev"

type searchFlags struct {
	resultType string
	sort       string
	page       int
	limit      int
	all        bool
	maxPages   int
	json       bool
	timeout    time.Duration
}

func Execute(args []string, stdout, stderr io.Writer) int {
	root := newRootCommand(stdout, stderr)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, "错误:", err)
		return exitCode(err)
	}
	return 0
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	credentialStore := auth.NewFileStore()
	root := &cobra.Command{
		Use:           "heybox",
		Short:         "搜索小黑盒社区内容",
		Version:       buildVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(newSearchCommand(stdout, stderr, credentialStore))
	root.AddCommand(newLoginCommand(stdout, stderr, credentialStore))
	root.AddCommand(newLogoutCommand(stdout, stderr, credentialStore))
	root.AddCommand(newAuthCommand(stdout, credentialStore))
	root.AddCommand(newCompletionCommand(root, stdout))
	return root
}

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	return resolveVersion(version, info)
}

func resolveVersion(injected string, info *debug.BuildInfo) string {
	if injected != "dev" {
		return injected
	}
	if info != nil && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return injected
}

func newCompletionCommand(root *cobra.Command, stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                   "completion [bash|zsh|fish|powershell]",
		Short:                 "生成 shell 自动补全脚本",
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE: func(_ *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(stdout)
			case "zsh":
				return root.GenZshCompletion(stdout)
			case "fish":
				return root.GenFishCompletion(stdout, true)
			case "powershell":
				return root.GenPowerShellCompletion(stdout)
			default:
				return fmt.Errorf("不支持的 shell %q", args[0])
			}
		},
	}
}

func newSearchCommand(stdout, stderr io.Writer, credentialStore auth.Store) *cobra.Command {
	flags := searchFlags{}
	command := &cobra.Command{
		Use:   "search <关键词>",
		Short: "搜索帖子、话题、用户或游戏",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := validate(flags, args[0]); err != nil {
				return err
			}
			cookie := strings.TrimSpace(os.Getenv("HEYBOX_COOKIE"))
			if cookie == "" {
				credential, credentialErr := credentialStore.Load()
				switch {
				case credentialErr == nil:
					cookie = credential.Cookie()
				case !errors.Is(credentialErr, auth.ErrNotLoggedIn):
					fmt.Fprintln(stderr, "警告: 无法读取保存的登录态，将匿名搜索：", credentialErr)
				}
			}
			client := search.NewClientAdapter(cookie, flags.timeout)
			service := search.NewService(client)
			page, err := service.Search(command.Context(), search.Options{
				Keyword:  strings.TrimSpace(args[0]),
				Type:     flags.resultType,
				Sort:     flags.sort,
				Page:     flags.page,
				Limit:    flags.limit,
				All:      flags.all,
				MaxPages: flags.maxPages,
			})
			if err != nil {
				return err
			}
			for _, warning := range page.Warnings {
				fmt.Fprintln(stderr, "警告:", warning)
			}
			if flags.json {
				return output.JSON(stdout, page)
			}
			return output.Table(stdout, page)
		},
	}
	command.Flags().StringVarP(&flags.resultType, "type", "t", "all", "结果类型: all, post, topic, user, game")
	command.Flags().StringVar(&flags.sort, "sort", "relevance", "排序: relevance, latest, hot")
	command.Flags().IntVarP(&flags.page, "page", "p", 1, "起始页码")
	command.Flags().IntVarP(&flags.limit, "limit", "n", 20, "每页结果数 (1-50)")
	command.Flags().BoolVar(&flags.all, "all", false, "连续获取多页结果")
	command.Flags().IntVar(&flags.maxPages, "max-pages", 5, "--all 最多获取的页数 (1-20)")
	command.Flags().BoolVar(&flags.json, "json", false, "输出稳定 JSON")
	command.Flags().DurationVar(&flags.timeout, "timeout", 15*time.Second, "单次 HTTP 请求超时")
	return command
}

func validate(flags searchFlags, keyword string) error {
	if strings.TrimSpace(keyword) == "" {
		return fmt.Errorf("关键词不能为空")
	}
	if !oneOf(flags.resultType, "all", "post", "topic", "user", "game") {
		return fmt.Errorf("无效的 --type %q", flags.resultType)
	}
	if !oneOf(flags.sort, "relevance", "latest", "hot") {
		return fmt.Errorf("无效的 --sort %q", flags.sort)
	}
	if flags.page < 1 {
		return fmt.Errorf("--page 必须大于等于 1")
	}
	if flags.limit < 1 || flags.limit > 50 {
		return fmt.Errorf("--limit 必须在 1 到 50 之间")
	}
	if flags.maxPages < 1 || flags.maxPages > 20 {
		return fmt.Errorf("--max-pages 必须在 1 到 20 之间")
	}
	if flags.timeout <= 0 {
		return fmt.Errorf("--timeout 必须大于 0")
	}
	return nil
}

func oneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func exitCode(err error) int {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return 6
	}
	switch api.Kind(err) {
	case api.ErrorAuth:
		return 3
	case api.ErrorCaptcha, api.ErrorRateLimit:
		return 4
	case api.ErrorIncompatible:
		return 5
	case api.ErrorNetwork:
		return 6
	default:
		return 2
	}
}
