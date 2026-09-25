/**
 * Subsequence search with a relevance score — the matcher behind the command
 * palette, the namespace picker and every table filter.
 *
 * Why not a plain `includes()`: with hundreds of namespaces and thousands of
 * pods, "kbsys" should find "kube-system" and "chkout" should find
 * "checkout-7d9f8b". A substring filter finds neither, which is exactly the
 * "I have to scroll to find anything" problem this replaces.
 *
 * Deliberately not a dependency: this is ~40 lines and runs on every keystroke,
 * so it stays in-process with no bundle cost.
 */

export interface FuzzyMatch {
  /** Higher is better. Only meaningful relative to other matches of the same query. */
  score: number;
  /** [start, end) index pairs of the matched characters, for highlighting. */
  ranges: [number, number][];
}

/**
 * Match `query` against `text` as an ordered subsequence, case-insensitively.
 * Returns null when a character of the query is missing.
 *
 * Scoring rewards, in order: an exact substring hit, matches at the start of a
 * word (after `-`, `_`, `.`, `/` or a case change — i.e. Kubernetes naming),
 * runs of adjacent characters, and matches near the start of the string.
 */
export function fuzzyMatch(query: string, text: string): FuzzyMatch | null {
  if (!query) return { score: 1, ranges: [] };

  const q = query.toLowerCase();
  const t = text.toLowerCase();

  // Fast path: a literal substring always beats a scattered subsequence.
  const direct = t.indexOf(q);
  if (direct !== -1) {
    const boundary = direct === 0 || isBoundary(text, direct);
    return {
      score: 1000 - direct + (boundary ? 400 : 0) + q.length * 8,
      ranges: [[direct, direct + q.length]],
    };
  }

  const ranges: [number, number][] = [];
  let score = 0;
  let ti = 0;
  let runStart = -1;

  for (let qi = 0; qi < q.length; qi++) {
    const found = t.indexOf(q[qi], ti);
    if (found === -1) return null;

    if (found === ti && runStart !== -1) {
      score += 15; // adjacent to the previous match — keep the run going
    } else {
      if (runStart !== -1) ranges.push([runStart, ti]);
      runStart = found;
      score += isBoundary(text, found) ? 30 : 3;
    }
    score -= Math.min(found, 20) * 0.15; // prefer earlier matches, but gently
    ti = found + 1;
  }
  if (runStart !== -1) ranges.push([runStart, ti]);

  // Shorter haystacks are more likely to be what the user meant.
  return { score: score + Math.max(0, 40 - text.length) * 0.5, ranges };
}

/** True when index `i` starts a word: string start, after a separator, or camelCase. */
function isBoundary(text: string, i: number): boolean {
  if (i === 0) return true;
  const prev = text[i - 1];
  if (prev === "-" || prev === "_" || prev === "." || prev === "/" || prev === " " || prev === ":") return true;
  return prev === prev.toLowerCase() && text[i] === text[i].toUpperCase() && text[i] !== prev;
}

/**
 * Filter and rank `items` by a query, using `key(item)` as the haystack.
 * Stable for equal scores, so an already-sorted list keeps its order.
 */
export function fuzzyFilter<T>(items: T[], query: string, key: (item: T) => string): T[] {
  if (!query.trim()) return items;
  return items
    .map((item, i) => ({ item, i, m: fuzzyMatch(query.trim(), key(item)) }))
    .filter((r): r is { item: T; i: number; m: FuzzyMatch } => r.m !== null)
    .sort((a, b) => b.m.score - a.m.score || a.i - b.i)
    .map((r) => r.item);
}

/**
 * Split `text` into alternating plain/matched segments for rendering
 * highlighted results. Always returns the full original text.
 */
export function highlightParts(text: string, query: string): { text: string; hit: boolean }[] {
  const m = query.trim() ? fuzzyMatch(query.trim(), text) : null;
  if (!m || m.ranges.length === 0) return [{ text, hit: false }];

  const parts: { text: string; hit: boolean }[] = [];
  let cursor = 0;
  for (const [start, end] of m.ranges) {
    if (start > cursor) parts.push({ text: text.slice(cursor, start), hit: false });
    parts.push({ text: text.slice(start, end), hit: true });
    cursor = end;
  }
  if (cursor < text.length) parts.push({ text: text.slice(cursor), hit: false });
  return parts;
}
