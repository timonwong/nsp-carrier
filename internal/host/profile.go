package host

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/timonwong/nsp-carrier/internal/awoo"
	"github.com/timonwong/nsp-carrier/internal/dbi"
	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/goldleaf"
)

var ErrUnsupportedContent = errors.New("content is unsupported by installer profile")

type ProfileID string

const (
	ProfileDBI      ProfileID = "dbi"
	ProfileAwoo     ProfileID = "awoo"
	ProfileGoldleaf ProfileID = "goldleaf"
)

type TransportKind string

const TransportUSB TransportKind = "usb"

type WireNamespace string

const (
	NamespaceFlatBasename   WireNamespace = "flat-basename"
	NamespaceVirtualCatalog WireNamespace = "VIRT:/"
)

type FilesystemAccess string

const (
	FilesystemNone     FilesystemAccess = "none"
	FilesystemReadOnly FilesystemAccess = "read-only"
)

type Profile struct {
	ID                        ProfileID        `json:"id"`
	DisplayName               string           `json:"displayName"`
	ProtocolFamily            string           `json:"protocolFamily"`
	Transport                 TransportKind    `json:"transport"`
	SupportedExtensions       []string         `json:"supportedExtensions"`
	WireNamespace             WireNamespace    `json:"wireNamespace"`
	FilesystemAccess          FilesystemAccess `json:"filesystemAccess"`
	CompatibleVersions        []string         `json:"compatibleVersions"`
	VerifiedVersions          []string         `json:"verifiedVersions"`
	KnownIncompatibleVersions []string         `json:"knownIncompatibleVersions"`
	AdapterAvailable          bool             `json:"adapterAvailable"`
}

var profileRegistry = []Profile{
	{
		ID: ProfileDBI, DisplayName: "DBI", ProtocolFamily: "DBI0",
		Transport: TransportUSB, SupportedExtensions: []string{".nsp", ".nsz", ".xci", ".xcz"},
		WireNamespace: NamespaceFlatBasename, FilesystemAccess: FilesystemNone,
		CompatibleVersions: []string{}, AdapterAvailable: true,
	},
	{
		ID: ProfileAwoo, DisplayName: "Awoo USB", ProtocolFamily: "Awoo USB",
		Transport: TransportUSB, SupportedExtensions: []string{".nsp", ".nsz", ".xci", ".xcz"},
		WireNamespace: NamespaceFlatBasename, FilesystemAccess: FilesystemNone,
		CompatibleVersions: []string{}, VerifiedVersions: []string{"1.6.2"}, AdapterAvailable: true,
	},
	{
		ID: ProfileGoldleaf, DisplayName: "Goldleaf 0.10+", ProtocolFamily: "Goldleaf 0.10+",
		Transport: TransportUSB, SupportedExtensions: []string{".nsp"},
		WireNamespace: NamespaceVirtualCatalog, FilesystemAccess: FilesystemReadOnly,
		CompatibleVersions: []string{"0.10+"}, VerifiedVersions: []string{"1.2.0"}, AdapterAvailable: true,
	},
}

func Profiles() []Profile {
	profiles := make([]Profile, len(profileRegistry))
	for index, profile := range profileRegistry {
		profiles[index] = cloneProfile(profile)
	}
	return profiles
}

func ProfileByID(id ProfileID) (Profile, bool) {
	for _, profile := range profileRegistry {
		if profile.ID == id {
			return cloneProfile(profile), true
		}
	}
	return Profile{}, false
}

func AllSupportedExtensions() []string {
	seen := make(map[string]struct{})
	var extensions []string
	for _, profile := range profileRegistry {
		for _, extension := range profile.SupportedExtensions {
			if _, exists := seen[extension]; exists {
				continue
			}
			seen[extension] = struct{}{}
			extensions = append(extensions, extension)
		}
	}
	return extensions
}

func cloneProfile(profile Profile) Profile {
	profile.SupportedExtensions = append([]string{}, profile.SupportedExtensions...)
	profile.CompatibleVersions = append([]string{}, profile.CompatibleVersions...)
	profile.VerifiedVersions = append([]string{}, profile.VerifiedVersions...)
	profile.KnownIncompatibleVersions = append([]string{}, profile.KnownIncompatibleVersions...)
	return profile
}

type ValidationCode string

const (
	ValidationUnsupportedExtension ValidationCode = "unsupported-extension"
	ValidationDuplicateWireName    ValidationCode = "duplicate-wire-name"
	ValidationInvalidWireName      ValidationCode = "invalid-wire-name"
)

type ItemValidationError struct {
	SourceID string         `json:"sourceId"`
	Name     string         `json:"name"`
	Code     ValidationCode `json:"code"`
	Message  string         `json:"message"`
}

type CatalogValidationErrors []ItemValidationError

func (e CatalogValidationErrors) Error() string {
	if len(e) == 0 {
		return "catalog validation failed"
	}
	messages := make([]string, len(e))
	for index, validationError := range e {
		messages[index] = validationError.Message
	}
	return strings.Join(messages, "; ")
}

func (e CatalogValidationErrors) Is(target error) bool {
	for _, validationError := range e {
		if target == files.ErrDuplicateBasename && validationError.Code == ValidationDuplicateWireName {
			return true
		}
		if target == ErrUnsupportedContent && validationError.Code == ValidationUnsupportedExtension {
			return true
		}
	}
	return false
}

func ValidateCatalog(profileID ProfileID, catalog *files.Catalog) ([]ItemValidationError, error) {
	profile, ok := ProfileByID(profileID)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownProfile, profileID)
	}
	allowed := make(map[string]struct{}, len(profile.SupportedExtensions))
	for _, extension := range profile.SupportedExtensions {
		allowed[extension] = struct{}{}
	}
	nameCounts := make(map[string]int)
	for _, entry := range catalog.Entries() {
		nameCounts[entry.Name]++
	}
	var validationErrors []ItemValidationError
	for _, entry := range catalog.Entries() {
		if _, ok := allowed[strings.ToLower(filepath.Ext(entry.Name))]; !ok {
			validationErrors = append(validationErrors, ItemValidationError{
				SourceID: entry.ID, Name: entry.Name, Code: ValidationUnsupportedExtension,
				Message: fmt.Sprintf("%s: %s does not support %s files", entry.Name, profile.DisplayName, filepath.Ext(entry.Name)),
			})
		}
		if nameCounts[entry.Name] > 1 {
			validationErrors = append(validationErrors, ItemValidationError{
				SourceID: entry.ID, Name: entry.Name, Code: ValidationDuplicateWireName,
				Message: fmt.Sprintf("%s projects more than one selected file as %q", profile.DisplayName, entry.Name),
			})
		}
		if validateWireName(profileID, entry.Name) != nil {
			validationErrors = append(validationErrors, ItemValidationError{
				SourceID: entry.ID, Name: entry.Name, Code: ValidationInvalidWireName,
				Message: fmt.Sprintf("%q cannot be represented safely by %s", entry.Name, profile.DisplayName),
			})
		}
	}
	return validationErrors, nil
}

func validateWireName(profileID ProfileID, name string) error {
	switch profileID {
	case ProfileDBI:
		return dbi.ValidateWireName(name)
	case ProfileAwoo:
		return awoo.ValidateWireName(name)
	case ProfileGoldleaf:
		return goldleaf.ValidateWireName(name)
	default:
		if strings.ContainsAny(name, "\x00\r\n") {
			return errors.New("invalid wire name")
		}
		return nil
	}
}
