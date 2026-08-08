//go:build !darwin && !windows

package xdg

func defaultNativeAppDirs() []string { return nil }
