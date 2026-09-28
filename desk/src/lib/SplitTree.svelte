<script>
  import { colorFor } from './color.js';

  // Split tree: every block that exists (allocated leaf, free leaf, or an
  // internal node that was split to reach them) up to the root.
  export let k;
  export let capacity;
  export let freeBlocks = []; // {addr, size, order}
  export let allocs = [];     // {addr, size, order, id}
  export let highlight = null;

  const W = 1000;
  const ROW_H = 36;
  const NODE_H = 24;

  function buildNodes() {
    const byKey = new Map();
    const addNode = (addr, size, order, kind, id) => {
      const key = `${order}:${addr}`;
      if (!byKey.has(key)) {
        byKey.set(key, { addr, size, order, kind, id, key });
      }
    };
    const addParents = (leaf) => {
      let addr = leaf.addr;
      for (let order = leaf.order; order < k; order++) {
        addr &= ~(1 << order);
        addNode(addr, 1 << (order + 1), order + 1, 'internal');
      }
    };

    for (const b of freeBlocks) {
      addNode(b.addr, b.size, b.order, 'free');
      addParents(b);
    }
    for (const a of allocs) {
      addNode(a.addr, a.size, a.order, 'alloc', a.id);
      addParents(a);
    }
    return [...byKey.values()];
  }

  $: nodes = buildNodes(k, freeBlocks, allocs);
  $: nodeByKey = new Map(nodes.map((n) => [n.key, n]));
  $: edges = nodes
    .filter((n) => n.order < k)
    .map((n) => {
      const pAddr = n.addr & ~((1 << (n.order + 1)) - 1);
      return { from: nodeByKey.get(n.order + 1 + ':' + pAddr), to: n };
    })
    .filter((e) => e.from);

  const cx = (n) => ((n.addr + n.size / 2) / capacity) * W;
  const yTop = (order) => (k - order) * ROW_H + 10;
  const nodeW = (n) => Math.max((n.size / capacity) * W - 6, 3);

  function isHi(n) {
    return highlight && n.addr === highlight.addr && n.size === highlight.size;
  }

  $: height = (k + 1) * ROW_H + 20;
</script>

<div class="tree" data-testid="split-tree">
  <svg viewBox="0 0 {W} {height}" preserveAspectRatio="xMidYMin meet" style="height:{height}px">
    {#each edges as e (e.to.key)}
      <line
        x1={cx(e.from)} y1={yTop(e.from.order) + NODE_H}
        x2={cx(e.to)} y2={yTop(e.to.order)}
        class="edge"
      />
    {/each}
    {#each nodes as n (n.key)}
      <g>
        <rect
          x={cx(n) - nodeW(n) / 2}
          y={yTop(n.order)}
          width={nodeW(n)}
          height={NODE_H}
          rx="4"
          class="node {n.kind}"
          class:hi={isHi(n)}
          style:fill={n.kind === 'alloc' ? colorFor(n.id) : undefined}
          data-testid={n.kind === 'alloc' ? `tree-alloc-${n.id}` : undefined}
        >
          <title>{n.kind}{n.id ? ' ' + n.id : ''} · order {n.order} · [{n.addr}, {n.addr + n.size})</title>
        </rect>
        {#if nodeW(n) > 44}
          <text x={cx(n)} y={yTop(n.order) + NODE_H / 2 + 4} class="nlbl" class:dim={n.kind !== 'alloc'}>
            {n.kind === 'alloc' ? n.id : n.addr}
          </text>
        {/if}
      </g>
    {/each}
    <text x="4" y={yTop(k) + NODE_H / 2 + 4} class="axis">order {k}</text>
    <text x="4" y={yTop(0) + NODE_H / 2 + 4} class="axis">order 0</text>
  </svg>
</div>

<style>
  .tree { overflow-x: auto; }
  svg { width: 100%; display: block; }
  .edge { stroke: var(--border); stroke-width: 1.5; }
  .node { stroke: var(--border); stroke-width: 1; }
  .node.internal { fill: var(--panel-2); }
  .node.free { fill: rgba(120, 135, 165, 0.25); stroke-dasharray: 3 2; }
  .node.hi { stroke: #fff; stroke-width: 2.5; }
  .nlbl { fill: #fff; font-size: 11px; text-anchor: middle; pointer-events: none; font-family: Consolas, monospace; }
  .nlbl.dim { fill: var(--muted); }
  .axis { fill: var(--muted); font-size: 10px; }
</style>
