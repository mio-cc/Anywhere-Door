package secret

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

// kdfParams 写在加密文件头里，以后可以升级参数。
type kdfParams struct {
	Alg  string `json:"alg"`
	M    uint32 `json:"m"`
	T    uint32 `json:"t"`
	P    uint8  `json:"p"`
	Salt string `json:"salt"`
}

// fileDoc 是 secrets.enc 的格式（B.07.02 §5）。
type fileDoc struct {
	V     int       `json:"v"`
	KDF   kdfParams `json:"kdf"`
	Check string    `json:"check"`
	Nonce string    `json:"nonce"`
	Data  string    `json:"data"`
}

const (
	additionalData = "anydoor-secrets-v1"
	checkLabel     = "anydoor-secrets-check"
)

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func derive(password string, k kdfParams) ([]byte, error) {
	if k.Alg != "argon2id" || k.M == 0 || k.T == 0 || k.P == 0 {
		return nil, fmt.Errorf("%w：不认识的密钥派生参数", ErrCorrupt)
	}
	salt, err := base64.StdEncoding.DecodeString(k.Salt)
	if err != nil || len(salt) < 8 {
		return nil, fmt.Errorf("%w：盐不对", ErrCorrupt)
	}
	return argon2.IDKey([]byte(password), salt, k.T, k.M, k.P, chacha20poly1305.KeySize), nil
}

func checkOf(key []byte) string {
	h := sha256.Sum256(append(append([]byte{}, key...), checkLabel...))
	return base64.StdEncoding.EncodeToString(h[:])
}

// Unlock 解锁文件后端；文件不存在时用这个密码新建（首次设置主密码）。系统密钥库后端什么也不做。
func (s *Store) Unlock(ctx context.Context, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kr != nil {
		return nil
	}
	now := s.o.Now()
	if now.Before(s.coolUntil) {
		return ErrTooManyAttempts
	}
	doc, err := s.readDoc()
	if errors.Is(err, fs.ErrNotExist) {
		if password == "" {
			return errors.New("secret: 主密码不能为空")
		}
		salt, err := randomBytes(16)
		if err != nil {
			return err
		}
		hdr := kdfParams{Alg: "argon2id", M: 64 * 1024, T: 3, P: 1, Salt: base64.StdEncoding.EncodeToString(salt)}
		key, err := derive(password, hdr)
		if err != nil {
			return err
		}
		s.key, s.hdr = key, hdr
		if err := s.writeSecrets(map[string]string{}); err != nil {
			s.key = nil
			return err
		}
		s.failures = 0
		return nil
	}
	if err != nil {
		return err
	}
	key, err := derive(password, doc.KDF)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(checkOf(key)), []byte(doc.Check)) != 1 {
		s.failures++
		if s.failures >= maxFailures {
			s.failures = 0
			s.coolUntil = now.Add(cooldown)
		}
		return ErrBadPassword
	}
	if _, err := decrypt(doc, key); err != nil {
		return err
	}
	s.key, s.hdr, s.failures = key, doc.KDF, 0
	return nil
}

func (s *Store) readDoc() (fileDoc, error) {
	var doc fileDoc
	b, err := os.ReadFile(s.d.Secrets())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return doc, err
		}
		return doc, fmt.Errorf("secret: 读不了加密文件：%w", err)
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return doc, fmt.Errorf("%w：%v", ErrCorrupt, err)
	}
	if doc.V != 1 {
		return doc, fmt.Errorf("%w：不认识的版本 %d", ErrCorrupt, doc.V)
	}
	return doc, nil
}

func decrypt(doc fileDoc, key []byte) (map[string]string, error) {
	nonce, err1 := base64.StdEncoding.DecodeString(doc.Nonce)
	ct, err2 := base64.StdEncoding.DecodeString(doc.Data)
	if err1 != nil || err2 != nil || len(nonce) != chacha20poly1305.NonceSizeX {
		return nil, fmt.Errorf("%w：nonce 或密文格式不对", ErrCorrupt)
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	pt, err := aead.Open(nil, nonce, ct, []byte(additionalData))
	if err != nil {
		return nil, fmt.Errorf("%w：解密失败", ErrCorrupt)
	}
	m := map[string]string{}
	if err := json.Unmarshal(pt, &m); err != nil {
		return nil, fmt.Errorf("%w：内容不是 JSON", ErrCorrupt)
	}
	return m, nil
}

// readSecrets 读出整张表；用完即丢，不缓存（S.03-R3）。
func (s *Store) readSecrets() (map[string]string, error) {
	if s.key == nil {
		return nil, ErrLocked
	}
	doc, err := s.readDoc()
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return decrypt(doc, s.key)
}

// writeSecrets 用新的随机 nonce 加密整张表，原子写。
func (s *Store) writeSecrets(m map[string]string) error {
	pt, err := json.Marshal(m)
	if err != nil {
		return err
	}
	nonce, err := randomBytes(chacha20poly1305.NonceSizeX)
	if err != nil {
		return err
	}
	aead, err := chacha20poly1305.NewX(s.key)
	if err != nil {
		return err
	}
	doc := fileDoc{V: 1, KDF: s.hdr, Check: checkOf(s.key),
		Nonce: base64.StdEncoding.EncodeToString(nonce),
		Data:  base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, pt, []byte(additionalData)))}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := datadir.WritePrivate(s.d.Secrets(), append(b, '\n')); err != nil {
		return fmt.Errorf("secret: 写不了加密文件：%w", err)
	}
	return nil
}
