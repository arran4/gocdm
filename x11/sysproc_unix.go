//go:build !windows

package x11

import (
	"os"
	"syscall"
)

func newSysProcAttrWithCred(uid, gid *int, groups []int) *syscall.SysProcAttr {
	attr := &syscall.SysProcAttr{Setsid: true}
	if uid != nil && gid != nil && os.Geteuid() == 0 {
		gids := make([]uint32, 0, len(groups)+1)
		for _, g := range groups {
			if g >= 0 {
				gids = append(gids, uint32(g))
			}
		}
		if len(gids) == 0 && *gid >= 0 {
			gids = append(gids, uint32(*gid))
		}
		attr.Credential = &syscall.Credential{
			Uid:    uint32(*uid),
			Gid:    uint32(*gid),
			Groups: gids,
		}
	}
	return attr
}
