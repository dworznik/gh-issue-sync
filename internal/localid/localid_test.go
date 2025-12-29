package localid

import (
	"regexp"
	"testing"
)

func TestGenerate(t *testing.T) {
	id, err := Generate()
	if err != nil {
		t.Fatalf("failed to generate ID: %v", err)
	}

	// Should be 8 characters (4 bytes hex encoded)
	if len(id) != 8 {
		t.Fatalf("expected 8 characters, got %d: %q", len(id), id)
	}

	// Should be valid hex
	if matched, _ := regexp.MatchString(`^[0-9a-f]{8}$`, id); !matched {
		t.Fatalf("expected hex string, got %q", id)
	}
}

func TestGenerateUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id, err := Generate()
		if err != nil {
			t.Fatalf("failed to generate ID: %v", err)
		}
		if seen[id] {
			t.Fatalf("duplicate ID generated: %q", id)
		}
		seen[id] = true
	}
}
