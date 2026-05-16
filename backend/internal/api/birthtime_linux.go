//go:build linux

package api

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// fileBirthtime returns the creation time (birthtime) of the file at path.
// Linux exposes birthtime via statx(2) (kernel ≥ 4.11). If the syscall fails
// or the filesystem does not populate STATX_BTIME, we fall back to mtime.
//
// TODO: filesystems that don't track birthtime (e.g. older ext3, tmpfs) will
// always fall back to mtime. The frontend sorts icons by this field; the order
// may differ from the Node.js version on such systems.
func fileBirthtime(path string, info os.FileInfo) time.Time {
	var stx unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_STATX_SYNC_AS_STAT, unix.STATX_BTIME, &stx); err == nil {
		if stx.Mask&unix.STATX_BTIME != 0 && stx.Btime.Sec != 0 {
			return time.Unix(stx.Btime.Sec, int64(stx.Btime.Nsec))
		}
	}
	return info.ModTime()
}
