//go:build !windows
// +build !windows

package main

type instanceHandle = uintptr

func acquireInstanceLock() (instanceHandle, bool) {
	return 0, true
}

func releaseInstanceLock(handle instanceHandle) {}
