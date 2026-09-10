//go:build linux || darwin

package util

import (
	"errors"
	"os"
	"syscall"
)

// openLockFile opens the lock file and takes a non-blocking exclusive flock.
// The lock lives on the open file description, so it survives the holder
// writing to the file and is dropped automatically when the process exits.
func openLockFile(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errLockHeld
		}
		return nil, err
	}
	return file, nil
}

func closeLockFile(file *os.File) error {
	unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	closeErr := file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
