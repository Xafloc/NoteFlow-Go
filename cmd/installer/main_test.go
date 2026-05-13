package main

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"
)

// PowerShell's -EncodedCommand expects base64-encoded UTF-16 little-endian.
// This test pins that contract so a future "let me simplify the encoding"
// refactor can't silently regress the installer back into the silent-fail
// state that v1.3.3 shipped with.

func TestEncodeUTF16LEBase64_RoundTrip(t *testing.T) {
	original := `Write-Host "hello, world"`
	got := encodeUTF16LEBase64(original)

	// Decode base64 → bytes → UTF-16LE → string and confirm we recover input.
	raw, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("output is not valid base64: %v", err)
	}
	if len(raw)%2 != 0 {
		t.Fatalf("UTF-16LE byte length must be even, got %d", len(raw))
	}
	codes := make([]uint16, len(raw)/2)
	for i := range codes {
		codes[i] = uint16(raw[i*2]) | uint16(raw[i*2+1])<<8
	}
	recovered := string(utf16.Decode(codes))
	if recovered != original {
		t.Errorf("round-trip mismatch:\n got: %q\nwant: %q", recovered, original)
	}
}

func TestEncodeUTF16LEBase64_HandlesPathsWithBackslashes(t *testing.T) {
	// Windows paths historically tripped the previous PowerShell hand-off
	// because cmd.exe argv-quoting interacted badly with backslashes.
	// EncodedCommand sidesteps cmd parsing entirely, so this must work.
	original := `if ($path -notlike "*C:\Users\greerd\bin*") { Write-Host yes }`
	got := encodeUTF16LEBase64(original)

	raw, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	codes := make([]uint16, len(raw)/2)
	for i := range codes {
		codes[i] = uint16(raw[i*2]) | uint16(raw[i*2+1])<<8
	}
	recovered := string(utf16.Decode(codes))
	if recovered != original {
		t.Errorf("backslash-heavy input did not round-trip:\n got: %q\nwant: %q", recovered, original)
	}
	// Sanity: backslashes survive unescaped — that's why we use this scheme.
	if !strings.Contains(recovered, `C:\Users\greerd\bin`) {
		t.Errorf("backslash path was mutated")
	}
}

func TestEncodeUTF16LEBase64_EmptyInput(t *testing.T) {
	if got := encodeUTF16LEBase64(""); got != "" {
		t.Errorf("encoding empty string should yield empty base64, got %q", got)
	}
}

// Pins the two PowerShell parser hazards that have bitten previous releases:
// v1.3.3 silently failed; v1.3.7 loudly failed with "InvalidVariableReference
// WithDrive" because $path followed by C:\... was parsed as a drive-qualified
// variable. Both fixes (env-var-passed dir, ${path} delimiter) need to stay.
func TestPathUpdateScript_ParserHazards(t *testing.T) {
	// Hazard 1: $path followed directly by alphanumeric or a colon must be
	// avoided. The script must use ${path} to delimit the variable name.
	if strings.Contains(pathUpdateScript, "$path$") ||
		matchesBareVarFollowedByAlnum(pathUpdateScript, "$path") {
		t.Errorf("$path appears without ${...} delimiter — will misparse when followed by drive letter:\n%s", pathUpdateScript)
	}
	if !strings.Contains(pathUpdateScript, "${path}") {
		t.Errorf("expected ${path} to delimit the variable name explicitly:\n%s", pathUpdateScript)
	}

	// Hazard 2: the script must not interpolate the install dir directly
	// (we pass it via $env:NF_INSTALL_DIR instead, so the script can be
	// static and any path string lands as data, not code).
	if !strings.Contains(pathUpdateScript, "$env:NF_INSTALL_DIR") {
		t.Errorf("script must read install dir from $env:NF_INSTALL_DIR, not from interpolation:\n%s", pathUpdateScript)
	}

	// Defense in depth: -like with wildcards in install dirs (e.g. brackets)
	// could match too broadly. Use .Contains() for literal matching.
	if strings.Contains(pathUpdateScript, "-like") || strings.Contains(pathUpdateScript, "-notlike") {
		t.Errorf("script uses -like/-notlike — switch to .Contains() so install-dir characters aren't wildcards:\n%s", pathUpdateScript)
	}
}

// matchesBareVarFollowedByAlnum looks for occurrences of varName immediately
// followed by an alphanumeric character or a colon — both of which extend
// PowerShell's variable-name parser past where we want it to stop.
func matchesBareVarFollowedByAlnum(s, varName string) bool {
	i := 0
	for {
		idx := strings.Index(s[i:], varName)
		if idx == -1 {
			return false
		}
		end := i + idx + len(varName)
		if end < len(s) {
			c := s[end]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == ':' {
				return true
			}
		}
		i = end
	}
}
