//go:build windows

package main

// dropPrivileges keeps the current Windows process token.
func dropPrivileges(string) error {
	return nil
}
