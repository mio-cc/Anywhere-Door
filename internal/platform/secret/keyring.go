package secret

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// systemKeyring 是系统密钥库（Windows 凭据管理器、macOS 钥匙串、Linux Secret Service）。
type systemKeyring struct{}

func (systemKeyring) Get(service, user string) (string, error) { return keyring.Get(service, user) }
func (systemKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}
func (systemKeyring) Delete(service, user string) error { return keyring.Delete(service, user) }

// keyringErr 把"没有这个条目"映射为 ErrNotFound，其余包装（不含密钥值）。
func (s *Store) keyringErr(name string, err error) error {
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("secret: 系统密钥库读不了 %s：%w", name, err)
}
