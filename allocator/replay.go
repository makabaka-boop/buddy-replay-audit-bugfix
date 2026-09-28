package main

import "fmt"

const (
	minOrder  = 4
	maxOrder  = 16
	maxEvents = 300
)

// Event is one allocate/free operation in a batch.
type Event struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // "allocate" | "free"
	AllocID string `json:"allocId,omitempty"`
	Size    int    `json:"size,omitempty"`
}

type replayRequest struct {
	CapacityOrder int     `json:"capacityOrder"` // k, capacity = 1<<k bytes
	Events        []Event `json:"events"`
}

// eventResult is the outcome of one event plus the full state snapshot taken
// after it was applied (OOM events repeat the previous state).
type eventResult struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	AllocID   string `json:"allocId,omitempty"`
	Requested int    `json:"requested,omitempty"`
	Status    string `json:"status"` // "ok" | "oom"
	Block     *Block `json:"block,omitempty"`
	snapshot
}

type replayResponse struct {
	Capacity      int           `json:"capacity"`
	CapacityOrder int           `json:"capacityOrder"`
	Initial       snapshot      `json:"initial"`
	Events        []eventResult `json:"events"`
}

// apiError marks a structural error: the whole batch is rejected with 400.
type apiError struct{ msg string }

func (e *apiError) Error() string { return e.msg }

func errf(format string, args ...any) *apiError {
	return &apiError{msg: fmt.Sprintf(format, args...)}
}

// replay validates the batch and simulates it against a fresh allocator.
// Structural errors (bad shape, duplicate ids, freeing an unknown or already
// freed allocation, ...) reject the entire batch. An allocate that cannot be
// satisfied is a valid OOM event: it is recorded with status "oom" and the
// state carries on unchanged.
func replay(req replayRequest) (replayResponse, *apiError) {
    if req.CapacityOrder < minOrder || req.CapacityOrder > maxOrder {
        return replayResponse{}, errf("capacityOrder must be between %d and %d", minOrder, maxOrder)
    }
    a := newAllocator(req.CapacityOrder)
    res := replayResponse{Capacity: a.capacity, CapacityOrder: req.CapacityOrder,
        Initial: a.takeSnapshot(), Events: []eventResult{}}
    for _, ev := range req.Events {
        item := eventResult{ID: ev.ID, Type: ev.Type, AllocID: ev.AllocID}
        if ev.Type == "allocate" {
            item.Requested = ev.Size
            if block, ok := a.allocate(ev.AllocID, ev.Size); ok {
                item.Status = "ok"
                item.Block = &block
            } else {
                item.Status = "oom"
            }
        } else if ev.Type == "free" {
            block := a.release(ev.AllocID)
            item.Status = "ok"
            item.Block = &block
        } else {
            item.Status = "ok"
        }
        item.snapshot = a.takeSnapshot()
        res.Events = append(res.Events, item)
    }
    return res, nil
}
