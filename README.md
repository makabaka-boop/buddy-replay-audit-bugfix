# Buddy Allocator Replay

A binary buddy-memory-allocator simulator with an HTTP replay service and an
interactive visualization desk.

- **allocator/** — Go HTTP service. Replays a batch of allocate/free events
  against a fresh buddy allocator and returns the per-event trace.
- **desk/** — Svelte page. Edits a batch, sends it to the service, and steps
  through the returned trace with a split tree, an address bar, and
  fragmentation metrics.
- **docker-compose.yml** — runs the two services.

## Buddy rules

- Capacity is `2^k` bytes, `k` in 4..16. Block sizes are powers of two
  (order 0 = 1 byte … order k = whole capacity).
- A request of `n` bytes rounds up to the smallest order that fits
  (internal fragmentation = block size − requested).
- Allocation takes the free block of the **smallest fitting order**; ties
  break to the **lowest address**. Splitting keeps the **low-address half**
  and releases the high half as a free buddy, level by level.
- Freeing merges with the buddy (`addr ^ (1 << order)`) only while the buddy
  is entirely free, cascading upward.
- **OOM is a valid event**: if no single free block fits, the event is
  recorded with status `oom` and the state is unchanged. Total free bytes are
  *not* allocatable bytes — external fragmentation
  (`totalFree − maxFreeBlock`) is exactly what a large request cannot use.
- **Structural errors reject the whole batch** (HTTP 400): duplicate event
  ids, duplicate allocation ids (allocation ids are globally unique per
  batch), sizes outside `1..capacity`, unknown event types, and `free` of an
  unknown or already-freed allocation id.

## API

`POST /api/replay`

```json
{
  "capacityOrder": 8,
  "events": [
    { "id": "e1", "type": "allocate", "allocId": "a", "size": 100 },
    { "id": "e2", "type": "free", "allocId": "a" }
  ]
}
```

→ `200` with the initial snapshot plus one entry per event:

```json
{
  "capacity": 256,
  "capacityOrder": 8,
  "initial": { "totalFree": 256, "maxFreeBlock": 256, "externalFrag": 0,
               "internalFrag": 0, "freeCounts": [0,0,0,0,0,0,0,0,1],
               "freeBlocks": [{ "addr": 0, "size": 256, "order": 8 }] },
  "events": [
    { "id": "e1", "type": "allocate", "allocId": "a", "requested": 100,
      "status": "ok", "block": { "addr": 0, "size": 128, "order": 7 },
      "internalFrag": 28, "totalFree": 128, "maxFreeBlock": 128,
      "externalFrag": 0, "freeCounts": [0,0,0,0,0,0,0,1,0],
      "freeBlocks": [{ "addr": 128, "size": 128, "order": 7 }] }
  ]
}
```

Each event carries the state snapshot taken **after** it was applied, so the
desk can replay the trace step by step. Structural errors → `400
{"error": "..."}` with nothing applied. At most 300 events per batch.

## Run

```sh
docker compose up --build
# desk:      http://localhost:5173
# allocator: http://localhost:8080/api/health
```

Local development without Docker:

```sh
cd allocator && go run .          # serves :8080
cd desk && npm install && npm run dev   # serves :5173, proxies /api → :8080
```

The page loads a demo batch that walks the main flow: four allocations fill
the space → two frees leave two 64 B holes → a 128 B allocate **OOMs on
external fragmentation** (128 B free, largest hole 64 B) → the remaining
frees merge everything back → a 128 B allocate **reuses** address 0.

## Tests

```sh
cd allocator && go test ./...
```

Go tests simulate randomized batches against an independent byte-occupancy
reference model and assert: allocated blocks never overlap, free + occupied
bytes conserve the capacity, the placement policy (smallest order, lowest
address, low-half splits) is followed exactly, frees merge to the canonical
decomposition, OOM leaves the state untouched, and structural errors reject
the batch.

Browser end-to-end (allocate → free → reuse main flow, plus batch rejection):

```sh
docker compose up -d --build
cd desk && npx playwright install chromium && npm run e2e
```
