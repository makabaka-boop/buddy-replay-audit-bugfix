<script>
  import { colorFor } from './color.js';

  // One row per order would be noisy; the address bar is the flat [0, capacity)
  // view: allocated blocks colored by id, free spans gray.
  export let capacity;
  export let blocks = []; // {addr, size, kind: 'alloc'|'free', id?, requested?}
  export let highlight = null; // block of the current event

  const pct = (n) => (n / capacity) * 100;

  function isHi(b) {
    return highlight && b.addr === highlight.addr && b.size === highlight.size;
  }
</script>

<div class="wrap" data-testid="address-bar">
  <div class="bar">
    {#each blocks as b (b.kind + b.addr)}
      <div
        class="blk {b.kind}"
        class:hi={isHi(b)}
        style:left="{pct(b.addr)}%"
        style:width="{pct(b.size)}%"
        style:background={b.kind === 'alloc' ? colorFor(b.id) : undefined}
        data-testid={b.kind === 'alloc' ? `bar-alloc-${b.id}` : 'bar-free'}
      >
        {#if pct(b.size) > 7}
          <span class="lbl mono">{b.kind === 'alloc' ? b.id : 'free'} · {b.size}</span>
        {/if}
        <span class="tip mono">{b.kind === 'alloc' ? b.id : 'free'} [{b.addr}, {b.addr + b.size}) {b.size}B{b.requested ? ` (requested ${b.requested}B)` : ''}</span>
      </div>
    {/each}
  </div>
  <div class="axis mono">
    <span>0</span>
    <span>{capacity}</span>
  </div>
</div>

<style>
  .wrap { margin: 4px 0 2px; }
  .bar {
    position: relative;
    height: 44px;
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
    background: repeating-linear-gradient(45deg, #141b2b, #141b2b 8px, #151d2f 8px, #151d2f 16px);
  }
  .blk {
    position: absolute;
    top: 0;
    bottom: 0;
    border-right: 1px solid rgba(0, 0, 0, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
  }
  .blk.free { background: rgba(120, 135, 165, 0.16); }
  .blk.hi { outline: 2px solid #fff; outline-offset: -2px; z-index: 2; }
  .lbl { font-size: 11px; color: #fff; text-shadow: 0 1px 2px rgba(0, 0, 0, 0.7); white-space: nowrap; pointer-events: none; }
  .blk.free .lbl { color: var(--muted); text-shadow: none; }
  .tip {
    display: none;
    position: absolute;
    top: 105%;
    left: 0;
    background: #000;
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 2px 6px;
    font-size: 11px;
    white-space: nowrap;
    z-index: 5;
  }
  .blk:hover .tip { display: block; }
  .axis { display: flex; justify-content: space-between; font-size: 11px; color: var(--muted); margin-top: 3px; }
</style>
