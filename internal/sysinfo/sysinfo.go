package sysinfo

import (
	"fmt"
	"runtime"
)

type Info struct {
	Arch string
	OS   string
}

func Get() Info {
	arch := map[string]string{
		"386":     "x86",
		"amd64":   "x86_64",
		"arm64":   "aarch64",
		"loong64": "loongarch64",
		"ppc64":   "powerpc64",
		"ppc64le": "powerpc64le",
	}
	os := runtime.GOOS
	if os == "darwin" {
		os = "macos"
	}
	if v, ok := arch[runtime.GOARCH]; ok {
		return Info{v, os}
	}
	return Info{runtime.GOARCH, os}
}

func (s Info) String() string {
	return fmt.Sprintf("%s-%s", s.Arch, s.OS)
}
