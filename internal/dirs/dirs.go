package dirs

import (
	"os"
	"path/filepath"
)

func Config() string {
	path, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	res := filepath.Join(path, "zvm")
	err = os.MkdirAll(res, 0755)
	if err != nil {
		return "."
	}
	return res
}

func Share() string {
	path, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	res := filepath.Join(path, ".local", "share", "zvm")
	err = os.MkdirAll(res, 0755)
	if err != nil {
		return "."
	}
	return res
}

func Bin() string {
	path, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	res := filepath.Join(path, ".local", "bin")
	err = os.MkdirAll(res, 0755)
	if err != nil {
		return "."
	}
	return res
}
