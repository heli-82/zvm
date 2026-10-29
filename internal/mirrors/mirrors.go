package mirrors

import (
	"bufio"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"zvm/internal/dirs"
)

const (
	MainMirror    = "https://ziglang.org/download/"
	CommunityList = "https://ziglang.org/download/community-mirrors.txt"
)

// Fetch downloads a newline-separated mirror list from url.
func Fetch(url string) ([]string, error) {
	var mirrors []string
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Status code != 200 (OK)")
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		mirrors = append(mirrors, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return mirrors, nil
}

func Default() []string {
	mirrors := []string{MainMirror}
	m, err := Fetch(CommunityList)
	if err == nil {
		mirrors = append(mirrors, m...)
	}
	return mirrors
}

func Read() ([]string, error) {
	fpath := filepath.Join(dirs.Config(), "mirrorlist.txt")
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

func Store(mirrors []string) error {
	fpath := filepath.Join(dirs.Config(), "mirrorlist.txt")
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
