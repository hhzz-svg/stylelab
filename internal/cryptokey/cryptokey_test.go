package cryptokey_test

import (
	"bytes"
	"testing"

	"stylelab/internal/cryptokey"
)

func TestSealOpenRoundtrip(t *testing.T) {
	master := bytes.Repeat([]byte{0x42}, 32)
	plain := "sk-test-secret-key"
	blob, err := cryptokey.Seal(master, plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if len(blob) == 0 {
		t.Fatal("empty blob")
	}
	if bytes.Contains(blob, []byte(plain)) {
		t.Fatal("blob contains plaintext")
	}
	got, err := cryptokey.Open(master, blob)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got != plain {
		t.Fatalf("got %q want %q", got, plain)
	}
}

func TestOpenRejectsTamperedBlob(t *testing.T) {
	master := bytes.Repeat([]byte{0x42}, 32)
	blob, err := cryptokey.Seal(master, "secret")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	blob[len(blob)-1] ^= 0x01
	if _, err := cryptokey.Open(master, blob); err == nil {
		t.Fatal("expected error for tampered blob")
	}
}

func TestOpenRejectsWrongMasterKey(t *testing.T) {
	master := bytes.Repeat([]byte{0x42}, 32)
	other := bytes.Repeat([]byte{0x43}, 32)
	blob, err := cryptokey.Seal(master, "secret")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := cryptokey.Open(other, blob); err == nil {
		t.Fatal("expected error for wrong master key")
	}
}
