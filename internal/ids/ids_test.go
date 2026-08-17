package ids_test

import (
	"strings"
	"testing"

	"stylelab/internal/ids"
)

func TestNewHasPrefixAndHexLength(t *testing.T) {
	id := ids.New("usr_")
	if !strings.HasPrefix(id, "usr_") {
		t.Fatalf("prefix: %s", id)
	}
	if !ids.Valid(id, "usr_") {
		t.Fatalf("expected valid %s", id)
	}
	a, b := ids.New("usr_"), ids.New("usr_")
	if a == b {
		t.Fatal("expected unique ids")
	}
}

func TestValidRejectsBad(t *testing.T) {
	if ids.Valid("usr_zz", "usr_") {
		t.Fatal("short/non-hex should be invalid")
	}
	if ids.Valid("prj_0123456789abcdef", "usr_") {
		t.Fatal("wrong prefix should be invalid")
	}
}
