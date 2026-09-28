<script>
  // Batch editor: capacity order + the list of allocate/free operations.
  export let capacityOrder;
  export let ops; // [{type:'allocate'|'free', allocId, size}]
  export let running = false;
  export let onRun = () => {};
  export let onDemo = () => {};

  function addOp(type) {
    const id = nextAllocId();
    ops = [...ops, type === 'allocate' ? { type, allocId: id, size: 64 } : { type, allocId: lastLiveId() }];
  }

  function removeOp(i) {
    ops = ops.filter((_, j) => j !== i);
  }

  function nextAllocId() {
    const used = new Set(ops.filter((o) => o.type === 'allocate').map((o) => o.allocId));
    let n = 1;
    while (used.has('a' + n)) n++;
    return 'a' + n;
  }

  function lastLiveId() {
    const live = new Set();
    for (const op of ops) {
      if (op.type === 'allocate') live.add(op.allocId);
      else live.delete(op.allocId);
    }
    return [...live].pop() || '';
  }

  $: capacity = 1 << capacityOrder;
</script>

<section class="editor" data-testid="editor">
  <div class="row head">
    <label>
      capacity
      <select bind:value={capacityOrder} data-testid="capacity-select">
        {#each Array.from({ length: 13 }, (_, k) => k + 4) as k}
          <option value={k}>2^{k} = {1 << k} B</option>
        {/each}
      </select>
    </label>
    <span class="spacer"></span>
    <button on:click={() => addOp('allocate')} data-testid="add-alloc">+ allocate</button>
    <button on:click={() => addOp('free')} data-testid="add-free">+ free</button>
    <button on:click={onDemo} data-testid="load-demo">load demo</button>
    <button class="primary" on:click={onRun} disabled={running} data-testid="run">
      {running ? 'replaying…' : 'replay batch'}
    </button>
  </div>

  <div class="ops">
    <div class="op op-head">
      <span>#</span><span>event</span><span>op</span><span>alloc id</span><span>size (1..{capacity})</span><span></span>
    </div>
    {#each ops as op, i (i)}
      <div class="op" data-testid="op-row">
        <span class="mono muted">{i + 1}</span>
        <span class="mono muted">e{i + 1}</span>
        <select bind:value={op.type}>
          <option value="allocate">allocate</option>
          <option value="free">free</option>
        </select>
        <input class="mono" bind:value={op.allocId} placeholder="alloc id" />
        {#if op.type === 'allocate'}
          <input class="mono" type="number" min="1" max={capacity} bind:value={op.size} />
        {:else}
          <span class="muted">—</span>
        {/if}
        <button class="del" title="remove" on:click={() => removeOp(i)}>×</button>
      </div>
    {/each}
    {#if ops.length === 0}
      <p class="muted empty">no operations yet — add some or load the demo</p>
    {/if}
  </div>
</section>

<style>
  .editor {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 12px 14px;
  }
  .row { display: flex; align-items: center; gap: 10px; }
  .head { margin-bottom: 10px; flex-wrap: wrap; }
  .head label { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--muted); }
  .spacer { flex: 1; }
  .ops { display: flex; flex-direction: column; gap: 4px; max-height: 260px; overflow-y: auto; }
  .op {
    display: grid;
    grid-template-columns: 2rem 3rem 7.5rem 8rem 9rem 2rem;
    gap: 8px;
    align-items: center;
  }
  .op-head { font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--muted); }
  .op input { width: 100%; }
  .del { padding: 2px 8px; }
  .muted { color: var(--muted); }
  .empty { margin: 6px 0; font-size: 13px; }
</style>
