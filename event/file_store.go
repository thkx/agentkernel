package event

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/thkx/agentkernel/types"
)

var _ EventStore = (*FileEventStore)(nil)

// FileEventStore persists events to local filesystem
type FileEventStore struct {
	mu               sync.RWMutex
	dir              string
	eventFile        string
	snapshotFile     string
	timelineFile     string
	events           []types.Event
	snapshot         map[string]any
	timeline         []types.ExecutionTimelineEntry
	snapshotInterval int
	eventCount       int
}

// NewFileEventStore creates a new file-based event store
func NewFileEventStore(dir string) (*FileEventStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create event store directory: %w", err)
	}

	fs := &FileEventStore{
		dir:              dir,
		eventFile:        filepath.Join(dir, "events.jsonl"),
		snapshotFile:     filepath.Join(dir, "snapshot.json"),
		timelineFile:     filepath.Join(dir, "timeline.jsonl"),
		snapshot:         make(map[string]any),
		events:           make([]types.Event, 0),
		timeline:         make([]types.ExecutionTimelineEntry, 0),
		snapshotInterval: 100, // Take snapshot every 100 events
	}

	// Load existing events and snapshot
	fs.loadFromDisk()
	return fs, nil
}

// Append persists an event
func (fs *FileEventStore) Append(event types.Event) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Append to memory
	fs.events = append(fs.events, event)
	// Update snapshot
	if result, ok := event.Result.(types.Result); ok {
		fs.snapshot[event.NodeID] = result.Output
	}

	// Write event to file
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(fs.eventFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}

	fs.eventCount++
	// Periodically save snapshot
	if fs.eventCount%fs.snapshotInterval == 0 {
		if err := fs.saveSnapshot(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to save snapshot: %v\n", err)
		}
	}

	return nil
}

// Events returns all events
func (fs *FileEventStore) Events() []types.Event {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	events := make([]types.Event, len(fs.events))
	copy(events, fs.events)
	return events
}

// Replay calls handler for each event
func (fs *FileEventStore) Replay(handler func(types.Event)) error {
	fs.mu.RLock()
	events := make([]types.Event, len(fs.events))
	copy(events, fs.events)
	fs.mu.RUnlock()

	for _, e := range events {
		handler(e)
	}
	return nil
}

// Snapshot returns current state
func (fs *FileEventStore) Snapshot() map[string]any {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	snap := make(map[string]any)
	for k, v := range fs.snapshot {
		snap[k] = v
	}
	return snap
}

// AppendTimeline records execution timeline
func (fs *FileEventStore) AppendTimeline(entry types.ExecutionTimelineEntry) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.timeline = append(fs.timeline, entry)

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(fs.timelineFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return err
}

// Timeline returns all timeline entries
func (fs *FileEventStore) Timeline() []types.ExecutionTimelineEntry {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	entries := make([]types.ExecutionTimelineEntry, len(fs.timeline))
	copy(entries, fs.timeline)
	return entries
}

// loadFromDisk initializes store from persisted files
func (fs *FileEventStore) loadFromDisk() {
	// Load events
	fs.loadEvents()
	// Load snapshot
	fs.loadSnapshot()
	// Load timeline
	fs.loadTimeline()
}

// loadEvents reads events from jsonl file
func (fs *FileEventStore) loadEvents() {
	f, err := os.Open(fs.eventFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("error reading events file: %v\n", err)
		}
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var event types.Event
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := json.Unmarshal(line, &event); err != nil {
			fmt.Fprintf(os.Stderr, "failed to unmarshal timeline event: %v\n", err)
			continue
		}
		fs.events = append(fs.events, event)
	}
}

// loadSnapshot reads snapshot from json file
func (fs *FileEventStore) loadSnapshot() {
	data, err := os.ReadFile(fs.snapshotFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("error reading snapshot file: %v\n", err)
		}
		return
	}

	json.Unmarshal(data, &fs.snapshot)
	// Note: ignoring unmarshal errors for now, but this could lead to corrupted state
}

// loadTimeline reads timeline from jsonl file
func (fs *FileEventStore) loadTimeline() {
	f, err := os.Open(fs.timelineFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("error reading timeline file: %v\n", err)
		}
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var entry types.ExecutionTimelineEntry
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := json.Unmarshal(line, &entry); err == nil {
			fs.timeline = append(fs.timeline, entry)
		}
	}
}

// saveSnapshot persists current snapshot
func (fs *FileEventStore) saveSnapshot() error {
	data, err := json.MarshalIndent(fs.snapshot, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fs.snapshotFile, data, 0644)
}

// EventStoreWithArchive wraps two stores: hot (memory) + cold (file)
type EventStoreWithArchive struct {
	hot  *InMemoryEventStore
	cold *FileEventStore
	mu   sync.Mutex
}

// NewEventStoreWithArchive creates a store with memory cache + file persistence
func NewEventStoreWithArchive(dir string) (*EventStoreWithArchive, error) {
	cold, err := NewFileEventStore(dir)
	if err != nil {
		return nil, err
	}

	hot := NewInMemoryEventStore()
	// Replay cold events into hot store
	cold.Replay(func(e types.Event) {
		hot.Append(e)
	})

	return &EventStoreWithArchive{
		hot:  hot,
		cold: cold,
	}, nil
}

// Append writes to both hot and cold stores
func (esa *EventStoreWithArchive) Append(event types.Event) error {
	esa.mu.Lock()
	defer esa.mu.Unlock()

	if err := esa.cold.Append(event); err != nil {
		return err
	}
	return esa.hot.Append(event)
}

// Events returns all events from hot store
func (esa *EventStoreWithArchive) Events() []types.Event {
	return esa.hot.Events()
}

// Replay replays from hot store
func (esa *EventStoreWithArchive) Replay(handler func(types.Event)) error {
	return esa.hot.Replay(handler)
}

// Snapshot returns snapshot
func (esa *EventStoreWithArchive) Snapshot() map[string]any {
	return esa.hot.Snapshot()
}

// AppendTimeline records timeline
func (esa *EventStoreWithArchive) AppendTimeline(entry types.ExecutionTimelineEntry) error {
	esa.mu.Lock()
	defer esa.mu.Unlock()

	if err := esa.cold.AppendTimeline(entry); err != nil {
		return err
	}
	return esa.hot.AppendTimeline(entry)
}

// Timeline returns timeline
func (esa *EventStoreWithArchive) Timeline() []types.ExecutionTimelineEntry {
	return esa.hot.Timeline()
}
