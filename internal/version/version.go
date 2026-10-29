package version

import "fmt"

type Version struct {
	Master              bool
	Major, Minor, Patch uint32
}

func New(s string) (*Version, error) {
	var v Version
	if s == "master" {
		v.Master = true
		return &v, nil
	}
	_, err := fmt.Sscanf(s, "%d.%d.%d", &v.Major, &v.Minor, &v.Patch)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (v *Version) String() string {
	if v.Master {
		return "master"
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Cmp orders versions newest first, master before everything.
func Cmp(a, b *Version) int {
	if a.Master && b.Master {
		return 0
	}
	if a.Master {
		return -1
	}
	if b.Master {
		return 1
	}
	av := [3]uint32{a.Major, a.Minor, a.Patch}
	bv := [3]uint32{b.Major, b.Minor, b.Patch}

	for i := range 3 {
		avm := av[i]
		bvm := bv[i]
		if avm == bvm {
			continue
		}
		if avm < bvm {
			return 1
		} else {
			return -1
		}
	}
	return 0
}
