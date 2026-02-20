package config

import "testing"

func TestParseModelRefWithAt(t *testing.T) {
	modelKey, provider, ok := ParseModelRef("gpt-4@openai")
	if !ok {
		t.Fatal("expected ok=true for model@provider")
	}
	if modelKey != "gpt-4" {
		t.Errorf("expected modelKey gpt-4, got %s", modelKey)
	}
	if provider != "openai" {
		t.Errorf("expected provider openai, got %s", provider)
	}
}

func TestParseModelRefBareProvider(t *testing.T) {
	modelKey, provider, ok := ParseModelRef("ollama")
	if ok {
		t.Fatal("expected ok=false for bare provider")
	}
	if modelKey != "" {
		t.Errorf("expected empty modelKey, got %s", modelKey)
	}
	if provider != "ollama" {
		t.Errorf("expected provider ollama, got %s", provider)
	}
}

func TestParseModelRefEmptyModel(t *testing.T) {
	// "@provider" — empty model key should fall back to bare provider
	_, provider, ok := ParseModelRef("@openai")
	if ok {
		t.Fatal("expected ok=false for empty model key")
	}
	if provider != "@openai" {
		t.Errorf("expected provider @openai (raw ref), got %s", provider)
	}
}

func TestParseModelRefEmptyProvider(t *testing.T) {
	// "model@" — empty provider should fall back to bare ref
	_, provider, ok := ParseModelRef("gpt-4@")
	if ok {
		t.Fatal("expected ok=false for empty provider")
	}
	if provider != "gpt-4@" {
		t.Errorf("expected raw ref gpt-4@, got %s", provider)
	}
}

func TestModelNameExplicit(t *testing.T) {
	mc := ModelConfig{Model: "claude-3-haiku-20240307"}
	name := mc.ModelName("haiku")
	if name != "claude-3-haiku-20240307" {
		t.Errorf("expected explicit model name, got %s", name)
	}
}

func TestModelNameFallsBackToKey(t *testing.T) {
	mc := ModelConfig{}
	name := mc.ModelName("gpt-4o-mini")
	if name != "gpt-4o-mini" {
		t.Errorf("expected map key as model name, got %s", name)
	}
}
