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

  const nodeKey = (order, addr) => `${order}:${addr}`;

  function buildTree(k, capacity, freeBlocks, allocs) {
    const byKey = new Map();

    const root = { addr: 0, size: capacity, order: k, key: nodeKey(k, 0), kind: 'internal' };
    byKey.set(root.key, root);

    for (const leaf of freeBlocks) {
      const node = { ...leaf, key: nodeKey(leaf.order, leaf.addr), kind: 'free' };
      byKey.set(node.key, node);
    }
    for (const leaf of allocs) {
      const node = { ...leaf, key: nodeKey(leaf.order, leaf.addr), kind: 'alloc' };
      byKey.set(node.key, node);
    }

    // Reconstruct every buddy that was split on the paths from the current
    // leaves back to the root, so intermediate states remain auditable.
    for (const leaf of [...freeBlocks, ...allocs]) {
      let current = { ...leaf };
      while (current.order < k) {
        const parentOrder = current.order + 1;
        const parentAddr = current.addr & ~((1 << parentOrder) - 1);
        const parentKey = nodeKey(parentOrder, parentAddr);
        if (!byKey.has(parentKey)) {
          byKey.set(parentKey, {
            addr: parentAddr,
            size: 1 << parentOrder,
            order: parentOrder,
            key: parentKey,
            kind: 'internal',
          });
        }
        current = { addr: parentAddr, order: parentOrder };
      }
    }

    const nodes = [...byKey.values()].sort((a, b) => b.order - a.order || a.addr - b.addr);
    const edges = nodes
      .filter((n) => n.order < k)
      .map((n) => {
        const parentAddr = n.addr & ~((1 << (n.order + 1)) - 1);
        return {
          from: byKey.get(nodeKey(n.order + 1, parentAddr)),
          to: n,
        };
      })
      .filter((e) => e.from);

    return { nodes, edges };
  }

  $: tree = buildTree(k, capacity, freeBlocks, allocs);
  $: nodes = tree.nodes;
  $: edges = tree.edges;

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
