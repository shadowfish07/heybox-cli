package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Store interface {
	Load() (Credential, error)
	Save(Credential) error
	Delete() error
	Path() string
}

type FileStore struct {
	path    string
	pathErr error
}

func NewFileStore() FileStore {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return FileStore{pathErr: fmt.Errorf("定位用户配置目录: %w", err)}
	}
	return NewFileStoreAt(filepath.Join(configDir, "heybox-cli", "session.json"))
}

func NewFileStoreAt(path string) FileStore {
	return FileStore{path: filepath.Clean(path)}
}

func (store FileStore) Path() string {
	return store.path
}

func (store FileStore) Load() (Credential, error) {
	if err := store.validatePath(); err != nil {
		return Credential{}, err
	}
	if err := validateExistingPrivateDirectory(filepath.Dir(store.path)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Credential{}, ErrNotLoggedIn
		}
		return Credential{}, err
	}
	info, err := os.Lstat(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return Credential{}, ErrNotLoggedIn
	}
	if err != nil {
		return Credential{}, fmt.Errorf("检查登录态文件: %w", err)
	}
	if err := validateSessionFile(info); err != nil {
		return Credential{}, err
	}
	data, err := os.ReadFile(store.path)
	if err != nil {
		return Credential{}, fmt.Errorf("读取登录态文件: %w", err)
	}
	var credential Credential
	if err := json.Unmarshal(data, &credential); err != nil {
		return Credential{}, fmt.Errorf("解析登录态文件: %w", err)
	}
	if err := credential.Validate(); err != nil {
		return Credential{}, fmt.Errorf("登录态文件无效: %w", err)
	}
	return credential, nil
}

func (store FileStore) Save(credential Credential) error {
	if err := store.validatePath(); err != nil {
		return err
	}
	if err := credential.Validate(); err != nil {
		return err
	}
	directory := filepath.Dir(store.path)
	if err := ensurePrivateDirectory(directory); err != nil {
		return err
	}
	if info, err := os.Lstat(store.path); err == nil {
		if err := validateSessionFile(info); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("检查已有登录态文件: %w", err)
	}

	temporary, err := os.CreateTemp(directory, ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时登录态文件: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("设置临时登录态文件权限: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(credential); err != nil {
		return fmt.Errorf("写入临时登录态文件: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("同步临时登录态文件: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("关闭临时登录态文件: %w", err)
	}
	closed = true
	if err := replaceFile(temporaryPath, store.path); err != nil {
		return fmt.Errorf("原子替换登录态文件: %w", err)
	}
	if err := os.Chmod(store.path, 0o600); err != nil {
		return fmt.Errorf("设置登录态文件权限: %w", err)
	}
	return nil
}

func (store FileStore) Delete() error {
	if err := store.validatePath(); err != nil {
		return err
	}
	if err := validateExistingPrivateDirectory(filepath.Dir(store.path)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotLoggedIn
		}
		return err
	}
	info, err := os.Lstat(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotLoggedIn
	}
	if err != nil {
		return fmt.Errorf("检查登录态文件: %w", err)
	}
	if err := validateSessionFile(info); err != nil {
		return err
	}
	if err := os.Remove(store.path); err != nil {
		return fmt.Errorf("删除登录态文件: %w", err)
	}
	return nil
}

func (store FileStore) validatePath() error {
	if store.pathErr != nil {
		return store.pathErr
	}
	if store.path == "" || store.path == "." {
		return fmt.Errorf("登录态文件路径为空")
	}
	return nil
}

func ensurePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("创建登录态目录: %w", err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return fmt.Errorf("检查登录态目录: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("登录态目录必须是真实目录，不能是符号链接: %s", path)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("设置登录态目录权限: %w", err)
	}
	return nil
}

func validateExistingPrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("登录态目录必须是真实目录，不能是符号链接: %s", path)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("登录态目录权限过宽（当前 %04o，要求 0700）", info.Mode().Perm())
	}
	return nil
}

func validateSessionFile(info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("登录态路径必须是普通文件，不能是符号链接")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("登录态文件权限过宽（当前 %04o，要求 0600）", info.Mode().Perm())
	}
	return nil
}
