//go:build windows

package util

import (
	"errors"
	"os"
	"syscall"
)

// errorSharingViolation is ERROR_SHARING_VIOLATION, returned when the lock
// file is already open without sharing.
const errorSharingViolation = syscall.Errno(32)

// openLockFile creates or opens the lock file with an empty share mode, so a
// second open fails while the lock is held. The handle is closed by
// closeLockFile, which also releases the lock.
func openLockFile(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(
		name,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		if errors.Is(err, errorSharingViolation) {
			return nil, errLockHeld
		}
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func closeLockFile(file *os.File) error { return file.Close() }
