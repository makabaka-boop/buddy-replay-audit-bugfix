<script>
  import { onMount } from 'svelte';
  import Editor from './lib/Editor.svelte';
  import AddressBar from './lib/AddressBar.svelte';
  import SplitTree from './lib/SplitTree.svelte';
  import MetricsPanel from './lib/MetricsPanel.svelte';
  import EventLog from './lib/EventLog.svelte';

  let capacityOrder = 8;
  let ops = [];
  let response = null;
  let error = '';
  let step = -1; // -1 = initial state, i = after events[i]
  let playing = false;
  let running = false;
  let timer = null;

  const DEMO = [
    { type: 'allocate', allocId: 'a', size: 64 },
    { type: 'allocate', allocId: 'b', size: 50 },
    { type: 'allocate', allocId: 'c', size: 64 },
    { type: 'allocate', allocId: 'd', size: 64 },
    { type: 'free', allocId: 'b' },
    { type: 'free', allocId: 'd' },
    { type: 'allocate', allocId: 'e1', size: 128 }, // OOM: 128 B free, but in two 64 B holes
    { type: 'free', allocId: 'a' },
    { type: 'free', allocId: 'c' }, // buddies merge back to one 256 B block
    { type: 'allocate', allocId: 'e2', size: 128 }, // reuse
  ];

  function loadDemo() {
    capacityOrder = 8;
    ops = DEMO.map((op) => ({ ...op }));
  }

  async function run() {
    stop();
    error = '';
    running = true;
    const payload = {
      capacityOrder,
      events: ops.map((op, i) =>
        op.type === 'allocate'
          ? { id: `e${i + 1}`, type: 'allocate', allocId: op.allocId.trim(), size: Number(op.size) }
          : { id: `e${i + 1}`, type: 'free', allocId: op.allocId.trim() }
      ),
    };
    try {
      const res = await fetch('/api/replay', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      const body = await res.json();
      if (!res.ok) {
        error = body.error || `replay failed (${res.status})`;
        response = null;
        step = -1;
      } else {
        response = body;
        step = -1;
      }
    } catch (e) {
      error = `cannot reach allocator: ${e.message}`;
      response = null;
    } finally {
      running = false;
    }
  }

  // --- stepping -----------------------------------------------------------

  $: events = response ? response.events : [];
  $: maxStep = events.length - 1;
  $: snap = response ? (step >= 0 ? events[step] : response.initial) : null;
  $: currentEvent = step >= 0 ? events[step] : null;
  $: highlight = currentEvent ? currentEvent.block : null;

  // Live allocations after `step`, derived from the event deltas.
  $: liveAllocs = (() => {
    const live = new Map();
    for (let i = 0; i <= step && i < events.length; i++) {
      const ev = events[i];
      if (ev.type === 'free') continue;
      if (ev.type === 'allocate') {
        live.set(ev.allocId, { ...ev.block, id: ev.allocId, requested: ev.requested });
      } else {
        live.delete(ev.allocId);
      }
    }
    return [...live.values()];
  })();

  $: barBlocks = response
    ? [
        ...snap.freeBlocks.map((b) => ({ ...b, kind: 'free' })),
        ...liveAllocs.map((a) => ({ ...a, kind: 'alloc' })),
      ].sort((x, y) => x.addr - y.addr)
    : [];

  function goTo(i) {
    stop();
    step = Math.max(-1, Math.min(i, maxStep));
  }

  function togglePlay() {
    if (playing) return stop();
    if (step >= maxStep) step = -1;
    playing = true;
    timer = setInterval(() => {
      if (step >= maxStep) stop();
      else step += 1;
    }, 900);
  }

  function stop() {
    playing = false;
    if (timer) clearInterval(timer);
    timer = null;
  }

  function describe(ev) {
    if (!ev) return 'initial state: one free block covering the whole capacity';
    if (ev.type === 'free') {
      return `${ev.id}: free ${ev.allocId} → released [${ev.block.addr}, ${ev.block.addr + ev.block.size}), buddies merged where possible`;
    }
    if (ev.status === 'oom') {
      return `${ev.id}: allocate ${ev.allocId} ${ev.requested} B → OOM — totalFree ${ev.totalFree} B would fit, but the largest free block is ${ev.maxFreeBlock} B (external fragmentation)`;
    }
    const frag = ev.block.size - ev.requested;
    return `${ev.id}: allocate ${ev.allocId} ${ev.requested} B → [${ev.block.addr}, ${ev.block.addr + ev.block.size})` +
      (frag > 0 ? ` (internal frag ${frag} B)` : '');
  }

  onMount(async () => {
    loadDemo();
    await run();
  });
</script>

<main>
  <header>
    <h1>Buddy Allocator Desk</h1>
    <p class="sub">
      requests round up to power-of-two blocks · smallest fitting order, lowest address ·
      frees merge only with a fully-free buddy
    </p>
  </header>

  <Editor bind:capacityOrder bind:ops {running} onRun={run} onDemo={() => { loadDemo(); run(); }} />

  {#if error}
    <div class="banner err" data-testid="error-banner">batch rejected: {error}</div>
  {/if}

  {#if response}
    <section class="player">
      <div class="controls">
        <button on:click={() => goTo(-1)} disabled={step <= -1} data-testid="step-first">«</button>
        <button on:click={() => goTo(step - 1)} disabled={step <= -1} data-testid="step-prev">‹</button>
        <button class="play" on:click={togglePlay} data-testid="play">{playing ? '‖' : '▶'}</button>
        <button on:click={() => goTo(step + 1)} disabled={step >= maxStep} data-testid="step-next">›</button>
        <button on:click={() => goTo(maxStep)} disabled={step >= maxStep} data-testid="step-last">»</button>
        <input
          type="range"
          min="0"
          max={events.length}
          value={step + 1}
          on:input={(e) => goTo(+e.target.value - 1)}
          data-testid="slider"
        />
        <span class="mono pos" data-testid="position">{step + 1} / {events.length}</span>
      </div>
      <p class="describe mono" class:oom={currentEvent && currentEvent.status === 'oom'} data-testid="describe">
        {describe(currentEvent)}
      </p>
    </section>

    <MetricsPanel {snap} />

    <section class="viz">
      <h2>address space <span class="mono dim">0 … {response.capacity} B</span></h2>
      <AddressBar capacity={response.capacity} blocks={barBlocks} {highlight} />
      <h2>split tree</h2>
      <SplitTree
        k={response.capacityOrder}
        capacity={response.capacity}
        freeBlocks={snap.freeBlocks}
        allocs={liveAllocs}
        {highlight}
      />
    </section>

    <section>
      <h2>events</h2>
      <EventLog {events} {step} onSelect={goTo} />
    </section>
  {/if}
</main>

<style>
  main { max-width: 1080px; margin: 0 auto; padding: 20px 16px 60px; display: flex; flex-direction: column; gap: 16px; }
  header h1 { margin: 0; font-size: 22px; }
  .sub { margin: 4px 0 0; color: var(--muted); font-size: 13px; }
  h2 { font-size: 14px; margin: 14px 0 8px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.06em; }
  .dim { text-transform: none; letter-spacing: 0; }
  .banner { border-radius: 8px; padding: 10px 14px; font-size: 13px; }
  .banner.err { background: rgba(255, 107, 107, 0.12); border: 1px solid rgba(255, 107, 107, 0.5); color: var(--err); }
  .player {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 10px 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .controls { display: flex; align-items: center; gap: 8px; }
  .controls input[type='range'] { flex: 1; accent-color: var(--accent); }
  .play { width: 44px; }
  .pos { color: var(--muted); font-size: 12px; min-width: 64px; text-align: right; }
  .describe { margin: 0; font-size: 12.5px; color: var(--text); }
  .describe.oom { color: var(--oom); }
  .viz { background: var(--panel); border: 1px solid var(--border); border-radius: 10px; padding: 4px 14px 14px; }
</style>
