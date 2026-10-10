package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"slices"
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

type SysInfo struct {
	arch string
	os   string
}

func (s SysInfo) String() string {
	return fmt.Sprintf("%s-%s", s.arch, s.os)
}

func getSysInfo() SysInfo {
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

func main() {
	sysinfo := getSysInfo()
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
}
