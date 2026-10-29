package executor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"zvm/internal/config"
	"zvm/internal/dirs"
	"zvm/internal/download"
	"zvm/internal/index"
	"zvm/internal/localversions"
	"zvm/internal/mirrors"
	"zvm/internal/sysinfo"
)

var UnableToLockProcess = errors.New("Unable to create lock file")

var NotEnoughArgs = errors.New("Not enough arguments")
var InvalidArgument = errors.New("Invalid argiment")
var VersionNotExist = errors.New("Version not exist")
var ReleaseNotExist = errors.New("Release not exist")
var WasntDownloadedYet = errors.New("Wasn't downloaded yet")

type Executor struct {
	Downloader    download.Downloader
	LockF         *os.File
	SysInfo       sysinfo.Info
	LocalVersions localversions.LocalVersions
	Config        config.Config
}

func New() (e Executor, err error) {
	m, err := mirrors.Read()
	if err != nil {
		m = mirrors.Default()
	}
	lv, _ := localversions.Load()
	cfg, _ := config.Load()
	e = Executor{
		Downloader:    download.Downloader{Mirrors: m},
		LockF:         nil,
		SysInfo:       sysinfo.Get(),
		LocalVersions: lv,
		Config:        cfg,
	}
	err = e.LockExec()
	if err != nil {
		return e, err
	}
	return e, nil
}

func (e *Executor) LockExec() error {
	f, err := os.OpenFile(filepath.Join(dirs.Share(), "zvm.lock"), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return UnableToLockProcess
	}
	e.LockF = f
	return nil
}

func (e *Executor) Deinit() {
	if e.LockF != nil {
		e.LockF.Close()
	}
}

func (e *Executor) ExeName(prog, version string) string {
	if version == "master" {
		return prog + "-master"
	}
	return prog + version
}

func (e *Executor) GetReleases() (map[string]index.Release, error) {
	var lastErr error
	for _, m := range e.Downloader.Mirrors {
		fmt.Printf("Trying %s\n", m)
		idx, err := index.Get(m)
		if err == nil {
			return idx, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no mirrors configured")
	}
	return nil, lastErr
}
