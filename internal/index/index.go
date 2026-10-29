package index

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

	"zvm/internal/version"
)

const ZlsSelectURL = "https://releases.zigtools.org/v1/zls/select-version"

var HTTPClient = &http.Client{Timeout: 10 * time.Second}

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

func Parse(data []byte) (map[string]Release, error) {
	var r map[string]Release
	err := json.Unmarshal(data, &r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Get fetches and parses index.json from a mirror.
func Get(mirror string) (map[string]Release, error) {
	u, err := url.Parse(mirror)
	if err != nil {
		return nil, err
	}
	resp, err := HTTPClient.Get(u.JoinPath("index.json").String())
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
	return Parse(data)
}

func Versions(m map[string]Release) []*version.Version {
	versions := make([]*version.Version, 0, len(m))
	for k := range m {
		v, err := version.New(k)
		if err == nil {
			versions = append(versions, v)
		}
	}
	slices.SortFunc(versions, version.Cmp)
	return versions
}

// SelectZls asks zigtools which ZLS release is compatible with zigVersion.
func SelectZls(zigVersion string) (Release, error) {
	q := url.Values{"zig_version": {zigVersion}, "compatibility": {"full"}}
	resp, err := HTTPClient.Get(ZlsSelectURL + "?" + q.Encode())
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Release{}, err
	}

	var r Release
	if err := json.Unmarshal(data, &r); err != nil {
		return r, err
	}
	if r.Version == "" { // error response: {"code":..,"message":..}
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &e)
		return r, fmt.Errorf("zls select-version (zig %s): %s", zigVersion, e.Message)
	}
	return r, nil
}
