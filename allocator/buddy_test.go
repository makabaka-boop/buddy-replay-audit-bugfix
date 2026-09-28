package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// refModel is an independent byte-occupancy reference: it knows nothing
// about buddy blocks, only which bytes are occupied.
type refModel struct {
	occ  []bool
	live map[string]Block
	req  map[string]int
}

func newRefModel(capacity int) *refModel {
	return &refModel{occ: make([]bool, capacity), live: map[string]Block{}, req: map[string]int{}}
}

func (m *refModel) occupy(b Block) {
	for a := b.Addr; a < b.Addr+b.Size; a++ {
		m.occ[a] = true
	}
}

func (m *refModel) release(b Block) {
	for a := b.Addr; a < b.Addr+b.Size; a++ {
		m.occ[a] = false
	}
}

func (m *refModel) occupied() int {
	n := 0
	for _, o := range m.occ {
		if o {
			n++
		}
	}
	return n
}

// canonical decomposes the free space into maximal aligned power-of-two
// blocks — the unique shape the buddy free lists must have.
func (m *refModel) canonical(k int) (counts []int, maxBlock int) {
	counts = make([]int, k+1)
	for s := 0; s < len(m.occ); {
		if m.occ[s] {
			s++
			continue
		}
		e := s
		for e < len(m.occ) && !m.occ[e] {
			e++
		}
		for start, end := s, e; start < end; {
			size := 1
			if start == 0 {
				for size*2 <= end-start {
					size *= 2
				}
			} else {
				size = start & -start // alignment budget
				for size > end-start {
					size /= 2
				}
			}
			counts[orderFor(size)]++
			if size > maxBlock {
				maxBlock = size
			}
			start += size
		}
		s = e
	}
	return counts, maxBlock
}

// expectedPlacement derives the block the allocator must choose from the
// pre-event snapshot: smallest order that fits, lowest address within it.
func expectedPlacement(prev snapshot, need int) (Block, bool) {
	for o := need; o < len(prev.FreeCounts); o++ {
		if prev.FreeCounts[o] == 0 {
			continue
		}
		lowest := -1
		for _, b := range prev.FreeBlocks {
			if b.Order == o && (lowest < 0 || b.Addr < lowest) {
				lowest = b.Addr
			}
		}
		return Block{Addr: lowest, Size: 1 << need, Order: need}, true
	}
	return Block{}, false
}

func checkSnapshot(t *testing.T, model *refModel, k int, snap snapshot, where string) {
	t.Helper()
	capacity := 1 << k

	// Conservation: free bytes + occupied bytes == capacity.
	sumFree := 0
	for o, c := range snap.FreeCounts {
		sumFree += c << o
	}
	if sumFree != snap.TotalFree {
		t.Fatalf("%s: freeCounts imply %d free bytes, totalFree says %d", where, sumFree, snap.TotalFree)
	}
	if got := capacity - model.occupied(); got != snap.TotalFree {
		t.Fatalf("%s: conservation broken: capacity %d - occupied %d = %d, totalFree %d", where, capacity, model.occupied(), got, snap.TotalFree)
	}

	// Free lists must equal the canonical decomposition of free space.
	wantCounts, wantMax := model.canonical(k)
	if !reflect.DeepEqual(wantCounts, snap.FreeCounts) {
		t.Fatalf("%s: freeCounts %v, canonical decomposition says %v", where, snap.FreeCounts, wantCounts)
	}
	if wantMax != snap.MaxFreeBlock {
		t.Fatalf("%s: maxFreeBlock %d, canonical says %d", where, snap.MaxFreeBlock, wantMax)
	}

	// Listed free blocks: aligned, disjoint, and actually free in the model.
	seen := make([]bool, capacity)
	listed := 0
	for _, b := range snap.FreeBlocks {
		if b.Size != 1<<b.Order || b.Addr%b.Size != 0 || b.Addr < 0 || b.Addr+b.Size > capacity {
			t.Fatalf("%s: malformed free block %+v", where, b)
		}
		listed += b.Size
		for a := b.Addr; a < b.Addr+b.Size; a++ {
			if model.occ[a] {
				t.Fatalf("%s: free block %+v covers occupied byte %d", where, b, a)
			}
			if seen[a] {
				t.Fatalf("%s: free blocks overlap at byte %d", where, a)
			}
			seen[a] = true
		}
	}
	if listed != snap.TotalFree {
		t.Fatalf("%s: freeBlocks list holds %d bytes, totalFree %d", where, listed, snap.TotalFree)
	}

	// Fragmentation metrics.
	if want := snap.TotalFree - snap.MaxFreeBlock; snap.ExternalFrag != want {
		t.Fatalf("%s: externalFrag %d, want %d (totalFree %d - maxFreeBlock %d)", where, snap.ExternalFrag, want, snap.TotalFree, snap.MaxFreeBlock)
	}
	wantInternal := 0
	for id, b := range model.live {
		wantInternal += b.Size - model.req[id]
	}
	if snap.InternalFrag != wantInternal {
		t.Fatalf("%s: internalFrag %d, want %d", where, snap.InternalFrag, wantInternal)
	}
}

