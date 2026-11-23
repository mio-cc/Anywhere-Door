//go:build windows

package datadir

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileAllAccess 是 FILE_ALL_ACCESS（STANDARD_RIGHTS_REQUIRED | SYNCHRONIZE | 0x1FF）。
// 不用 GENERIC_ALL：目录上带继承标志的通用权限会被系统拆成两条（一条映射后的、一条只继承的）。
const fileAllAccess = windows.ACCESS_MASK(0x1F01FF)

// currentUser 是当前进程的用户 SID。
func currentUser() (*windows.SID, error) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return tu.User.Sid.Copy()
}

// Restrict 把 path 的 DACL 设为受保护（不继承上级）、只有一条"当前用户完全控制"；目录上这一条可被继承。
func Restrict(p string) error {
	fi, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("datadir: 收紧权限 %s：%w", p, err)
	}
	sid, err := currentUser()
	if err != nil {
		return fmt.Errorf("datadir: 收紧权限 %s：取当前用户：%w", p, err)
	}
	inherit := uint32(windows.NO_INHERITANCE)
	if fi.IsDir() {
		inherit = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: fileAllAccess,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inherit,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}}, nil)
	if err != nil {
		return fmt.Errorf("datadir: 收紧权限 %s：%w", p, err)
	}
	err = windows.SetNamedSecurityInfo(p, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
	if err != nil {
		return fmt.Errorf("datadir: 收紧权限 %s：%w", p, err)
	}
	return nil
}

// IsPrivate 判断 path 的 DACL 是否只有当前用户的允许项。
func IsPrivate(p string) (bool, error) {
	me, err := currentUser()
	if err != nil {
		return false, err
	}
	sd, err := windows.GetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false, err
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return false, err
	}
	n := int(dacl.AceCount)
	for i := 0; i < n; i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(i), &ace); err != nil {
			return false, err
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return false, nil
		}
		if !(*windows.SID)(unsafe.Pointer(&ace.SidStart)).Equals(me) {
			return false, nil
		}
	}
	return n > 0, nil
}
