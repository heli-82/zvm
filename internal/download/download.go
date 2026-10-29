package download

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"sync"
	"time"

	"zvm/internal/progress"
)

type Downloader struct {
	Mirrors []string
}

type PingResult struct {
	Url  string
	Time time.Duration
	Err  error
}

// URL helpers

func FilenameFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return path.Base(u.Path), nil
}

func RootURL(rawURL string) (string, error) {
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

// Mirror ranking

func Ping(url string) (int, error) {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Head(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func (d *Downloader) RankMirrors() []PingResult {
	ping_results := make([]PingResult, 0, len(d.Mirrors))

	in := make(chan string)
	out := make(chan PingResult)

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for m := range in {
				u, err := RootURL(m)
				t := time.Now()
				if err == nil {
					_, err = Ping(u)
				}
				out <- PingResult{m, time.Since(t), err}
			}
		})
	}

	go func() {
		for _, m := range d.Mirrors {
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
		d.Mirrors[i] = v.Url
	}

	return ping_results
}

// File downloads url to path with a progress bar.
func File(url, path string) error {
	name, _ := FilenameFromURL(url)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	p := progress.New("Downloading "+name, resp.ContentLength)
	_, err = io.Copy(io.MultiWriter(out, p), resp.Body)
	p.Finish(err)
	return err
}
