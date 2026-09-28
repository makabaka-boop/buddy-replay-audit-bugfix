<script>
  // Per-event log; clicking a row jumps the replay to that step.
  export let events = [];
  export let step = -1; // index of the last applied event (-1 = initial state)
  export let onSelect = () => {};
</script>

<div class="log" data-testid="event-log">
  <div class="erow ehead">
    <span>event</span><span>op</span><span>alloc</span><span>request</span><span>block</span><span>status</span><span>ext frag</span><span>int frag</span>
  </div>
  {#each events as ev, i (ev.id)}
    <button
      class="erow"
      class:current={i === step}
      class:done={i <= step}
      on:click={() => onSelect(i)}
      data-testid="ev-{ev.id}"
    >
      <span class="mono">{ev.id}</span>
      <span>{ev.type}</span>
      <span class="mono">{ev.allocId}</span>
      <span class="mono">{ev.type === 'allocate' ? ev.requested + ' B' : '—'}</span>
      <span class="mono">
        {#if ev.block}[{ev.block.addr}, {ev.block.addr + ev.block.size}){:else}—{/if}
      </span>
      <span>
        <span class="badge {ev.status}" data-testid={ev.status === 'oom' ? 'badge-oom' : undefined}>{ev.status}</span>
      </span>
      <span class="mono muted">{ev.externalFrag} B</span>
      <span class="mono muted">{ev.internalFrag} B</span>
    </button>
  {/each}
</div>

<style>
  .log { display: flex; flex-direction: column; gap: 2px; max-height: 300px; overflow-y: auto; }
  .erow {
    display: grid;
    grid-template-columns: 3.5rem 4.5rem 5rem 5.5rem 8.5rem 4rem 5rem 5rem;
    gap: 8px;
    align-items: center;
    text-align: left;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 6px;
    padding: 4px 8px;
    font-size: 12px;
    color: var(--text);
  }
  .erow:not(.ehead) { cursor: pointer; }
  .erow:not(.ehead):hover { background: var(--panel-2); }
  .erow.ehead { font-size: 10px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--muted); cursor: default; }
  .erow.current { border-color: var(--accent); background: var(--panel-2); }
  .erow:not(.done):not(.current) { opacity: 0.45; }
  .badge { padding: 1px 8px; border-radius: 10px; font-size: 11px; font-weight: 600; }
  .badge.ok { background: rgba(63, 206, 143, 0.15); color: var(--ok); }
  .badge.oom { background: rgba(245, 166, 35, 0.18); color: var(--oom); }
  .muted { color: var(--muted); }
</style>
