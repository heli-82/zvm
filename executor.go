package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Executor struct {
	downloader    Downloader
	lockf         *os.File
	sysinfo       SysInfo
	localversions LocalVersions
	config        Config
}

func (e *Executor) deinit() {
	e.lockf.Close()
	lockfp := filepath.Join(getShareDir(), "zvm.lock")
	_ = os.Remove(lockfp)
}

func NewExecutor() (e Executor, err error) {
	m, err := ReadMirrorList()
	if err != nil {
		m = GetDefaultMirrorList()
	}
	localversions, _ := LoadLocalVersons()
	config, _ := LoadConfig()
	e = Executor{
		downloader:    Downloader{mirrors: m},
		lockf:         nil,
		sysinfo:       GetSysInfo(),
		localversions: localversions,
		config:        config,
	}
	err = e.lockExec()
	if err != nil {
		return e, err
	}
	return e, nil
}

var UnableToLockProcess = errors.New("Unable to create lock file")

func (e *Executor) lockExec() error {
	lockfp := filepath.Join(getShareDir(), "zvm.lock")
	_, err := os.Stat(lockfp)
	if err != nil {
		lockf, err := os.Create(lockfp)
		if err != nil {
			return err
		}
		e.lockf = lockf
		return nil
	}
	return UnableToLockProcess
}

var NotEnoughArgs = errors.New("Not enough arguments")
var InvalidArgument = errors.New("Invalid argiment")
var VersionNotExist = errors.New("Version not exist")
var ReleaseNotExist = errors.New("Release not exist")
var WasntDownloadedYet = errors.New("Wasn't downloaded yet")

func (e *Executor) setZigVersion(v string) error {
	if p, ok := e.localversions.Zig[v]; ok {
		target := filepath.Join(getBinDir(), "zig")
		source := filepath.Join(p, "zig")
		err := os.Symlink(source, target)
		return err
	} else {
		return WasntDownloadedYet
	}
}

func (e *Executor) setZlsVersion(v string) error {
	if p, ok := e.localversions.Zls[v]; ok {
		target := filepath.Join(getBinDir(), "zls")
		source := filepath.Join(p, "zls")
		err := os.Symlink(source, target)
		return err
	} else {
		return WasntDownloadedYet
	}
}

func (e *Executor) getReleases() map[string]Release {
	var releases map[string]Release
	for _, m := range e.downloader.mirrors {
		index, err := getIndex(m)
		if err == nil {
			releases = index
		}
	}
	return releases
}

func (e *Executor) getZigVersion(v string) error {
	if _, ok := e.localversions.Zig[v]; ok {
		println("Already downloaded")
		return nil
	}

	release, ok := e.getReleases()[v]
	if !ok {
		return VersionNotExist
	}
	artifact, ok := release.Artifacts[e.sysinfo.String()]
	if !ok {
		return ReleaseNotExist
	}

	tarballUrl := artifact.Tarball
	tarballName, err := getFilenameFromURL(tarballUrl)
	if err != nil {
		return err
	}
	tarballPath := filepath.Join(getShareDir(), tarballName)

	err = DownloadFile(tarballUrl, tarballPath)
	if err != nil {
		return err
	}

	minisigUrl := tarballUrl + ".minisig"

	minisigName, err := getFilenameFromURL(minisigUrl)
	if err != nil {
		return err
	}
	minisigPath := filepath.Join(getShareDir(), minisigName)

	err = DownloadFile(minisigUrl, minisigPath)
	if err != nil {
		return err
	}

	publicKey := "RWSGOq2NVecA2UPNdBUZykf1CCb147pkmdtYxgb3Ti+JO/wCYvhbAb/U"

	err = VerifyMinisig(tarballPath, minisigPath, publicKey)
	if err != nil {
		return err
	}

	zigDir, err := ExtractTarXz(tarballPath, getShareDir())
	if err != nil {
		return err
	}

	zigPath := filepath.Join(zigDir, "zig")
	exeName := e.getExeName("zig", v)
	binPath := filepath.Join(getBinDir(), exeName)
	err = os.Symlink(zigPath, binPath)
	if err != nil {
		return err
	}

	e.localversions.Zig[v] = zigDir
	e.localversions.Store()

	return nil
}

func (e *Executor) getExeName(prog, version string) string {
	if version == "master" {
		return prog + "-master"
	}
	return prog + version
}

func (e *Executor) getZlsVersion(v string) error {
	if _, ok := e.localversions.Zls[v]; ok {
		println("Already downloaded")
		return nil
	}
	if v == "master" {
		v = e.getReleases()[v].Version
	}

	// "https://builds.zigtools.org/zls-x86_64-linux-0.17.0.tar.xz"
	url := "https://builds.zigtools.org/zls"

	tarballUrl := fmt.Sprintf("%s-%s-%s.tar.xz", url, e.sysinfo.String(), v)
	tarballName, err := getFilenameFromURL(tarballUrl)
	if err != nil {
		return err
	}
	tarballPath := filepath.Join(getShareDir(), tarballName)

	err = DownloadFile(tarballUrl, tarballPath)
	if err != nil {
		return err
	}

	minisigUrl := tarballUrl + ".minisig"

	minisigName, err := getFilenameFromURL(minisigUrl)
	if err != nil {
		return err
	}
	minisigPath := filepath.Join(getShareDir(), minisigName)

	err = DownloadFile(minisigUrl, minisigPath)
	if err != nil {
		return err
	}

	publicKey := "RWR+9B91GBZ0zOjh6Lr17+zKf5BoSuFvrx2xSeDE57uIYvnKBGmMjOex"

	err = VerifyMinisig(tarballPath, minisigPath, publicKey)
	if err != nil {
		return err
	}

	zlsDir, err := ExtractTarXz(tarballPath, getShareDir())
	if err != nil {
		return err
	}

	zlsPath := filepath.Join(zlsDir, "zls")
	exeName := e.getExeName("zls", v)
	binPath := filepath.Join(getBinDir(), exeName)
	err = os.Symlink(zlsPath, binPath)
	if err != nil {
		return err
	}

	e.localversions.Zls[v] = zlsDir
	e.localversions.Store()

	return nil
}

