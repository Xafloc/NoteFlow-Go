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
