package settings_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/host"
	"github.com/timonwong/nsp-carrier/internal/settings"
)

func TestProfileSettingsDefaultAndMigrationFallbackToDBI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store := settings.FileStore{Path: path}
	for _, test := range []struct {
		name    string
		content string
	}{
		{name: "missing"},
		{name: "legacy without profile", content: `{"theme":"dark"}`},
		{name: "invalid profile", content: `{"profile":"automatic"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.content == "" {
				_ = os.Remove(path)
			} else if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			profile, err := store.LoadProfile()
			if err != nil || profile != host.ProfileDBI {
				t.Fatalf("LoadProfile() = %q, %v", profile, err)
			}
		})
	}
}

func TestProfileSettingsPersistStableProfileID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")
	store := settings.FileStore{Path: path}
	if err := store.SaveProfile(host.ProfileGoldleaf); err != nil {
		t.Fatal(err)
	}
	profile, err := store.LoadProfile()
	if err != nil || profile != host.ProfileGoldleaf {
		t.Fatalf("LoadProfile() = %q, %v", profile, err)
	}
}
