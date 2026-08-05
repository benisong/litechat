package service

import "testing"

func TestCharacterCardParserRegistryResolvesJSONParser(t *testing.T) {
	registry := NewCharacterCardParserRegistry()
	parser, err := registry.Resolve("json-character-card", "1.0")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if parser.Format() != "json-character-card" {
		t.Fatalf("unexpected parser format: %s", parser.Format())
	}
}

func TestCharacterCardParserRegistryResolvesLegacyAdapter(t *testing.T) {
	parser, err := NewCharacterCardParserRegistry().Resolve("legacy-character-card", "legacy")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if parser.Format() != "legacy-character-card" {
		t.Fatalf("unexpected parser format: %s", parser.Format())
	}
}

func TestCharacterCardParserRegistryRejectsUnknownFormat(t *testing.T) {
	_, err := NewCharacterCardParserRegistry().Resolve("legacy", "")
	if err == nil {
		t.Fatal("expected unknown parser format to fail")
	}
}
