//go:build windows
// +build windows

package main

import "golang.org/x/sys/windows"

type instanceHandle = windows.Handle

const instanceMutexName = "TokenBridge_SingleInstance_v1"

// acquireInstanceLock 尝试创建全局命名 Mutex。
// 返回 (handle, true) 表示成功获取锁；返回 (0, false) 表示已有实例在运行。
func acquireInstanceLock() (instanceHandle, bool) {
	namePtr, err := windows.UTF16PtrFromString(instanceMutexName)
	if err != nil {
		return 0, true
	}

	handle, err := windows.CreateMutex(nil, false, namePtr)
	if err != nil {
		if err == windows.ERROR_ALREADY_EXISTS {
			return 0, false
		}
		return 0, true
	}

	return handle, true
}

func releaseInstanceLock(handle instanceHandle) {
	if handle != 0 {
		_ = windows.CloseHandle(handle)
	}
}
