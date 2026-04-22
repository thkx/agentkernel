package event

import (
	"os"
	"testing"
	"time"

	"github.com/thkx/agentkernel/types"
)

func TestEventBusSubscription(t *testing.T) {
	bus := NewSourcingBus(NewInMemoryEventStore())

	sub1 := bus.Subscribe()
	sub2 := bus.Subscribe()

	event := types.Event{NodeID: "n1", TraceID: "trace1"}
	bus.Publish(event)

	select {
	case e := <-sub1:
		if e.NodeID != "n1" {
			t.Fatalf("Sub1 expected n1, got %q", e.NodeID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Sub1 timeout")
	}

	select {
	case e := <-sub2:
		if e.NodeID != "n1" {
			t.Fatalf("Sub2 expected n1, got %q", e.NodeID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Sub2 timeout")
	}
}

func TestEventReplay(t *testing.T) {
	store := NewInMemoryEventStore()

	for i := 0; i < 3; i++ {
		store.Append(types.Event{NodeID: "n1"})
	}

	count := 0
	store.Replay(func(ev types.Event) {
		count++
	})

	if count != 3 {
		t.Fatalf("Expected 3 events, got %d", count)
	}
}

func TestFileEventStore(t *testing.T) {
	tmpdir := t.TempDir()

	store, err := NewFileEventStore(tmpdir)
	if err != nil {
		t.Fatalf("Failed to create file store: %v", err)
	}

	// Append events
	store.Append(types.Event{NodeID: "n1", Result: types.Result{Output: "result1"}})
	store.Append(types.Event{NodeID: "n2", Result: types.Result{Output: "result2"}})

	// Verify events
	events := store.Events()
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	// Verify snapshot
	snap := store.Snapshot()
	if snap["n1"] != "result1" {
		t.Fatalf("Expected snapshot n1=result1, got %v", snap["n1"])
	}
}

func TestFileEventStoreRecovery(t *testing.T) {
	tmpdir := t.TempDir()

	// Create and populate store
	store1, _ := NewFileEventStore(tmpdir)
	store1.Append(types.Event{NodeID: "n1", Result: types.Result{Output: "data1"}})
	store1.Append(types.Event{NodeID: "n2", Result: types.Result{Output: "data2"}})

	// Create new store from same directory
	store2, _ := NewFileEventStore(tmpdir)

	// Verify events were recovered
	events := store2.Events()
	if len(events) != 2 {
		t.Fatalf("Expected 2 recovered events, got %d", len(events))
	}
}

func TestEventStoreWithArchive(t *testing.T) {
	tmpdir := t.TempDir()

	archive, err := NewEventStoreWithArchive(tmpdir)
	if err != nil {
		t.Fatalf("Failed to create archive: %v", err)
	}

	// Append through archive
	archive.Append(types.Event{NodeID: "n1"})
	archive.Append(types.Event{NodeID: "n2"})

	// Verify hot store
	events := archive.Events()
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	// Verify cold store wrote files
	if _, err := os.Stat(tmpdir + "/events.jsonl"); err != nil {
		t.Fatalf("Events file not persisted: %v", err)
	}
}
