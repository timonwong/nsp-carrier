package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/timonwong/nsp-carrier/internal/host"
)

type ProfileStore interface {
	LoadProfile() (host.ProfileID, error)
	SaveProfile(host.ProfileID) error
}

type FileStore struct {
	Path string
}

func DefaultStore() (FileStore, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return FileStore{}, err
	}
	return FileStore{Path: filepath.Join(root, "nsp-carrier", "settings.json")}, nil
}

func (s FileStore) LoadProfile() (host.ProfileID, error) {
	content, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return host.ProfileDBI, nil
	}
	if err != nil {
		return host.ProfileDBI, err
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(content, &values); err != nil {
		return host.ProfileDBI, err
	}
	var id host.ProfileID
	if value, ok := values["profile"]; ok {
		_ = json.Unmarshal(value, &id)
	}
	if _, ok := host.ProfileByID(id); !ok {
		return host.ProfileDBI, nil
	}
	return id, nil
}

func (s FileStore) SaveProfile(id host.ProfileID) error {
	if _, ok := host.ProfileByID(id); !ok {
		return fmt.Errorf("%w: %q", host.ErrUnknownProfile, id)
	}
	values := make(map[string]json.RawMessage)
	if content, err := os.ReadFile(s.Path); err == nil {
		if err := json.Unmarshal(content, &values); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	encodedID, err := json.Marshal(id)
	if err != nil {
		return err
	}
	values["profile"] = encodedID
	content, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.Path), ".settings-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, s.Path)
}
