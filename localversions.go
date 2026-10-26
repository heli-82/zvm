package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type LocalVersions struct {
	Zig map[string]string `json:"zig"`
	Zls map[string]string `json:"zls"`
}

func LoadLocalVersons() (l LocalVersions, err error) {
	l = LocalVersions{Zig: map[string]string{}, Zls: map[string]string{}}
	versionspath := filepath.Join(getShareDir(), "versions.json")
	dat, err := os.ReadFile(versionspath)
	if err != nil {
		return l, err
	}
	err = json.Unmarshal(dat, &l)
	if err != nil {
		return l, err
	}
	return l, nil
}

func (l *LocalVersions) Store() error {
	dat, err := json.MarshalIndent(l, "", "\t")
	if err != nil {
		return err
	}
	fpath := filepath.Join(getShareDir(), "versions.json")
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(dat)
	if err != nil {
		return err
	}

	return nil
}