func TestReplayMatchesReferenceModel(t *testing.T) {
	const k = 8
	rng := rand.New(rand.NewSource(1))

	req := replayRequest{CapacityOrder: k}
	var liveIDs []string
	allocN := 0
	for i := 0; i < 300; i++ {
		if len(liveIDs) == 0 || rng.Intn(100) < 55 {
			allocN++
			req.Events = append(req.Events, Event{
				ID:      fmt.Sprintf("e%d", i),
				Type:    "allocate",
				AllocID: fmt.Sprintf("a%d", allocN),
				Size:    1 + rng.Intn(1<<k),
			})
		} else {
			j := rng.Intn(len(liveIDs))
			req.Events = append(req.Events, Event{ID: fmt.Sprintf("e%d", i), Type: "free", AllocID: liveIDs[j]})
			liveIDs = append(liveIDs[:j], liveIDs[j+1:]...)
		}
	}

	res, aerr := replay(req)
	if aerr != nil {
		t.Fatalf("replay rejected valid batch: %v", aerr)
	}

	model := newRefModel(1 << k)
	prev := res.Initial
	checkSnapshot(t, model, k, prev, "initial")

	for _, ev := range res.Events {
		where := fmt.Sprintf("event %s (%s %s)", ev.ID, ev.Type, ev.AllocID)
		switch ev.Type {
		case "allocate":
			need := orderFor(ev.Requested)
			want, fit := expectedPlacement(prev, need)
			if ev.Status == "oom" {
				if fit {
					t.Fatalf("%s: reported OOM but %v could place order %d", where, prev.FreeCounts, need)
				}
				if !reflect.DeepEqual(prev, ev.snapshot) {
					t.Fatalf("%s: OOM must leave state unchanged", where)
				}
			} else {
				if !fit {
					t.Fatalf("%s: placed but no block could fit order %d", where, need)
				}
				if *ev.Block != want {
					t.Fatalf("%s: placed %+v, policy (smallest order, lowest address) wants %+v", where, *ev.Block, want)
				}
				for a := ev.Block.Addr; a < ev.Block.Addr+ev.Block.Size; a++ {
					if model.occ[a] {
						t.Fatalf("%s: block %+v overlaps live allocation at byte %d", where, *ev.Block, a)
					}
				}
				model.occupy(*ev.Block)
				model.live[ev.AllocID] = *ev.Block
				model.req[ev.AllocID] = ev.Requested
			}
		case "free":
			b, ok := model.live[ev.AllocID]
			if !ok {
				t.Fatalf("%s: server freed unknown allocation", where)
			}
			if *ev.Block != b {
				t.Fatalf("%s: freed block %+v, model has %+v", where, *ev.Block, b)
			}
			model.release(b)
			delete(model.live, ev.AllocID)
			delete(model.req, ev.AllocID)
		}
		checkSnapshot(t, model, k, ev.snapshot, where)
		prev = ev.snapshot
	}
}

