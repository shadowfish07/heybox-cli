package auth

import (
	"fmt"
	"io"
	"os"

	qrcode "github.com/skip2/go-qrcode"
)

func WriteTerminalQRCode(writer io.Writer, content string) error {
	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("生成终端二维码: %w", err)
	}
	bitmap := code.Bitmap()
	for row := 0; row < len(bitmap); row += 2 {
		for column := range bitmap[row] {
			top := bitmap[row][column]
			bottom := row+1 < len(bitmap) && bitmap[row+1][column]
			switch {
			case top && bottom:
				if _, err := io.WriteString(writer, "█"); err != nil {
					return fmt.Errorf("输出终端二维码: %w", err)
				}
			case top:
				if _, err := io.WriteString(writer, "▀"); err != nil {
					return fmt.Errorf("输出终端二维码: %w", err)
				}
			case bottom:
				if _, err := io.WriteString(writer, "▄"); err != nil {
					return fmt.Errorf("输出终端二维码: %w", err)
				}
			default:
				if _, err := io.WriteString(writer, " "); err != nil {
					return fmt.Errorf("输出终端二维码: %w", err)
				}
			}
		}
		if _, err := io.WriteString(writer, "\n"); err != nil {
			return fmt.Errorf("输出终端二维码: %w", err)
		}
	}
	return nil
}

func CreateTemporaryQRCodePNG(content string) (string, func(), error) {
	data, err := qrcode.Encode(content, qrcode.Medium, 512)
	if err != nil {
		return "", nil, fmt.Errorf("生成二维码图片: %w", err)
	}
	file, err := os.CreateTemp("", "heybox-login-*.png")
	if err != nil {
		return "", nil, fmt.Errorf("创建临时二维码图片: %w", err)
	}
	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("设置临时二维码图片权限: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("写入临时二维码图片: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("关闭临时二维码图片: %w", err)
	}
	return path, cleanup, nil
}
