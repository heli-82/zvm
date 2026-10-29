package fsutil

import "os"

func FileExists(filePath string) bool {
	stat, err := os.Stat(filePath)
	if err == nil {
		return !stat.IsDir()
	}
	return false
}

func Symlink(target, link string) error {
	tmp := link + ".tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, link)
}