// blockAt finds the result of the i-th event and returns its block.
func mustReplay(t *testing.T, req replayRequest) replayResponse {
	t.Helper()
	res, aerr := replay(req)
	if aerr != nil {
		t.Fatalf("replay rejected batch: %v", aerr)
	}
	return res
}

func TestPlacementPolicy(t *testing.T) {
	// capacity 16. Free order-1 block at 8 and order-2 block at 0:
	// a size-2 request must take the order-1 block even though the
	// order-2 block has the lower address.
	res := mustReplay(t, replayRequest{CapacityOrder: 4, Events: []Event{
		{ID: "e1", Type: "allocate", AllocID: "a", Size: 4}, // [0,4)
		{ID: "e2", Type: "allocate", AllocID: "b", Size: 4}, // [4,8)
		{ID: "e3", Type: "allocate", AllocID: "c", Size: 2}, // [8,10)
		{ID: "e4", Type: "allocate", AllocID: "d", Size: 2}, // [10,12)
		{ID: "e5", Type: "allocate", AllocID: "e", Size: 4}, // [12,16)
		{ID: "e6", Type: "free", AllocID: "a"},              // free order2 @0
		{ID: "e7", Type: "free", AllocID: "c"},              // free order1 @8
		{ID: "e8", Type: "allocate", AllocID: "f", Size: 2}, // must land @8 (smallest fitting order)
	}})
	if got := res.Events[7].Block.Addr; got != 8 {
		t.Fatalf("smallest-order preference: f placed at %d, want 8", got)
	}
	if got := res.Events[7].Block.Order; got != 1 {
		t.Fatalf("f order %d, want 1", got)
	}

	// Lowest address wins among free blocks of the same order.
	res2 := mustReplay(t, replayRequest{CapacityOrder: 4, Events: []Event{
		{ID: "e1", Type: "allocate", AllocID: "a", Size: 2}, // [0,2)
		{ID: "e2", Type: "allocate", AllocID: "b", Size: 2}, // [2,4)
		{ID: "e3", Type: "allocate", AllocID: "c", Size: 2}, // [4,6)
		{ID: "e4", Type: "allocate", AllocID: "d", Size: 2}, // [6,8)
		{ID: "e5", Type: "allocate", AllocID: "e", Size: 8}, // [8,16)
		{ID: "e6", Type: "free", AllocID: "d"},              // free @6
		{ID: "e7", Type: "free", AllocID: "b"},              // free @2
		{ID: "e8", Type: "allocate", AllocID: "f", Size: 2}, // must land @2
	}})
	if got := res2.Events[7].Block.Addr; got != 2 {
		t.Fatalf("lowest-address tie-break: f placed at %d, want 2", got)
	}

	// Splits keep the low-address half: the first byte-sized allocation in a
	// fresh 16-byte space lands at 0 and releases buddies 1,2,4,8.
	res3 := mustReplay(t, replayRequest{CapacityOrder: 4, Events: []Event{
		{ID: "e1", Type: "allocate", AllocID: "a", Size: 1},
	}})
	snap := res3.Events[0].snapshot
	wantCounts := []int{1, 1, 1, 1, 0}
	if !reflect.DeepEqual(snap.FreeCounts, wantCounts) {
		t.Fatalf("after split freeCounts %v, want %v", snap.FreeCounts, wantCounts)
	}
	for i, b := range snap.FreeBlocks {
		if b.Addr != 1<<i {
			t.Fatalf("buddy[%d] at %d, want %d (low-half split)", i, b.Addr, 1<<i)
		}
	}
}

