package llm

import "testing"

func TestBusyTrackerOllamaLimit(t *testing.T) {
	bt := NewBusyTracker(map[string]string{
		"ollama": "ollama",
	})

	// First acquire should succeed
	if !bt.Acquire("ollama", "llama3") {
		t.Fatal("first acquire should succeed")
	}

	// Second acquire same provider should fail (limit=1 for ollama)
	if bt.Acquire("ollama", "llama3") {
		t.Fatal("second acquire should fail for ollama (limit=1)")
	}

	// Should be busy
	if !bt.IsBusy("ollama", "llama3") {
		t.Error("expected IsBusy=true")
	}

	// Release
	bt.Release("ollama", "llama3")

	// Should no longer be busy
	if bt.IsBusy("ollama", "llama3") {
		t.Error("expected IsBusy=false after release")
	}

	// Can acquire again
	if !bt.Acquire("ollama", "llama3") {
		t.Fatal("acquire after release should succeed")
	}
}

func TestBusyTrackerCloudUnlimited(t *testing.T) {
	bt := NewBusyTracker(map[string]string{
		"openai": "openai",
	})

	// Cloud providers should have no limit
	for i := 0; i < 100; i++ {
		if !bt.Acquire("openai", "gpt-4") {
			t.Fatalf("acquire %d should succeed for cloud provider", i)
		}
	}

	// Should never be busy
	if bt.IsBusy("openai", "gpt-4") {
		t.Error("cloud provider should never report busy")
	}
}

func TestBusyTrackerDifferentModels(t *testing.T) {
	bt := NewBusyTracker(map[string]string{
		"ollama": "ollama",
	})

	// Acquire model A
	if !bt.Acquire("ollama", "modelA") {
		t.Fatal("acquire modelA should succeed")
	}

	// Acquire model B — should also succeed (different model key)
	if !bt.Acquire("ollama", "modelB") {
		t.Fatal("acquire modelB should succeed (different key)")
	}

	// But modelA is still busy
	if !bt.IsBusy("ollama", "modelA") {
		t.Error("modelA should be busy")
	}
}

func TestBusyTrackerReleaseDoesNotGoNegative(t *testing.T) {
	bt := NewBusyTracker(map[string]string{
		"ollama": "ollama",
	})

	// Release without acquire — should not panic or go negative
	bt.Release("ollama", "llama3")

	// Should still be able to acquire
	if !bt.Acquire("ollama", "llama3") {
		t.Fatal("acquire should succeed after spurious release")
	}
}
