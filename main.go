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
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/jedisct1/go-minisign"
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

func parseIndex(data []byte) (map[string]Release, error) {
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

func DownloadFile(url, path string) error {
	name, _ := getFilenameFromURL(url)
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

	p := newProgress("Downloading "+name, resp.ContentLength)
	_, err = io.Copy(io.MultiWriter(out, p), resp.Body)
	p.finish(err)
	return err
}

var InvalidSignature = errors.New("Invalid signature")
var InvalidComment = errors.New("Invalid comment")

func VerifyMinisig(filePath, minisigPath, publicKey string) error {
	pk, err := minisign.NewPublicKey(publicKey)
	if err != nil {
		return err
	}

	sigdata, err := os.ReadFile(minisigPath)
	if err != nil {
		return err
	}

	signature, err := minisign.DecodeSignature(string(sigdata))
	if err != nil {
		return err
	}
	valid, err := pk.VerifyFromFile(filePath, signature)
	if err != nil {
		return err
	}

	if !valid {
		return InvalidSignature
	}

	re := regexp.MustCompile(`^trusted comment: timestamp:\d+\tfile:(.*)\thashed$`)
	match := re.FindStringSubmatch(signature.TrustedComment)
	if len(match) != 2 {
		return errors.New("Trusted comment does not match")
	}
	if match[1] != filepath.Base(filePath) {
		return errors.New("Invalid filename")
	}

	return nil
}

func getFilenameFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return path.Base(u.Path), nil
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

var httpClient = &http.Client{Timeout: 10 * time.Second}

func getIndex(mirror string) (map[string]Release, error) {
	u, err := url.Parse(mirror)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Get(u.JoinPath("index.json").String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: status %d", mirror, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseIndex(data)
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

func main() {
	e, err := NewExecutor()
	if err != nil {
		fmt.Fprintln(os.Stderr, "zvm:", err)
		os.Exit(1)
	}
	defer e.deinit()
	e.parseCommand()
}
