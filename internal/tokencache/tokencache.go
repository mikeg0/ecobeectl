package tokencache

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/99designs/keyring"
)

var ErrNotFound = errors.New("token not found")

type CachedToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	AccountID    string    `json:"account_id,omitempty"`
}

type Identity struct {
	AuthURL  string
	ClientID string
	Email    string
}

type Store interface {
	Save(Identity, CachedToken) error
	Load(Identity) (*CachedToken, error)
	Clear(Identity) error
}

type Manager struct {
	primary  Store
	fallback Store
}

func New(serviceName, fileDir string) Store {
	return &Manager{
		primary:  newKeyringStore(serviceName),
		fallback: newFileStore(fileDir),
	}
}

func (m *Manager) Save(id Identity, token CachedToken) error {
	if m.primary != nil {
		if err := m.primary.Save(id, token); err == nil {
			return nil
		}
	}
	return m.fallback.Save(id, token)
}

func (m *Manager) Load(id Identity) (*CachedToken, error) {
	if m.primary != nil {
		token, err := m.primary.Load(id)
		if err == nil {
			return token, nil
		}
		if !errors.Is(err, ErrNotFound) {
			if token, fallbackErr := m.fallback.Load(id); fallbackErr == nil {
				return token, nil
			}
			return nil, err
		}
	}
	return m.fallback.Load(id)
}

func (m *Manager) Clear(id Identity) error {
	var errs []string
	if m.primary != nil {
		if err := m.primary.Clear(id); err != nil && !errors.Is(err, ErrNotFound) {
			errs = append(errs, err.Error())
		}
	}
	if err := m.fallback.Clear(id); err != nil && !errors.Is(err, ErrNotFound) {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

type keyringStore struct {
	once sync.Once
	ring keyring.Keyring
	err  error
	name string
}

func newKeyringStore(serviceName string) Store {
	return &keyringStore{name: serviceName}
}

func (s *keyringStore) open() (keyring.Keyring, error) {
	s.once.Do(func() {
		backends := make([]keyring.BackendType, 0, len(keyring.AvailableBackends()))
		for _, backend := range keyring.AvailableBackends() {
			if backend == keyring.FileBackend || backend == keyring.PassBackend {
				continue
			}
			backends = append(backends, backend)
		}
		if len(backends) == 0 {
			s.err = keyring.ErrNoAvailImpl
			return
		}
		s.ring, s.err = keyring.Open(keyring.Config{
			ServiceName:     s.name,
			AllowedBackends: backends,
		})
	})
	return s.ring, s.err
}

func (s *keyringStore) Save(id Identity, token CachedToken) error {
	ring, err := s.open()
	if err != nil {
		return err
	}
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return ring.Set(keyring.Item{
		Key:  cacheKey(id),
		Data: data,
	})
}

func (s *keyringStore) Load(id Identity) (*CachedToken, error) {
	ring, err := s.open()
	if err != nil {
		return nil, err
	}
	item, err := ring.Get(cacheKey(id))
	if err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var token CachedToken
	if err := json.Unmarshal(item.Data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *keyringStore) Clear(id Identity) error {
	ring, err := s.open()
	if err != nil {
		return err
	}
	if err := ring.Remove(cacheKey(id)); err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

type fileStore struct {
	dir string
}

func newFileStore(dir string) Store {
	return &fileStore{dir: dir}
}

func (s *fileStore) Save(id Identity, token CachedToken) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(id), data, 0o600)
}

func (s *fileStore) Load(id Identity) (*CachedToken, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var token CachedToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *fileStore) Clear(id Identity) error {
	if err := os.Remove(s.path(id)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *fileStore) path(id Identity) string {
	return filepath.Join(s.dir, url.PathEscape(cacheKey(id))+".json")
}

func cacheKey(id Identity) string {
	return fmt.Sprintf("ecobeectl-token:%s|%s|%s", id.AuthURL, id.ClientID, strings.ToLower(strings.TrimSpace(id.Email)))
}
