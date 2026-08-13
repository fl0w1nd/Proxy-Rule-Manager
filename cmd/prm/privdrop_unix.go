//go:build linux || darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// containerUID/GID is the unprivileged uid/gid the server drops to when
// started as root. It matches the distroless nonroot user so bind-mounted
// volumes prepared for that user keep working.
const (
	containerUID = 65532
	containerGID = 65532
)

// dropPrivileges makes a root-started process safe to run unprivileged: it
// chowns the data directory to containerUID/GID (so the soon-to-be non-root
// process can keep writing its state and artifacts) and then switches the
// process credentials. Processes already using an unprivileged uid keep their
// current credentials, covering local runs and `docker run --user 65532`.
func dropPrivileges(dataDir string) error {
	if os.Getuid() != 0 {
		return nil
	}
	if err := chownR(dataDir, containerUID, containerGID); err != nil {
		return fmt.Errorf("chown data dir %s: %w", dataDir, err)
	}
	if err := syscall.Setgroups([]int{}); err != nil {
		return fmt.Errorf("setgroups: %w", err)
	}
	if err := syscall.Setgid(containerGID); err != nil {
		return fmt.Errorf("setgid: %w", err)
	}
	if err := syscall.Setuid(containerUID); err != nil {
		return fmt.Errorf("setuid: %w", err)
	}
	return nil
}

// chownR recursively chowns root and everything under it to uid:gid. When the
// top of the tree is already owned by uid:gid (the steady state after the
// first boot), it returns immediately so restarts skip the O(n) scan.
func chownR(root string, uid, gid int) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) == uid && int(stat.Gid) == gid {
		return nil
	}
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}