func TestMerging(t *testing.T) {
	// Free in an order that cascades all the way back to one full block.
	res := mustReplay(t, replayRequest{CapacityOrder: 4, Events: []Event{
		{ID: "e1", Type: "allocate", AllocID: "a", Size: 1}, // @0
		{ID: "e2", Type: "allocate", AllocID: "b", Size: 1}, // @1
		{ID: "e3", Type: "allocate", AllocID: "c", Size: 2}, // @2
		{ID: "e4", Type: "allocate", AllocID: "d", Size: 4}, // @4
		{ID: "e5", Type: "allocate", AllocID: "e", Size: 8}, // @8
		{ID: "e6", Type: "free", AllocID: "a"},
		{ID: "e7", Type: "free", AllocID: "b"}, // merge -> order1 @0
		{ID: "e8", Type: "free", AllocID: "c"}, // merge -> order2 @0
		{ID: "e9", Type: "free", AllocID: "d"}, // merge -> order3 @0
		{ID: "e10", Type: "free", AllocID: "e"}, // merge -> order4 @0
	}})
	steps := []struct {
		idx        int
		wantCounts []int
	}{
		{5, []int{1, 0, 0, 0, 0}}, // a freed, buddy @1 still allocated
		{6, []int{0, 1, 0, 0, 0}}, // a+b merged
		{7, []int{0, 0, 1, 0, 0}},
		{8, []int{0, 0, 0, 1, 0}},
		{9, []int{0, 0, 0, 0, 1}}, // fully merged
	}
	for _, s := range steps {
		if got := res.Events[s.idx].snapshot.FreeCounts; !reflect.DeepEqual(got, s.wantCounts) {
			t.Fatalf("after event %d: freeCounts %v, want %v", s.idx, got, s.wantCounts)
		}
	}
	final := res.Events[9].snapshot
	if final.TotalFree != 16 || final.MaxFreeBlock != 16 || final.ExternalFrag != 0 {
		t.Fatalf("final snapshot %+v, want fully free", final)
	}
}

func TestFragmentationOOM(t *testing.T) {
	// 8 free bytes in two 4-byte holes: an 8-byte request must OOM even
	// though totalFree >= request. Total free bytes are not allocatable bytes.
	res := mustReplay(t, replayRequest{CapacityOrder: 4, Events: []Event{
		{ID: "e1", Type: "allocate", AllocID: "a", Size: 4},
		{ID: "e2", Type: "allocate", AllocID: "b", Size: 4},
		{ID: "e3", Type: "allocate", AllocID: "c", Size: 4},
		{ID: "e4", Type: "allocate", AllocID: "d", Size: 4},
		{ID: "e5", Type: "free", AllocID: "a"}, // hole @0
		{ID: "e6", Type: "free", AllocID: "c"}, // hole @8
		{ID: "e7", Type: "allocate", AllocID: "big", Size: 8}, // OOM: fragmented
		{ID: "e8", Type: "free", AllocID: "b"},
		{ID: "e9", Type: "free", AllocID: "d"}, // everything merges back
		{ID: "e10", Type: "allocate", AllocID: "big2", Size: 8}, // reuse after merge
	}})
	before := res.Events[5].snapshot
	if before.TotalFree != 8 || before.MaxFreeBlock != 4 || before.ExternalFrag != 4 {
		t.Fatalf("pre-OOM snapshot %+v, want totalFree 8 / maxFree 4 / externalFrag 4", before)
	}
	oom := res.Events[6]
	if oom.Status != "oom" {
		t.Fatalf("fragmented allocate status %q, want oom", oom.Status)
	}
	if !reflect.DeepEqual(before, oom.snapshot) {
		t.Fatalf("OOM changed state: %+v -> %+v", before, oom.snapshot)
	}
	merged := res.Events[8].snapshot
	if merged.TotalFree != 16 || merged.MaxFreeBlock != 16 {
		t.Fatalf("after merges %+v, want fully free", merged)
	}
	reuse := res.Events[9]
	if reuse.Status != "ok" || reuse.Block.Addr != 0 || reuse.Block.Size != 8 {
		t.Fatalf("reuse after merge got %+v, want ok block [0,8)", reuse)
	}
}

