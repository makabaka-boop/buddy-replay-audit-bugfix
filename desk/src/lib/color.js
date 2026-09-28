// Deterministic color per allocation id.
export function colorFor(id) {
  let h = 0;
  for (let i = 0; i < id.length; i++) {
    h = (h * 31 + id.charCodeAt(i)) >>> 0;
  }
  return `hsl(${h % 360} 65% 52%)`;
}

export function fmtBytes(n) {
  return `${n} B`;
}
