package main

import "sort"

// Block is a contiguous half-open range [Addr, Addr+Size) whose size is a
// power of two: Size == 1<<Order. Addr is always a multiple of Size.
type Block struct {
	Addr  int `json:"addr"`
	Size  int `json:"size"`
	Order int `json:"order"`
}

// allocator is a binary buddy allocator over capacity = 1<<k bytes.
//
// Invariants:
//   - free[o] holds the sorted start addresses of free blocks of order o.
//   - No two free blocks are buddies (merges are eager), so the free lists
//     are exactly the canonical power-of-two decomposition of the free space.
//   - Allocated blocks never overlap each other or free blocks.
type allocator struct {
	k        int
	capacity int
	free     [][]int
	allocs   map[string]Block
	reqSize  map[string]int // requested bytes per live allocation, for internal fragmentation
}

func newAllocator(k int) *allocator {
	a := &allocator{
		k:        k,
		capacity: 1 << k,
		free:     make([][]int, k+1),
		allocs:   make(map[string]Block),
		reqSize:  make(map[string]int),
	}
	a.free[k] = []int{0}
	return a
}

// orderFor returns the smallest order whose block can hold size bytes.
func orderFor(size int) int {
	o := 0
	for (1 << o) < size {
		o++
	}
	return o
}

// allocate places a request of size bytes under id. It takes the free block
// of the smallest order that still fits (ties broken by lowest address) and
// splits it down, keeping the low-address half at every level. On OOM it
// reports ok=false and leaves the state untouched.
func (a *allocator) allocate(id string, size int) (Block, bool) {
	need := orderFor(size)
	for o := need; o <= a.k; o++ {
		if len(a.free[o]) == 0 {
			continue
		}
		addr := a.free[o][0]
		a.free[o] = a.free[o][1:]

		// Split from the top down. Keep the low-address half for the
		// allocation and release its high-address buddy as free.
		for current := o; current > need; current-- {
			buddyAddr := addr + 1<<(current-1)
			a.free[current-1] = insertSorted(a.free[current-1], buddyAddr)
		}

		b := Block{Addr: addr, Size: 1 << need, Order: need}
		a.allocs[id] = b
		a.reqSize[id] = size
		return b, true
	}
	return Block{}, false
}

// release removes id's allocation and merges with its buddy level by level
// while the buddy is entirely free. Callers must guarantee id is live.
func (a *allocator) release(id string) Block {
	b := a.allocs[id]
	delete(a.allocs, id)
	delete(a.reqSize, id)

	addr := b.Addr
	o := b.Order
	for o < a.k {
		buddy := addr ^ (1 << o)
		idx := searchAddr(a.free[o], buddy)
		if idx < 0 {
			break
		}
		a.free[o] = removeAt(a.free[o], idx)
		addr &^= 1 << o
		o++
	}
	a.free[o] = insertSorted(a.free[o], addr)
	return b
}

// snapshot is a consistent view of the allocator after (or before) an event.
type snapshot struct {
	InternalFrag int     `json:"internalFrag"` // sum of (block size - requested) over live allocations
	TotalFree    int     `json:"totalFree"`
	MaxFreeBlock int     `json:"maxFreeBlock"`
	ExternalFrag int     `json:"externalFrag"` // totalFree - maxFreeBlock
	FreeCounts   []int   `json:"freeCounts"`   // FreeCounts[o] = number of free blocks of order o
	FreeBlocks   []Block `json:"freeBlocks"`
}

func (a *allocator) takeSnapshot() snapshot {
	s := snapshot{FreeCounts: make([]int, a.k+1), FreeBlocks: []Block{}}
	for id, b := range a.allocs {
		s.InternalFrag += b.Size - a.reqSize[id]
	}
	for o, addresses := range a.free {
		s.FreeCounts[o] = len(addresses)
		s.TotalFree += len(addresses) << o
		if len(addresses) > 0 {
			s.MaxFreeBlock = 1 << o
		}
		for _, addr := range addresses {
			s.FreeBlocks = append(s.FreeBlocks, Block{Addr: addr, Size: 1 << o, Order: o})
		}
	}
	s.ExternalFrag = s.TotalFree - s.MaxFreeBlock
	return s
}

func insertSorted(xs []int, v int) []int {
	i := sort.SearchInts(xs, v)
	xs = append(xs, 0)
	copy(xs[i+1:], xs[i:])
	xs[i] = v
	return xs
}

// searchAddr returns the index of v in sorted xs, or -1.
func searchAddr(xs []int, v int) int {
	i := sort.SearchInts(xs, v)
	if i < len(xs) && xs[i] == v {
		return i
	}
	return -1
}

func removeAt(xs []int, i int) []int {
	return append(xs[:i], xs[i+1:]...)
}
