package main

import (
	"fmt"
	"runtime"
)

type SysInfo struct {
	arch string
	os   string
}

func (s SysInfo) String() string {
	return fmt.Sprintf("%s-%s", s.arch, s.os)
}

func GetSysInfo() SysInfo {
	arch := map[string]string{
		"386":     "x86",
		"amd64":   "x86_64",
		"arm64":   "aarch64",
		"loong64": "loongarch64",
		"ppc64":   "powerpc64",
		"ppc64le": "powerpc64le",
	}
	if v, ok := arch[runtime.GOARCH]; ok {
		return SysInfo{v, runtime.GOOS}
	}
	return SysInfo{runtime.GOARCH, runtime.GOOS}
}
