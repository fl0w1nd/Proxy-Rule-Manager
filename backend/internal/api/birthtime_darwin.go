//go:build darwin

package api

import (
	"os"
	"syscall"
	"time"
)

// fileBirthtime returns the creation time of the file at path.
// On macOS, Birthtimespec is populated by the HFS+/APFS kernel layer.
func fileBirthtime(path string, info os.FileInfo) time.Time {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		bt := time.Unix(st.Birthtimespec.Sec, st.Birthtimespec.Nsec)
		if !bt.IsZero() {
			return bt
		}
	}
	return info.ModTime()
}
