package state

import (
	"testing"
)

func TestStore_GetSetDelete(t *testing.T) {
	store := NewStore()

	// Test Set and Get
	store.Set("key1", "value1")
	store.Set("key2", 42)

	val1 := store.Get("key1")
	if val1 != "value1" {
		t.Errorf("expected 'value1', got %v", val1)
	}

	val2 := store.Get("key2")
	if val2 != 42 {
		t.Errorf("expected 42, got %v", val2)
	}

	// Test Get non-existent key
	result := store.Get("nonexistent")
	if result != nil {
		t.Errorf("expected nil for nonexistent key, got %v", result)
	}

	// Test Delete
	store.Delete("key1")
	result = store.Get("key1")
	if result != nil {
		t.Errorf("expected nil after delete, got %v", result)
	}

	// key2 should still exist
	result = store.Get("key2")
	if result != 42 {
		t.Errorf("expected key2 to still exist with value 42, got %v", result)
	}
}

func TestStore_Snapshot(t *testing.T) {
	store := NewStore()

	store.Set("a", 1)
	store.Set("b", "test")

	snapshot := store.Snapshot()

	// Modify original
	store.Set("a", 999)
	store.Delete("b")

	// Snapshot should be unchanged
	if valA := snapshot["a"]; valA != 1 {
		t.Errorf("snapshot should preserve original value, got %v", valA)
	}

	if valB := snapshot["b"]; valB != "test" {
		t.Errorf("snapshot should preserve original value, got %v", valB)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := NewStore()

	// This is a basic test - in real scenarios, proper concurrent testing would be needed
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 100; i++ {
			store.Set("key", i)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			store.Get("key")
		}
		done <- true
	}()

	<-done
	<-done

	// If we get here without panicking, basic concurrency works
}
