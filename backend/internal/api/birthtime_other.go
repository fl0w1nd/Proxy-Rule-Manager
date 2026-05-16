//go:build !darwin && !linux

package api

import (
	"os"
	"time"
)

// fileBirthtime falls back to mtime on platforms where birthtime is not
// readily available (Windows, BSDs without a dedicated build tag, etc.).
//
// TODO: add BSD support via syscall.Stat_t.Birthtimespec (same as darwin).
func fileBirthtime(path string, info os.FileInfo) time.Time {
	return info.ModTime()
}
