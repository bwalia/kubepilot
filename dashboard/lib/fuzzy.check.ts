/**
 * Self-check for the fuzzy matcher. Run with `npm run test:fuzzy`.
 *
 * No test framework on purpose: this is the only non-trivial pure function in
 * the dashboard, and `tsc` is already a devDependency, so a plain assert script
 * buys the coverage without adding a runner to the build.
 */
import assert from "node:assert/strict";
import { fuzzyMatch, fuzzyFilter, highlightParts } from "./fuzzy";

// --- the behaviour the command palette depends on ------------------------

// Abbreviations find hyphenated Kubernetes names.
assert.ok(fuzzyMatch("kbsys", "kube-system"), "kbsys should match kube-system");
assert.ok(fuzzyMatch("chkout", "checkout-7d9f8b-x2k4l"), "chkout should match checkout-...");

// Missing characters, and out-of-order characters, do not match.
assert.equal(fuzzyMatch("xyz", "kube-system"), null);
assert.equal(fuzzyMatch("metsys", "kube-system"), null, "order must be respected");

// An empty query matches everything, so an unfiltered list stays whole.
assert.deepEqual(fuzzyMatch("", "anything"), { score: 1, ranges: [] });

// Case-insensitive both ways.
assert.ok(fuzzyMatch("KUBE", "kube-system"));
assert.ok(fuzzyMatch("kube", "KUBE-SYSTEM"));

// --- ranking -------------------------------------------------------------

// A literal substring beats a scattered subsequence.
const direct = fuzzyMatch("system", "kube-system")!;
const scattered = fuzzyMatch("system", "s-y-s-t-e-m")!;
assert.ok(direct.score > scattered.score, "substring hit must outrank a scattered one");

// A word-boundary hit beats a mid-word hit of the same length.
const boundary = fuzzyMatch("sys", "kube-system")!;
const midword = fuzzyMatch("sys", "resysteming")!;
assert.ok(boundary.score > midword.score, "boundary hit must outrank mid-word");

// Ranking drives the result order, not just inclusion.
assert.deepEqual(
  fuzzyFilter(["my-kube-thing", "kube-system", "kubelet"], "kube", (s) => s),
  ["kube-system", "kubelet", "my-kube-thing"],
  "prefix matches should sort above mid-string ones"
);

// Equal scores keep input order (stability), so a sorted table stays sorted.
assert.deepEqual(fuzzyFilter(["a-x", "b-x"], "x", (s) => s), ["a-x", "b-x"]);

// --- highlighting --------------------------------------------------------

// Highlight segments always reassemble into the original string — a bug here
// would silently drop characters from every name on screen.
for (const [q, text] of [
  ["kbsys", "kube-system"],
  ["system", "kube-system"],
  ["", "kube-system"],
  ["zzz", "kube-system"],
] as const) {
  assert.equal(
    highlightParts(text, q).map((p) => p.text).join(""),
    text,
    `highlight must preserve "${text}" for query "${q}"`
  );
}

assert.deepEqual(highlightParts("kube-system", "system"), [
  { text: "kube-", hit: false },
  { text: "system", hit: true },
]);

console.log("fuzzy: all checks passed");
