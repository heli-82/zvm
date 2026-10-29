package executor

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"zvm/internal/dirs"
	"zvm/internal/download"
	"zvm/internal/extract"
	"zvm/internal/fsutil"
	"zvm/internal/index"
	"zvm/internal/verify"
	"zvm/internal/version"
)

func (e *Executor) GetZlsVersion(v string) error {
	if _, ok := e.LocalVersions.Zls[v]; ok {
		println("Already downloaded")
		return nil
	}
	zigV := v
	if zigV == "master" {
		r, _ := e.GetReleases()
		zigV = r["master"].Version
	}

	rel, err := index.SelectZls(zigV)
	if err != nil {
		return err
	}
	artifact, ok := rel.Artifacts[e.SysInfo.String()]
	if !ok {
		return ReleaseNotExist
	}

	tarballUrl := artifact.Tarball
	tarballName, err := download.FilenameFromURL(tarballUrl)
	if err != nil {
		return err
	}
	tarballPath := filepath.Join(dirs.Share(), tarballName)
	if !fsutil.FileExists(tarballPath) {
		err = download.File(tarballUrl, tarballPath)
		if err != nil {
			return err
		}
	}

	minisigUrl := tarballUrl + ".minisig"

	minisigName, err := download.FilenameFromURL(minisigUrl)
	if err != nil {
		return err
	}
	minisigPath := filepath.Join(dirs.Share(), minisigName)
	if !fsutil.FileExists(minisigPath) {
		err = download.File(minisigUrl, minisigPath)
		if err != nil {
			return err
		}
	}

	publicKey := "RWR+9B91GBZ0zOjh6Lr17+zKf5BoSuFvrx2xSeDE57uIYvnKBGmMjOex"

	println("Verifying downloaded files")

	err = verify.Minisig(tarballPath, minisigPath, publicKey)
	if err != nil {
		return err
	}

	zlsDir, err := extract.TarXz(tarballPath, dirs.Share())
	if err != nil {
		return err
	}

	zlsPath := filepath.Join(zlsDir, "zls")
	exeName := e.ExeName("zls", v)
	binPath := filepath.Join(dirs.Bin(), exeName)
	err = fsutil.Symlink(zlsPath, binPath)
	if err != nil {
		return err
	}

	e.LocalVersions.Zls[v] = zlsDir
	e.LocalVersions.Store()

	return nil
}

func (e *Executor) SetZlsVersion(v string) error {
	if p, ok := e.LocalVersions.Zls[v]; ok {
		target := filepath.Join(dirs.Bin(), "zls")
		source := filepath.Join(p, "zls")
		err := fsutil.Symlink(source, target)
		return err
	} else {
		return WasntDownloadedYet
	}
}

// ActivateZls sets v as the active zls, downloading it first if needed.
func (e *Executor) ActivateZls(v string) error {
	err := e.SetZlsVersion(v)
	if errors.Is(err, WasntDownloadedYet) {
		err = e.GetZlsVersion(v)
	}
	if err == nil {
		zlsPath := filepath.Join(dirs.Bin(), "zls")
		zlsVPath := filepath.Join(dirs.Bin(), e.ExeName("zls", v))
		err = fsutil.Symlink(zlsVPath, zlsPath)
		if err == nil {
			e.Config.ZlsVersion = v
			e.Config.Store()
		}
	}
	return err
}

func (e *Executor) ListZls() {
	out := []*version.Version{}
	for v := range e.LocalVersions.Zls {
		ver, err := version.New(v)
		if err != nil {
			continue
		}
		out = append(out, ver)
	}
	slices.SortFunc(out, version.Cmp)

	for _, v := range out {
		fmt.Printf("[+] %s\n", v.String())
	}
}
