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
	"zvm/internal/verify"
	"zvm/internal/version"
)

func (e *Executor) GetZigVersion(v string) error {
	if _, ok := e.LocalVersions.Zig[v]; ok {
		println("Already downloaded")
		return nil
	}
	r, err := e.GetReleases()
	if err != nil {
		return err
	}
	release, ok := r[v]
	if !ok {
		return VersionNotExist
	}
	artifact, ok := release.Artifacts[e.SysInfo.String()]
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

	publicKey := "RWSGOq2NVecA2UPNdBUZykf1CCb147pkmdtYxgb3Ti+JO/wCYvhbAb/U"

	println("Verifying downloaded files")

	err = verify.Minisig(tarballPath, minisigPath, publicKey)
	if err != nil {
		return err
	}

	zigDir, err := extract.TarXz(tarballPath, dirs.Share())
	if err != nil {
		return err
	}

	zigPath := filepath.Join(zigDir, "zig")
	exeName := e.ExeName("zig", v)
	binPath := filepath.Join(dirs.Bin(), exeName)
	err = fsutil.Symlink(zigPath, binPath)
	if err != nil {
		return err
	}

	e.LocalVersions.Zig[v] = zigDir
	e.LocalVersions.Store()

	return nil
}

func (e *Executor) SetZigVersion(v string) error {
	if p, ok := e.LocalVersions.Zig[v]; ok {
		target := filepath.Join(dirs.Bin(), "zig")
		source := filepath.Join(p, "zig")
		err := fsutil.Symlink(source, target)
		return err
	} else {
		return WasntDownloadedYet
	}
}

// ActivateZig sets v as the active zig, downloading it first if needed.
func (e *Executor) ActivateZig(v string) error {
	err := e.SetZigVersion(v)
	if errors.Is(err, WasntDownloadedYet) {
		err = e.GetZigVersion(v)
	}
	if err == nil {
		zigPath := filepath.Join(dirs.Bin(), "zig")
		zigVPath := filepath.Join(dirs.Bin(), e.ExeName("zig", v))
		err = fsutil.Symlink(zigVPath, zigPath)
		if err == nil {
			e.Config.ZigVersion = v
			e.Config.Store()
		}
	}
	return err
}

func (e *Executor) ListZig() {
	out := []*version.Version{}
	for v := range e.LocalVersions.Zig {
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
