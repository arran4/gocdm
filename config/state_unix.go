//go:build !windows

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

var stateTmpSeq atomic.Uint64

func safeSaveStatePlatform(home string, data []byte, expectedUID int) error {
	homeFD, err := unix.Open(home, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return &os.PathError{Op: "open", Path: home, Err: err}
	}
	defer unix.Close(homeFD)

	var st unix.Stat_t
	if err := unix.Fstat(homeFD, &st); err != nil {
		return &os.PathError{Op: "stat", Path: home, Err: err}
	}
	if (uint32(st.Mode) & uint32(unix.S_IFMT)) != uint32(unix.S_IFDIR) {
		return fmt.Errorf("home path %q is not a directory", home)
	}
	if expectedUID >= 0 && st.Uid != uint32(expectedUID) {
		return fmt.Errorf("refusing to write state: home directory %q is owned by uid %d (expected %d)", home, st.Uid, expectedUID)
	}

	cfgFD, err := ensureSubdirNoFollow(homeFD, ".config", 0o755, expectedUID, home)
	if err != nil {
		return err
	}
	defer unix.Close(cfgFD)

	gocdmFD, err := ensureSubdirNoFollow(cfgFD, "gocdm", 0o700, expectedUID, home)
	if err != nil {
		return err
	}
	defer unix.Close(gocdmFD)

	statePath := filepath.Join(home, ".config", "gocdm", "state.json")
	if err := unix.Fstatat(gocdmFD, "state.json", &st, unix.AT_SYMLINK_NOFOLLOW); err == nil {
		mode := uint32(st.Mode) & uint32(unix.S_IFMT)
		if mode == uint32(unix.S_IFLNK) {
			return fmt.Errorf("refusing to overwrite symlink state file %q", statePath)
		}
		if mode != uint32(unix.S_IFREG) {
			return fmt.Errorf("refusing to overwrite non-regular state file %q", statePath)
		}
		if expectedUID >= 0 && st.Uid != uint32(expectedUID) {
			return fmt.Errorf("existing state file %q is owned by uid %d (expected %d); it may have been created by an earlier GoCDM version running as root — remove or fix ownership manually", statePath, st.Uid, expectedUID)
		}
	} else if err != unix.ENOENT {
		return &os.PathError{Op: "lstat", Path: statePath, Err: err}
	}

	tmpName := fmt.Sprintf("state.json.tmp.%d.%d", os.Getpid(), stateTmpSeq.Add(1))
	tmpFD, err := unix.Openat(gocdmFD, tmpName, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return &os.PathError{Op: "open", Path: filepath.Join(home, ".config", "gocdm", tmpName), Err: err}
	}

	tmpFile := os.NewFile(uintptr(tmpFD), tmpName)
	writeErr := writeAllAndSync(tmpFile, data)
	closeErr := tmpFile.Close()
	if writeErr != nil {
		_ = unix.Unlinkat(gocdmFD, tmpName, 0)
		return writeErr
	}
	if closeErr != nil {
		_ = unix.Unlinkat(gocdmFD, tmpName, 0)
		return closeErr
	}

	if err := unix.Renameat(gocdmFD, tmpName, gocdmFD, "state.json"); err != nil {
		_ = unix.Unlinkat(gocdmFD, tmpName, 0)
		return &os.PathError{Op: "rename", Path: statePath, Err: err}
	}
	return nil
}

func ensureSubdirNoFollow(parentFD int, name string, mode uint32, expectedUID int, home string) (int, error) {
	fd, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err == unix.ENOENT {
		if mkErr := unix.Mkdirat(parentFD, name, mode); mkErr != nil && mkErr != unix.EEXIST {
			return -1, &os.PathError{Op: "mkdir", Path: filepath.Join(home, name), Err: mkErr}
		}
		fd, err = unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	}
	if err != nil {
		if err == unix.ELOOP {
			return -1, fmt.Errorf("refusing to write state through symlink %q in %q", name, home)
		}
		return -1, &os.PathError{Op: "open", Path: filepath.Join(home, name), Err: err}
	}

	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		_ = unix.Close(fd)
		return -1, &os.PathError{Op: "stat", Path: filepath.Join(home, name), Err: err}
	}
	if (uint32(st.Mode) & uint32(unix.S_IFMT)) != uint32(unix.S_IFDIR) {
		_ = unix.Close(fd)
		return -1, fmt.Errorf("state path %q in %q is not a directory", name, home)
	}
	if expectedUID >= 0 && st.Uid != uint32(expectedUID) {
		_ = unix.Close(fd)
		return -1, fmt.Errorf("state directory %q in %q is owned by uid %d (expected %d); it may have been created by an earlier GoCDM version running as root — fix or remove it manually", name, home, st.Uid, expectedUID)
	}
	return fd, nil
}

func writeAllAndSync(f *os.File, data []byte) error {
	for len(data) > 0 {
		n, err := f.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return f.Sync()
}
