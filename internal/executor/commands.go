package executor

import (
	"fmt"
	"os"

	"zvm/internal/mirrors"
)

func (e *Executor) ParseCommand() {
	if len(os.Args) == 1 {
		e.DoHelp(os.Args)
		return
	}
	act := os.Args[1]
	mp := map[string](func([]string) error){
		"set":     e.DoSet,
		"get":     e.DoGet,
		"remove":  e.DoRemove,
		"list":    e.DoList,
		"update":  e.DoUpdate,
		"mirrors": e.DoMirrors,
		"help":    e.DoHelp,
	}

	if f, ok := mp[act]; ok {
		err := f(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "zvm:", err)
			os.Exit(1)
		}
	} else {
		println("Invalid command: ", act)
		PrintHelp()
	}
}

func (e *Executor) DoGet(a []string) error {
	if len(a) < 2 {
		return NotEnoughArgs
	}
	tool := a[0]
	version := a[1]
	switch tool {
	case "zig":
		return e.GetZigVersion(version)
	case "zls":
		return e.GetZlsVersion(version)
	case "toolchain":
		err := e.GetZigVersion(version)
		if err != nil {
			return err
		}
		err = e.GetZlsVersion(version)
		return err
	default:
		return InvalidArgument
	}
}

func (e *Executor) DoSet(a []string) error {
	if len(a) < 2 {
		return NotEnoughArgs
	}
	tool := a[0]
	version := a[1]
	switch tool {
	case "zig":
		return e.ActivateZig(version)
	case "zls":
		return e.ActivateZls(version)
	case "toolchain":
		err := e.ActivateZig(version)
		if err != nil {
			return err
		}
		err = e.ActivateZls(version)
		return err
	default:
		return InvalidArgument
	}
}

func (e *Executor) DoList(a []string) error {
	if len(a) == 0 {
		return NotEnoughArgs
	}
	tool := a[0]
	switch tool {
	case "zig":
		e.ListZig()
	case "zls":
		e.ListZls()
	case "toolchain":
		println("Zig:")
		e.ListZig()
		println("Zls:")
		e.ListZls()
	}
	return nil
}

func (e *Executor) DoRemove(a []string) error {
	return nil
}

func (e *Executor) DoUpdate(a []string) error {
	println("TODO")
	return nil
}

func (e *Executor) DoMirrors(a []string) error {
	n := len(a)
	if n == 0 {
		return NotEnoughArgs
	}
	act := a[0]
	switch act {
	case "list":

	case "update":
		set := map[string]struct{}{
			mirrors.MainMirror: {},
		}
		urls := map[string]struct{}{
			mirrors.CommunityList: {},
		}
		for i := 1; i < n; i++ {
			urls[a[i]] = struct{}{}
		}
		for u := range urls {
			m, err := mirrors.Fetch(u)
			if err != nil {
				continue
			}
			for _, mirror := range m {
				set[mirror] = struct{}{}
			}
		}
		m := []string{}
		for mirror := range set {
			m = append(m, mirror)
		}
		e.Downloader.Mirrors = m
		fallthrough

	case "rank":
		results := e.Downloader.RankMirrors()
		for _, res := range results {
			sym := '+'
			if res.Err != nil {
				sym = '-'
			}
			fmt.Printf("[%c] %s\n", sym, res.Url)
		}
		err := mirrors.Store(e.Downloader.Mirrors)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Executor) DoHelp([]string) error {
	PrintHelp()
	return nil
}

func PrintHelp() {
	a := `Usage: zvm <action>
Actions:
  - set <zig/zls/toolchain(both)> <version> - set version to zig, zls or both if it is possible
  - get <zig/zls/toolchain(both)> <version> - get version to zig, zls or both if it is possible
  - remove <zig/zls/toolchain(both)> <version> - remove version of zig, zls or both, setting zig to last local version
  - list <zig/zls/toolchain(both)> - list all versions
  - update - updates to last patch if on stable branch, uses last dev version if on master
  - mirrors update [mirror urls...] - update mirrors adding new mirrors to list using urls to lists
  			list - list them
     		rank - order mirrors by lower ping
  e.g: zvm set zig 0.17.0`
	println(a)
}
