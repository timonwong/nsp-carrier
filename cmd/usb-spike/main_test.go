package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIProfileContract(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "usb-spike")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build usb-spike: %v\n%s", err, output)
	}

	help := exec.Command(binary, "--help")
	helpOutput, _ := help.CombinedOutput()
	if !strings.Contains(string(helpOutput), `default "dbi"`) {
		t.Fatalf("--help does not expose DBI default:\n%s", helpOutput)
	}

	unknown := exec.Command(binary, "--profile", "automatic", "game.nsp")
	unknownOutput, err := unknown.CombinedOutput()
	if err == nil || !strings.Contains(string(unknownOutput), `unknown installer profile: "automatic"`) {
		t.Fatalf("unknown profile result = %v\n%s", err, unknownOutput)
	}

	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	goldleaf := exec.Command(binary, "--timeout", "1ns", "--reset-on-connect=false", "--profile", "goldleaf", path)
	goldleafOutput, err := goldleaf.CombinedOutput()
	if err == nil || strings.Contains(string(goldleafOutput), "installer profile adapter is not implemented") ||
		!strings.Contains(string(goldleafOutput), "WaitingForDevice") ||
		!strings.Contains(string(goldleafOutput), "context deadline exceeded") {
		t.Fatalf("Goldleaf routing result = %v\n%s", err, goldleafOutput)
	}

	unsupported := []string{
		filepath.Join(t.TempDir(), "compressed.nsz"),
		filepath.Join(t.TempDir(), "cartridge.xci"),
	}
	for _, path := range unsupported {
		if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	validation := exec.Command(binary, append([]string{"--profile", "goldleaf"}, unsupported...)...)
	validationOutput, err := validation.CombinedOutput()
	if err == nil || !strings.Contains(string(validationOutput), "compressed.nsz") || !strings.Contains(string(validationOutput), "cartridge.xci") {
		t.Fatalf("validation result = %v\n%s", err, validationOutput)
	}

	structured := exec.Command(binary, append([]string{"--json", "--profile", "goldleaf"}, unsupported...)...)
	structuredOutput, err := structured.CombinedOutput()
	if err == nil {
		t.Fatalf("structured validation unexpectedly succeeded:\n%s", structuredOutput)
	}
	lines := strings.Split(strings.TrimSpace(string(structuredOutput)), "\n")
	if len(lines) != 2 {
		t.Fatalf("structured validation lines = %d, want 2:\n%s", len(lines), structuredOutput)
	}
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil || record["event"] != "catalog_validation" {
			t.Fatalf("structured record = %q, %v", line, err)
		}
	}
}
