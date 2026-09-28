<script>
  // Fragmentation metrics for the snapshot at the current step.
  export let snap; // {internalFrag, totalFree, maxFreeBlock, externalFrag, freeCounts}

  $: maxCount = Math.max(1, ...(snap ? snap.freeCounts : [1]));
</script>

{#if snap}
  <div class="metrics" data-testid="metrics">
    <div class="stat">
      <span class="k">total free</span>
      <span class="v mono" data-testid="m-total-free">{snap.totalFree} B</span>
    </div>
    <div class="stat">
      <span class="k">max free block</span>
      <span class="v mono" data-testid="m-max-free">{snap.maxFreeBlock} B</span>
    </div>
    <div class="stat warn" title="totalFree − maxFreeBlock: bytes that are free but unusable for a single large request">
      <span class="k">external frag</span>
      <span class="v mono" data-testid="m-ext-frag">{snap.externalFrag} B</span>
    </div>
    <div class="stat" title="sum of (block size − requested) over live allocations">
      <span class="k">internal frag</span>
      <span class="v mono" data-testid="m-int-frag">{snap.internalFrag} B</span>
    </div>
    <div class="orders">
      {#each snap.freeCounts as count, order}
        <div class="orow" title="order {order}: block size {1 << order} B">
          <span class="mono olbl">{order}</span>
          <div class="otrack">
            <div class="ofill" style:width="{(count / maxCount) * 100}%"></div>
          </div>
          <span class="mono ocount">{count}</span>
        </div>
      {/each}
    </div>
  </div>
{/if}

<style>
  .metrics {
    display: flex;
    gap: 10px;
    align-items: stretch;
    flex-wrap: wrap;
  }
  .stat {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 8px 12px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 110px;
  }
  .stat.warn { border-color: rgba(245, 166, 35, 0.5); }
  .k { font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em; color: var(--muted); }
  .v { font-size: 16px; }
  .orders {
    flex: 1;
    min-width: 220px;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 8px 12px;
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 2px;
  }
  .orow { display: flex; align-items: center; gap: 8px; }
  .olbl { width: 1.4em; text-align: right; font-size: 11px; color: var(--muted); }
  .otrack { flex: 1; height: 8px; background: var(--panel-2); border-radius: 4px; overflow: hidden; }
  .ofill { height: 100%; background: var(--accent); border-radius: 4px; transition: width 0.2s; }
  .ocount { width: 2.5em; font-size: 11px; color: var(--muted); }
</style>
