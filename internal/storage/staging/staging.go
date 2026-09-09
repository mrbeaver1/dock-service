package staging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

type Directory interface {
	Close() error
}

type directory struct {
	path            string
	lock            *os.File
	previousTempDir string
	hadTempDir      bool
	closeOnce       sync.Once
	closeErr        error
}

var process struct {
	sync.Mutex
	active bool
}

const ownership = "dock-service multipart staging v1\n"
const lockName = ".dock-service.lock"

func NewDirectory(path string) (_ Directory, err error) {
	process.Lock()
	defer process.Unlock()
	if process.active {
		return nil, errors.New("multipart staging is already initialized in this process")
	}
	if path == "" {
		return nil, errors.New("multipart staging directory is required")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("multipart staging must be a directory, not a symbolic link")
	}
	lock, err := os.OpenFile(filepath.Join(path, lockName), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = lock.Close()
		}
	}()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, fmt.Errorf("lock multipart staging directory: %w", err)
	}
	if err = claim(path, lock); err != nil {
		return nil, err
	}
	if err = clean(path); err != nil {
		return nil, err
	}
	previous, existed := os.LookupEnv("TMPDIR")
	if err = os.Setenv("TMPDIR", path); err != nil {
		return nil, err
	}
	process.active = true
	return &directory{path: path, lock: lock, previousTempDir: previous, hadTempDir: existed}, nil
}

func claim(path string, lock *os.File) error {
	data, err := io.ReadAll(io.LimitReader(lock, int64(len(ownership)+1)))
	if err != nil {
		return err
	}
	if string(data) == ownership {
		return nil
	}
	if len(data) != 0 {
		return errors.New("unrecognized multipart staging owner")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != lockName {
			return errors.New("refusing to adopt a nonempty multipart staging directory")
		}
	}
	if _, err := lock.WriteString(ownership); err != nil {
		return err
	}
	return lock.Sync()
}

func clean(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "multipart-") {
			continue
		}
		if entry.IsDir() {
			return errors.New("unexpected directory in multipart staging")
		}
		if err := os.Remove(filepath.Join(path, entry.Name())); err != nil {
			return fmt.Errorf("remove abandoned multipart file: %w", err)
		}
	}
	return nil
}

func (d *directory) Close() error {
	d.closeOnce.Do(func() {
		process.Lock()
		defer process.Unlock()
		d.closeErr = clean(d.path)
		if os.Getenv("TMPDIR") == d.path {
			var err error
			if d.hadTempDir {
				err = os.Setenv("TMPDIR", d.previousTempDir)
			} else {
				err = os.Unsetenv("TMPDIR")
			}
			d.closeErr = errors.Join(d.closeErr, err)
		}
		d.closeErr = errors.Join(d.closeErr, d.lock.Close())
		process.active = false
	})
	return d.closeErr
}
