package subscribers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSubscribeAndList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subscribers.json")

	store, err := NewStore(context.Background(), path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	ctx := context.Background()

	if err := store.Subscribe(ctx, 100); err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	if err := store.Subscribe(ctx, 100); err != nil {
		t.Fatalf("Subscribe duplicate error: %v", err)
	}

	if err := store.Subscribe(ctx, 200); err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	ids, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}

	expected := []int64{100, 200}
	if len(ids) != len(expected) {
		t.Fatalf("unexpected ids length: %v", ids)
	}

	for i, id := range expected {
		if ids[i] != id {
			t.Fatalf("expected ids[%d]=%d, got %d", i, id, ids[i])
		}
	}

	// Ensure data persisted.
	store2, err := NewStore(context.Background(), path)
	if err != nil {
		t.Fatalf("NewStore second error: %v", err)
	}

	ids2, err := store2.List(ctx)
	if err != nil {
		t.Fatalf("List second error: %v", err)
	}

	if len(ids2) != len(expected) {
		t.Fatalf("unexpected ids length after reload: %v", ids2)
	}
}

func TestStoreUnsubscribe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subscribers.json")

	store, err := NewStore(context.Background(), path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	ctx := context.Background()

	if err := store.Subscribe(ctx, 777); err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	if err := store.Unsubscribe(ctx, 777); err != nil {
		t.Fatalf("Unsubscribe error: %v", err)
	}

	ids, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}

	if len(ids) != 0 {
		t.Fatalf("expected empty list, got %v", ids)
	}
}

func TestStoreHandlesCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subscribers.json")

	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}

	store, err := NewStore(context.Background(), path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	ids, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}

	if len(ids) != 0 {
		t.Fatalf("expected empty list, got %v", ids)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var decoded []int64
	if err := json.Unmarshal(contents, &decoded); err != nil {
		t.Fatalf("expected valid json after recovery: %v", err)
	}

	if len(decoded) != 0 {
		t.Fatalf("expected empty list after recovery, got %v", decoded)
	}
}

func TestStoreRespectsContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subscribers.json")

	store, err := NewStore(context.Background(), path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := store.Subscribe(ctx, 1); err == nil {
		t.Fatal("expected context error on Subscribe")
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if err := store.Unsubscribe(ctx2, 1); err == nil {
		t.Fatal("expected context error on Unsubscribe")
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), time.Nanosecond)
	cancel3()
	if _, err := store.List(ctx3); err == nil {
		t.Fatal("expected context error on List")
	}
}
