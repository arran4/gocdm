//go:build !windows

package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const maxSafeUserFileSize = 1 << 20 // 1 MiB

var safePathGeteuid = os.Geteuid

func validateRelUserPath(relPath string) ([]string, error) {
	if relPath == "" {
		return nil, fmt.Errorf("relative path cannot be empty")
	}
	if strings.ContainsRune(relPath, '\x00') {
		return nil, fmt.Errorf("relative path contains NUL byte")
	}
	if filepath.IsAbs(relPath) || strings.HasPrefix(relPath, "/") || strings.HasPrefix(relPath, "\\") {
		return nil, fmt.Errorf("path %q must be relative", relPath)
	}
	segs := strings.Split(filepath.ToSlash(relPath), "/")
	for _, seg := range segs {
		if seg == "" || seg == "." || seg == ".." {
			return nil, fmt.Errorf("invalid path segment %q in %q", seg, relPath)
		}
	}
	return segs, nil
}

// SafeReadUserFile opens and reads relPath beneath homeDir without following symlinks.
// When running as root (euid == 0) for a non-root expectedUID (> 0), every traversed
// directory and the target file are checked for ownership matching expectedUID (or root
// if allowRootOwned is true).
func SafeReadUserFile(homeDir, relPath string, expectedUID int, allowRootOwned bool) ([]byte, error) {
	if err := ValidateHomeDir(homeDir); err != nil {
		return nil, err
	}
	segs, err := validateRelUserPath(relPath)
	if err != nil {
		return nil, err
	}

	enforceUID := safePathGeteuid() == 0 && expectedUID > 0

	curFD, err := unix.Open(homeDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: homeDir, Err: err}
	}
	defer func() {
		if curFD >= 0 {
			_ = unix.Close(curFD)
		}
	}()

	var st unix.Stat_t
	if err := unix.Fstat(curFD, &st); err != nil {
		return nil, &os.PathError{Op: "stat", Path: homeDir, Err: err}
	}
	if (uint32(st.Mode) & uint32(unix.S_IFMT)) != uint32(unix.S_IFDIR) {
		return nil, fmt.Errorf("home path %q is not a directory", homeDir)
	}
	if enforceUID && st.Uid != uint32(expectedUID) {
		return nil, fmt.Errorf("refusing to read user file: home directory %q is owned by uid %d (expected %d)", homeDir, st.Uid, expectedUID)
	}

	for _, seg := range segs[:len(segs)-1] {
		nextFD, err := unix.Openat(curFD, seg, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			if err == unix.ELOOP {
				return nil, fmt.Errorf("refusing to follow symlink at %q in %q", seg, homeDir)
			}
			return nil, &os.PathError{Op: "open", Path: filepath.Join(homeDir, relPath), Err: err}
		}
		_ = unix.Close(curFD)
		curFD = nextFD

		if err := unix.Fstat(curFD, &st); err != nil {
			return nil, &os.PathError{Op: "stat", Path: filepath.Join(homeDir, seg), Err: err}
		}
		if enforceUID && st.Uid != uint32(expectedUID) && !(allowRootOwned && st.Uid == 0) {
			return nil, fmt.Errorf("refusing to traverse %q in %q owned by uid %d (expected %d)", seg, homeDir, st.Uid, expectedUID)
		}
	}

	finalSeg := segs[len(segs)-1]
	fullPath := filepath.Join(homeDir, relPath)
	fileFD, err := unix.Openat(curFD, finalSeg, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		if err == unix.ELOOP {
			return nil, fmt.Errorf("refusing to follow symlink %q in %q", fullPath, homeDir)
		}
		return nil, &os.PathError{Op: "open", Path: fullPath, Err: err}
	}

	f := os.NewFile(uintptr(fileFD), fullPath)
	defer f.Close()

	if err := unix.Fstat(fileFD, &st); err != nil {
		return nil, &os.PathError{Op: "stat", Path: fullPath, Err: err}
	}
	if (uint32(st.Mode) & uint32(unix.S_IFMT)) != uint32(unix.S_IFREG) {
		return nil, fmt.Errorf("refusing to read non-regular file %q", fullPath)
	}
	if enforceUID && st.Uid != uint32(expectedUID) && !(allowRootOwned && st.Uid == 0) {
		return nil, fmt.Errorf("refusing to read %q owned by uid %d (expected %d)", fullPath, st.Uid, expectedUID)
	}

	data, err := io.ReadAll(io.LimitReader(f, maxSafeUserFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSafeUserFileSize {
		return nil, fmt.Errorf("user file %q exceeds %d bytes", fullPath, maxSafeUserFileSize)
	}
	return data, nil
}
