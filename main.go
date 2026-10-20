package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

func GetMirrors(url string) ([]string, error) {
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

type Artifact struct {
	Tarball string `json:"tarball"`
	Shasum  string `json:"shasum"`
	Size    string `json:"size"`
}

type Release struct {
	Version   string
	Date      string
	Docs      string
	StdDocs   string
	Notes     string
	Artifacts map[string]Artifact
}

func (r *Release) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	r.Artifacts = make(map[string]Artifact)

	for k, v := range raw {
		var err error
		switch k {
		case "version":
			err = json.Unmarshal(v, &r.Version)
		case "date":
			err = json.Unmarshal(v, &r.Date)
		case "docs":
			err = json.Unmarshal(v, &r.Docs)
		case "stdDocs":
			err = json.Unmarshal(v, &r.StdDocs)
		case "notes":
			err = json.Unmarshal(v, &r.Notes)
		default:
			var a Artifact
			if json.Unmarshal(v, &a) == nil && a.Tarball != "" {
				r.Artifacts[k] = a
			}
		}
		if err != nil {
			return fmt.Errorf("field %q: %w", k, err)
		}
	}
	return nil
}

func parseReleases(data []byte) (map[string]Release, error) {
	var r map[string]Release
	err := json.Unmarshal(data, &r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func pingUrl(url string) (int, error) {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Head(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

type Downloader struct {
	mirrors []string
}

type PingResult struct {
	Url  string
	Time time.Duration
	Err  error
}

func (d *Downloader) RankMirrors() []PingResult {
	ping_results := make([]PingResult, 0, len(d.mirrors))

	in := make(chan string)
	out := make(chan PingResult)

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for m := range in {
				u, err := getRootURL(m)
				t := time.Now()
				if err == nil {
					_, err = pingUrl(u)
				}
				out <- PingResult{m, time.Since(t), err}
			}
		})
	}

	go func() {
		for _, m := range d.mirrors {
			in <- m
		}
		close(in)
	}()

	go func() {
		wg.Wait()
		close(out)
	}()

	for r := range out {
		ping_results = append(ping_results, r)
	}

	slices.SortFunc(ping_results, func(a, b PingResult) int {
		if a.Err != nil || b.Err != nil {
			if a.Err != nil && b.Err != nil {
				return 0
			}
			if a.Err != nil {
				return 1
			} else {
				return -1
			}

		}
		if a.Time < b.Time {
			return -1
		} else {
			return 1
		}
	})

	for i, v := range ping_results {
		d.mirrors[i] = v.Url
	}

	return ping_results
}

func getRootURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""

	return u.String(), nil
}

func getIndex(mirror string) (map[string]Release, error) {
	u, err := url.Parse(mirror)
	if err != nil {
		return nil, err
	}
	index_url := u.JoinPath("index.json")

	resp, err := http.Get(index_url.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Sc not OK")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	m, err := parseReleases(data)
	return m, err
}

func versionsFromReleases(m map[string]Release) []*Version {
	versions := make([]*Version, 0, len(m))
	for k, _ := range m {
		v, err := NewVersion(k)
		if err == nil {
			versions = append(versions, v)
		}
	}
	slices.SortFunc(versions, cmpVersions)
	return versions
}

func getConfigDir() string {
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

func getShareDir() string {
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

func printHelp() {
	a := `Usage: zvm <action>
Actions:
  - set <zig/zls/toolchain(both)> <version> - set version of zig, zls or both if it is possible
  - get/list <zig/zls/toolchain(both)> - print selected version / list all available versions
  - update - updates to last patch if on stable branch, uses last dev version on master
  - mirrors <update/list> - update mirrors , list them
  e.g: zvm set zig 0.17.0`
	println(a)
}

func doSet(a []string) {

}

func doGet(a []string) {

}

func doList(a []string) {

}

func doUpdate(a []string) {

}

func doMirrors(a []string) {

}

func parseCommand() {
	act := os.Args[1]
	mp := map[string](func([]string)){
		"set":  doSet,
		"get":  doGet,
		"list": doList,
	}

	if f, ok := mp[act]; ok {
		f(os.Args[2:])
	} else {
		printHelp()
	}
}

func main() {
	sysinfo := GetSysInfo()
	println(sysinfo.String())
	mirrors := []string{"https://ziglang.org/download/"}
	mirrors_url := "https://ziglang.org/download/community-mirrors.txt"
	m, err := GetMirrors(mirrors_url)
	if err == nil {
		mirrors = append(mirrors, m...)
	}

	fmt.Println(mirrors)

	d := Downloader{mirrors: mirrors}
	d.RankMirrors()
	StoreMirrorList(d.mirrors)

	releases, err := getIndex(mirrors[0])
	if err == nil {
		for k, _ := range releases {
			println(k)
		}
		r := versionsFromReleases(releases)
		for _, v := range r {
			println(v.String())
		}
	}
}
