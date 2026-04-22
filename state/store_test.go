package state

import "testing"

func TestStoreSnapshotClonesNestedContainers(t *testing.T) {
	store := NewStoreFromMap(map[string]any{
		"map": map[string]any{
			"nested": "value",
		},
		"slice": []string{"a", "b"},
	})

	snapshot := store.Snapshot()
	snapshot["map"].(map[string]any)["nested"] = "changed"
	snapshot["slice"].([]string)[0] = "changed"

	if store.Get("map").(map[string]any)["nested"] != "value" {
		t.Fatal("expected nested map mutation to stay isolated from store")
	}
	if store.Get("slice").([]string)[0] != "a" {
		t.Fatal("expected nested slice mutation to stay isolated from store")
	}
}

func TestStoreGetReturnsClonedContainers(t *testing.T) {
	store := NewStore()
	store.Set("payload", map[string]any{
		"items": []int{1, 2, 3},
	})

	value := store.Get("payload").(map[string]any)
	value["items"].([]int)[0] = 99

	current := store.Get("payload").(map[string]any)
	if current["items"].([]int)[0] != 1 {
		t.Fatal("expected Get to return a cloned payload")
	}
}

func TestStoreDeleteRemovesKey(t *testing.T) {
	store := NewStoreFromMap(map[string]any{
		"present": true,
	})

	store.Delete("present")

	if value := store.Get("present"); value != nil {
		t.Fatalf("expected deleted key to be absent, got %v", value)
	}
}
