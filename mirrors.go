package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func StoreMirrorList(mirrors []string) error {
	fpath := filepath.Join(getConfigDir(), "mirrorlist.txt")
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, m := range mirrors {
		f.WriteString(m + "\n")
	}
	return nil
}

func ReadMirrorList() ([]string, error) {
	fpath := filepath.Join(getConfigDir(), "mirrorlist.txt")
	f, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	mirrors := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		mirrors = append(mirrors, line)
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return mirrors, nil
}

func GetDefaultMirrorList() []string {
	mirrors := []string{"https://ziglang.org/download/"}
	mirrors_url := "https://ziglang.org/download/community-mirrors.txt"
	m, err := GetMirrors(mirrors_url)
	if err == nil {
		mirrors = append(mirrors, m...)
	}
	return mirrors
}
