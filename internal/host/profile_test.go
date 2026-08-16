package host_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
)

func TestProfileRegistryIsTheImmutableCapabilitySourceOfTruth(t *testing.T) {
	profiles := host.Profiles()
	want := []host.Profile{
		{
			ID: host.ProfileDBI, DisplayName: "DBI", ProtocolFamily: "DBI0",
			Transport: host.TransportUSB, SupportedExtensions: []string{".nsp", ".nsz", ".xci", ".xcz"},
			WireNamespace: host.NamespaceFlatBasename, FilesystemAccess: host.FilesystemNone,
			CompatibleVersions: []string{}, VerifiedVersions: []string{}, KnownIncompatibleVersions: []string{},
			AdapterAvailable: true,
		},
		{
			ID: host.ProfileAwoo, DisplayName: "Awoo USB", ProtocolFamily: "Awoo USB",
			Transport: host.TransportUSB, SupportedExtensions: []string{".nsp", ".nsz", ".xci", ".xcz"},
			WireNamespace: host.NamespaceFlatBasename, FilesystemAccess: host.FilesystemNone,
			CompatibleVersions: []string{}, VerifiedVersions: []string{"1.6.2"}, KnownIncompatibleVersions: []string{},
			AdapterAvailable: true,
		},
		{
			ID: host.ProfileGoldleaf, DisplayName: "Goldleaf 0.10+", ProtocolFamily: "Goldleaf 0.10+",
			Transport: host.TransportUSB, SupportedExtensions: []string{".nsp"},
			WireNamespace: host.NamespaceVirtualCatalog, FilesystemAccess: host.FilesystemReadOnly,
			CompatibleVersions: []string{"0.10+"}, VerifiedVersions: []string{"1.2.0"}, KnownIncompatibleVersions: []string{},
			AdapterAvailable: true,
		},
	}
	if !reflect.DeepEqual(profiles, want) {
		t.Fatalf("Profiles() = %#v, want %#v", profiles, want)
	}
	profiles[0].SupportedExtensions[0] = ".changed"
	refetched, _ := host.ProfileByID(host.ProfileDBI)
	if refetched.SupportedExtensions[0] != ".nsp" {
		t.Fatal("Profiles() exposed mutable registry storage")
	}
	encoded, err := json.Marshal(host.Profiles())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(":null")) {
		t.Fatalf("typed profile serialization contains null collection: %s", encoded)
	}
}

func TestProfileValidationReturnsItemSpecificCapabilityErrors(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"base.nsp", "compressed.NSZ"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := files.BuildCatalog([]string{root}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	errors, err := host.ValidateCatalog(host.ProfileGoldleaf, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(errors) != 1 || errors[0].Name != "compressed.NSZ" || errors[0].Code != host.ValidationUnsupportedExtension {
		t.Fatalf("validation errors = %#v", errors)
	}
	if dbiErrors, err := host.ValidateCatalog(host.ProfileDBI, catalog); err != nil || len(dbiErrors) != 0 {
		t.Fatalf("DBI validation = %#v, %v", dbiErrors, err)
	}
}

func TestAwooValidationRejectsNamesThatBreakTheWireList(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows rejects these filenames before catalog discovery")
	}
	tests := []struct {
		label string
		name  string
	}{
		{label: "line delimiter", name: "line\nbreak.nsp"},
		{label: "backslash", name: `back\slash.nsp`},
		{label: "invalid UTF-8", name: string([]byte{0xff}) + ".nsp"},
	}
	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			if test.label == "invalid UTF-8" && runtime.GOOS == "darwin" {
				t.Skip("macOS rejects invalid UTF-8 before catalog discovery")
			}
			path := filepath.Join(t.TempDir(), test.name)
			if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
				t.Fatal(err)
			}
			catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
			if err != nil {
				t.Fatal(err)
			}
			validationErrors, err := host.ValidateCatalog(host.ProfileAwoo, catalog)
			if err != nil {
				t.Fatal(err)
			}
			if len(validationErrors) != 1 || validationErrors[0].Code != host.ValidationInvalidWireName {
				t.Fatalf("validation errors = %#v", validationErrors)
			}
		})
	}
}

func TestGoldleafValidationRejectsNamesThatEscapeTheFlatVirtualCatalog(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows rejects backslashes in filenames before catalog discovery")
	}
	path := filepath.Join(t.TempDir(), `back\slash.nsp`)
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	validationErrors, err := host.ValidateCatalog(host.ProfileGoldleaf, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(validationErrors) != 1 || validationErrors[0].Code != host.ValidationInvalidWireName {
		t.Fatalf("validation errors = %#v", validationErrors)
	}
}