func (e *Executor) doSet(a []string) error {
	if len(a) < 2 {
		return NotEnoughArgs
	}
	tool := a[0]
	version := a[1]
	setZig := func(v string) error {
		err := e.setZigVersion(v)
		if err == nil {
			return nil
		}
		if errors.Is(err, WasntDownloadedYet) {
			err = e.getZigVersion(v)
			if err == nil {
				zigPath := filepath.Join(getBinDir(), "zig")
				zigVPath := filepath.Join(getBinDir(), e.getExeName("zig", v))
				err = os.Symlink(zigVPath, zigPath)
			}
		}
		return err
	}
	setZls := func(v string) error {
		err := e.setZlsVersion(v)
		if err == nil {
			return nil
		}
		if errors.Is(err, WasntDownloadedYet) {
			err = e.getZlsVersion(v)
			if err == nil {
				zlsPath := filepath.Join(getBinDir(), "zls")
				zlsVPath := filepath.Join(getBinDir(), e.getExeName("zls", v))
				err = os.Symlink(zlsVPath, zlsPath)
			}
		}
		return err
	}
	switch tool {
	case "zig":
		return setZig(version)
	case "zls":
		return setZls(version)
	case "toolchain":
		err := setZig(version)
		if err != nil {
			return err
		}
		err = setZls(version)
		return err
	default:
		return InvalidArgument
	}
}

func (e *Executor) doGet(a []string) error {
	if len(a) < 2 {
		return NotEnoughArgs
	}
	tool := a[0]
	version := a[1]
	switch tool {
	case "zig":
		return e.getZigVersion(version)
	case "zls":
		return e.getZlsVersion(version)
	case "toolchain":
		err := e.getZigVersion(version)
		if err != nil {
			return err
		}
		err = e.getZlsVersion(version)
		return err
	default:
		return InvalidArgument
	}
}

func (e *Executor) doList(a []string) error {
	if len(a) == 0 {
		return NotEnoughArgs
	}
	listZig := func() {
		for v, _ := range e.localversions.Zig {
			fmt.Printf("[+] %s\n", v)
		}
	}
	listZls := func() {
		for v, _ := range e.localversions.Zls {
			fmt.Printf("[+] %s\n", v)
		}
	}
	tool := a[0]
	switch tool {
	case "zig":
		listZig()
	case "zls":
		listZls()
	case "toolchain":
		println("Zig:")
		listZig()
		println("Zls:")
		listZls()
	}
	return nil
}

func (e *Executor) doUpdate(a []string) error {
	println("TODO")
	return nil
}

func (e *Executor) doMirrors(a []string) error {
	n := len(a)
	if n == 0 {
		return NotEnoughArgs
	}
	act := a[0]
	switch act {
	case "list":

	case "update":
		mirrors := map[string]struct{}{
			"https://ziglang.org/download/": {},
		}
		mirror_urls := map[string]struct{}{
			"https://ziglang.org/download/community-mirrors.txt": {},
		}
		for i := 1; i < n; i++ {
			mirror_urls[a[i]] = struct{}{}
		}
		for mirror_url, _ := range mirror_urls {
			m, err := GetMirrors(mirror_url)
			if err != nil {
				continue
			}
			for _, mirror := range m {
				mirrors[mirror] = struct{}{}
			}
		}
		m := []string{}
		for mirror, _ := range mirrors {
			m = append(m, mirror)
		}
		e.downloader.mirrors = m
		fallthrough

	case "rank":
		results := e.downloader.RankMirrors()
		for _, res := range results {
			sym := '+'
			if res.Err != nil {
				sym = '-'
			}
			fmt.Printf("[%c] %s", sym, res.Url)
		}
		err := StoreMirrorList(e.downloader.mirrors)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Executor) doHelp([]string) error {
	printHelp()
	return nil
}

func (e *Executor) parseCommand() {
	if len(os.Args) == 1 {
		e.doHelp(os.Args)
		return
	}
	act := os.Args[1]
	mp := map[string](func([]string) error){
		"set":     e.doSet,
		"get":     e.doGet,
		"list":    e.doList,
		"update":  e.doUpdate,
		"mirrors": e.doMirrors,
		"help":    e.doHelp,
	}

	if f, ok := mp[act]; ok {
		err := f(os.Args[2:])
		if err != nil {
			println(err.Error())
		}
	} else {
		println("Invalid command: ", act)
		printHelp()
	}
}

func printHelp() {
	a := `Usage: zvm <action>
Actions:
  - set <zig/zls/toolchain(both)> <version> - set version to zig, zls or both if it is possible
  - get <zig/zls/toolchain(both)> <version> - get version to zig, zls or both if it is possible
  - list <zig/zls/toolchain(both)> - list all versions
  - update - updates to last patch if on stable branch, uses last dev version if on master
  - mirrors update [mirror urls...] - update mirrors adding new mirrors to list using urls to lists
  			list - list them
     		rank - order mirrors by lower ping
  e.g: zvm set zig 0.17.0`
	println(a)
}
