package devtools

import (
	"sync"
	"testing"
)

func TestRequestRingKeepsNewestAtCapacity(t *testing.T) {
	ring := newRequestRing(200)
	for status := 1; status <= 250; status++ {
		ring.Add(RequestEntry{Status: status})
	}
	snapshot := ring.Snapshot()
	if len(snapshot) != 200 {
		t.Fatalf("snapshot length = %d, want 200", len(snapshot))
	}
	if snapshot[0].Status != 250 || snapshot[0].ID != 250 {
		t.Fatalf("newest entry first, got status=%d id=%d", snapshot[0].Status, snapshot[0].ID)
	}
	if snapshot[199].Status != 51 || snapshot[199].ID != 51 {
		t.Fatalf("oldest retained entry wrong, got status=%d id=%d", snapshot[199].Status, snapshot[199].ID)
	}
	for index := 1; index < len(snapshot); index++ {
		if snapshot[index].ID != snapshot[index-1].ID-1 {
			t.Fatalf("snapshot not newest-first at %d: %d then %d", index, snapshot[index-1].ID, snapshot[index].ID)
		}
	}
}

func TestRequestRingConcurrentAddAndSnapshot(t *testing.T) {
	ring := newRequestRing(64)
	var group sync.WaitGroup
	for range 8 {
		group.Add(2)
		go func() {
			defer group.Done()
			for i := 0; i < 200; i++ {
				ring.Add(RequestEntry{Method: "GET", Path: "/x"})
			}
		}()
		go func() {
			defer group.Done()
			for i := 0; i < 200; i++ {
				if snapshot := ring.Snapshot(); len(snapshot) > 64 {
					t.Errorf("snapshot exceeds capacity: %d", len(snapshot))
					return
				}
			}
		}()
	}
	group.Wait()
	if len(ring.Snapshot()) != 64 {
		t.Fatalf("ring not full after writes: %d", len(ring.Snapshot()))
	}
}
