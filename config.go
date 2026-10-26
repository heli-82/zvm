package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ZigVersion string `json:"zig"`
	ZlsVersion string `json:"zls"`
}

func LoadConfig() (c Config, err error) {
	configpath := filepath.Join(getConfigDir(), "config.json")
	dat, err := os.ReadFile(configpath)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(dat, &c)
	if err != nil {
		return c, err
	}
	return c, nil
}

func (c *Config) Store() error {
	dat, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		return err
	}
	fpath := filepath.Join(getConfigDir(), "config.json")
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