func TestStructuralErrors(t *testing.T) {
	alloc := func(id, allocID string, size int) Event {
		return Event{ID: id, Type: "allocate", AllocID: allocID, Size: size}
	}
	free := func(id, allocID string) Event { return Event{ID: id, Type: "free", AllocID: allocID} }

	cases := []struct {
		name string
		req  replayRequest
	}{
		{"order too small", replayRequest{CapacityOrder: 3}},
		{"order too large", replayRequest{CapacityOrder: 17}},
		{"duplicate event id", replayRequest{CapacityOrder: 4, Events: []Event{alloc("e1", "a", 1), alloc("e1", "b", 1)}}},
		{"empty event id", replayRequest{CapacityOrder: 4, Events: []Event{{ID: "", Type: "free", AllocID: "a"}}}},
		{"unknown type", replayRequest{CapacityOrder: 4, Events: []Event{{ID: "e1", Type: "realloc", AllocID: "a"}}}},
		{"allocate without allocId", replayRequest{CapacityOrder: 4, Events: []Event{{ID: "e1", Type: "allocate", Size: 1}}}},
		{"duplicate allocation id", replayRequest{CapacityOrder: 4, Events: []Event{alloc("e1", "a", 1), alloc("e2", "a", 1)}}},
		{"size zero", replayRequest{CapacityOrder: 4, Events: []Event{alloc("e1", "a", 0)}}},
		{"size above capacity", replayRequest{CapacityOrder: 4, Events: []Event{alloc("e1", "a", 17)}}},
		{"free unknown id", replayRequest{CapacityOrder: 4, Events: []Event{free("e1", "ghost")}}},
		{"double free", replayRequest{CapacityOrder: 4, Events: []Event{alloc("e1", "a", 1), free("e2", "a"), free("e3", "a")}}},
		{"free after oom is unknown", replayRequest{CapacityOrder: 4, Events: []Event{
			alloc("e1", "a", 16), alloc("e2", "b", 1), // e2 OOMs: 16 bytes taken
			free("e3", "b"),
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, aerr := replay(tc.req); aerr == nil {
				t.Fatalf("expected structural error, got success")
			}
		})
	}

	// 301 events exceed the batch cap.
	big := replayRequest{CapacityOrder: 4}
	for i := 0; i <= maxEvents; i++ {
		big.Events = append(big.Events, alloc(fmt.Sprintf("e%d", i), fmt.Sprintf("a%d", i), 1))
	}
	if _, aerr := replay(big); aerr == nil {
		t.Fatalf("expected error for %d events", len(big.Events))
	}
	if _, aerr := replay(replayRequest{CapacityOrder: 4, Events: big.Events[:maxEvents]}); aerr != nil {
		t.Fatalf("exactly %d events must be accepted: %v", maxEvents, aerr)
	}
}

func TestReplayHTTP(t *testing.T) {
	body := `{"capacityOrder":4,"events":[
		{"id":"e1","type":"allocate","allocId":"a","size":5},
		{"id":"e2","type":"free","allocId":"a"}
	]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/replay", bytes.NewBufferString(body))
	handleReplay(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	var res replayResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Capacity != 16 || len(res.Events) != 2 {
		t.Fatalf("capacity %d events %d", res.Capacity, len(res.Events))
	}
	first := res.Events[0]
	if first.Status != "ok" || first.Block.Addr != 0 || first.Block.Size != 8 || first.InternalFrag != 3 {
		t.Fatalf("first event %+v, want ok block [0,8) internalFrag 3", first)
	}
	if res.Events[1].TotalFree != 16 {
		t.Fatalf("after free totalFree %d, want 16", res.Events[1].TotalFree)
	}

	// Structural error -> 400 with message, nothing applied.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/replay", bytes.NewBufferString(
		`{"capacityOrder":4,"events":[{"id":"e1","type":"free","allocId":"ghost"}]}`))
	handleReplay(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
	var errBody map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil || errBody["error"] == "" {
		t.Fatalf("error body %q", rec.Body)
	}

	// OOM is not an error: HTTP 200 with status "oom".
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/replay", bytes.NewBufferString(
		`{"capacityOrder":4,"events":[{"id":"e1","type":"allocate","allocId":"a","size":16},{"id":"e2","type":"allocate","allocId":"b","size":1}]}`))
	handleReplay(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("OOM batch status %d, want 200", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Events[1].Status != "oom" {
		t.Fatalf("second event status %q, want oom", res.Events[1].Status)
	}
}
