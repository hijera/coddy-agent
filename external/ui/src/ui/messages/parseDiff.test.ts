import { expect, test } from "vitest";
import { flattenDiffLines, parseDiffPatch, totalDiffLines } from "./parseDiff";

const SIMPLE_PATCH = [
  "--- a/src/foo.ts",
  "+++ b/src/foo.ts",
  "@@ -1,4 +1,5 @@",
  " line1",
  "-removed",
  "+added a",
  "+added b",
  " line4",
].join("\n");

test("parseDiffPatch extracts file path from +++ line", () => {
  const d = parseDiffPatch(SIMPLE_PATCH);
  expect(d.filePath).toBe("src/foo.ts");
});

test("parseDiffPatch uses fallbackPath when no +++ line", () => {
  const d = parseDiffPatch("@@ -1 +1 @@\n+new", "fallback.ts");
  expect(d.filePath).toBe("fallback.ts");
});

test("parseDiffPatch produces one hunk", () => {
  const d = parseDiffPatch(SIMPLE_PATCH);
  expect(d.hunks).toHaveLength(1);
});

test("parseDiffPatch assigns correct line kinds and numbers", () => {
  const d = parseDiffPatch(SIMPLE_PATCH);
  const lines = d.hunks[0].lines;
  expect(lines[0]).toMatchObject({ kind: "ctx", oldNo: 1, newNo: 1, content: "line1" });
  expect(lines[1]).toMatchObject({ kind: "del", oldNo: 2, newNo: null, content: "removed" });
  expect(lines[2]).toMatchObject({ kind: "add", oldNo: null, newNo: 2, content: "added a" });
  expect(lines[3]).toMatchObject({ kind: "add", oldNo: null, newNo: 3, content: "added b" });
  expect(lines[4]).toMatchObject({ kind: "ctx", oldNo: 3, newNo: 4, content: "line4" });
});

test("parseDiffPatch handles multiple hunks", () => {
  const patch = [
    "--- a/bar.ts",
    "+++ b/bar.ts",
    "@@ -1,2 +1,2 @@",
    "-old1",
    "+new1",
    " ctx1",
    "@@ -10,2 +10,2 @@",
    " ctx10",
    "-old11",
    "+new11",
  ].join("\n");
  const d = parseDiffPatch(patch);
  expect(d.hunks).toHaveLength(2);
  expect(d.hunks[1].lines[0]).toMatchObject({ kind: "ctx", oldNo: 10, newNo: 10 });
  expect(d.hunks[1].lines[1]).toMatchObject({ kind: "del", oldNo: 11, newNo: null });
});

test("parseDiffPatch skips backslash no-newline markers", () => {
  const patch = "@@ -1 +1 @@\n-old\n\\ No newline at end of file\n+new";
  const d = parseDiffPatch(patch);
  expect(d.hunks[0].lines).toHaveLength(2);
});

test("flattenDiffLines returns all lines across hunks", () => {
  const d = parseDiffPatch(SIMPLE_PATCH);
  expect(flattenDiffLines(d)).toHaveLength(5);
});

test("totalDiffLines counts all content lines", () => {
  const d = parseDiffPatch(SIMPLE_PATCH);
  expect(totalDiffLines(d)).toBe(5);
});
