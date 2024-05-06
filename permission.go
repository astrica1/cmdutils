package cmdutils

import (
	"fmt"
	"strconv"
)

type PermissionMode uint16

const (
	Perm_rwx PermissionMode = 7
	Perm_rwo PermissionMode = 6
	Perm_rox PermissionMode = 5
	Perm_roo PermissionMode = 4
	Perm_owx PermissionMode = 3
	Perm_owo PermissionMode = 2
	Perm_oox PermissionMode = 1
	Perm_ooo PermissionMode = 0
)

func mergePerm(owner, group, other PermissionMode) (uint16, error) {
	res, err := strconv.ParseUint(fmt.Sprintf("%v%v%v", owner, group, other), 10, 32)
	if err != nil {
		return 0, err
	}

	return uint16(res), nil
}
