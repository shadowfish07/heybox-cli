package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/shadowfish07/heybox-cli/internal/api"
	"github.com/shadowfish07/heybox-cli/internal/auth"
)

func newLoginCommand(stdout, stderr io.Writer, credentialStore auth.Store) *cobra.Command {
	var timeout time.Duration
	var browser bool
	var noBrowser bool
	command := &cobra.Command{
		Use:   "login",
		Short: "登录小黑盒并保存登录态",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if timeout <= 0 {
				return fmt.Errorf("--timeout 必须大于 0")
			}
			if noBrowser && !browser {
				return fmt.Errorf("--no-browser 只能与 --browser 一起使用")
			}
			loginContext, cancel := context.WithTimeout(command.Context(), timeout)
			defer cancel()

			var (
				credential auth.Credential
				err        error
			)
			if browser {
				credential, err = browserLogin(loginContext, stdout, stderr, noBrowser)
			} else {
				credential, err = qrLogin(loginContext, stdout)
			}
			if err != nil {
				return err
			}
			if err := credentialStore.Save(credential); err != nil {
				return fmt.Errorf("登录成功，但保存登录态失败: %w", err)
			}
			fmt.Fprintf(stdout, "登录成功，heybox_id=%s。\n", credential.HeyboxID)
			fmt.Fprintf(stdout, "登录态已保存到 %s（macOS/Linux 权限：目录 0700、文件 0600）。\n", credentialStore.Path())
			return nil
		},
	}
	command.Flags().DurationVar(&timeout, "timeout", 3*time.Minute, "等待完成登录的最长时间")
	command.Flags().BoolVar(&browser, "browser", false, "使用官方网页和本机回环回调登录")
	command.Flags().BoolVar(&noBrowser, "no-browser", false, "不自动打开网页，只输出地址（需配合 --browser）")
	return command
}

func qrLogin(ctx context.Context, stdout io.Writer) (auth.Credential, error) {
	client := api.NewClient("", 15*time.Second)
	challenge, err := client.CreateQRLogin(ctx)
	if err != nil {
		return auth.Credential{}, fmt.Errorf("获取小黑盒登录二维码: %w", err)
	}
	pngPath, cleanup, err := auth.CreateTemporaryQRCodePNG(challenge.URL)
	if err != nil {
		return auth.Credential{}, err
	}
	defer cleanup()

	fmt.Fprintln(stdout, "请使用小黑盒 App 扫码并在手机上确认登录：")
	if isTerminalWriter(stdout) {
		if err := auth.WriteTerminalQRCode(stdout, challenge.URL); err != nil {
			return auth.Credential{}, err
		}
	}
	fmt.Fprintln(stdout, "二维码图片：", pngPath)
	fmt.Fprintln(stdout, "等待扫码……（二维码图片会在命令结束时删除）")

	pollContext := ctx
	var cancel context.CancelFunc
	if challenge.ExpiresIn > 0 {
		pollContext, cancel = context.WithTimeout(ctx, challenge.ExpiresIn)
		defer cancel()
	}
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	lastState := api.QRLoginWaiting
	for {
		select {
		case <-pollContext.Done():
			if ctx.Err() != nil {
				return auth.Credential{}, fmt.Errorf("等待二维码登录: %w", ctx.Err())
			}
			if errors.Is(pollContext.Err(), context.DeadlineExceeded) {
				return auth.Credential{}, fmt.Errorf("二维码已过期，请重新运行 heybox login")
			}
			return auth.Credential{}, fmt.Errorf("等待二维码登录: %w", pollContext.Err())
		case <-ticker.C:
			result, err := client.PollQRLogin(pollContext, challenge.Token)
			if err != nil {
				if ctx.Err() != nil {
					return auth.Credential{}, fmt.Errorf("等待二维码登录: %w", ctx.Err())
				}
				if pollContext.Err() != nil {
					return auth.Credential{}, fmt.Errorf("二维码已过期，请重新运行 heybox login")
				}
				return auth.Credential{}, fmt.Errorf("查询二维码登录状态: %w", err)
			}
			switch result.State {
			case api.QRLoginWaiting:
				continue
			case api.QRLoginScanned:
				if lastState != result.State {
					fmt.Fprintln(stdout, "已扫码，请在手机上确认登录……")
				}
				lastState = result.State
			case api.QRLoginSucceeded:
				return auth.Credential{
					HeyboxID:     result.HeyboxID,
					PKey:         result.PKey,
					ExpireAt:     result.ExpireAt,
					XXHHHeyboxID: result.XXHHHeyboxID,
				}, nil
			case api.QRLoginExpired:
				return auth.Credential{}, fmt.Errorf("%s；请重新运行 heybox login", result.Message)
			default:
				return auth.Credential{}, fmt.Errorf("无法识别的二维码登录状态: %s", result.State)
			}
		}
	}
}

func browserLogin(ctx context.Context, stdout, stderr io.Writer, noBrowser bool) (auth.Credential, error) {
	session, err := auth.NewLoginSession()
	if err != nil {
		return auth.Credential{}, err
	}
	defer func() { _ = session.Close() }()
	if noBrowser {
		fmt.Fprintln(stdout, "请在这台电脑的浏览器中打开以下地址完成登录：")
		fmt.Fprintln(stdout, session.URL())
	} else {
		fmt.Fprintln(stdout, "正在打开小黑盒官方登录页……")
		if err := auth.OpenURL(session.URL()); err != nil {
			fmt.Fprintln(stderr, "警告:", err)
			fmt.Fprintln(stdout, "请手动打开：", session.URL())
		}
	}
	return session.Wait(ctx)
}

func isTerminalWriter(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func newLogoutCommand(stdout, stderr io.Writer, credentialStore auth.Store) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "删除本机保存的小黑盒登录态",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			err := credentialStore.Delete()
			if errors.Is(err, auth.ErrNotLoggedIn) {
				fmt.Fprintln(stdout, "本机没有保存的小黑盒登录态。")
			} else if err != nil {
				return err
			} else {
				fmt.Fprintf(stdout, "已删除 %s。\n", credentialStore.Path())
			}
			if strings.TrimSpace(os.Getenv("HEYBOX_COOKIE")) != "" {
				fmt.Fprintln(stderr, "提示: 当前 shell 仍设置了 HEYBOX_COOKIE；请执行 unset HEYBOX_COOKIE。")
			}
			return nil
		},
	}
}

func newAuthCommand(stdout io.Writer, credentialStore auth.Store) *cobra.Command {
	command := &cobra.Command{
		Use:   "auth",
		Short: "查看登录状态",
	}
	command.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "显示当前登录状态",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(os.Getenv("HEYBOX_COOKIE")) != "" {
				fmt.Fprintln(stdout, "已登录：当前使用 HEYBOX_COOKIE 环境变量（优先级最高）。")
				return nil
			}
			credential, err := credentialStore.Load()
			if errors.Is(err, auth.ErrNotLoggedIn) {
				fmt.Fprintln(stdout, "未登录。请运行 heybox login。")
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "已登录：heybox_id=%s，登录态来自 %s。\n", credential.HeyboxID, credentialStore.Path())
			return nil
		},
	})
	return command
}
