export type DiffLineKind = "add" | "del" | "ctx";

export interface ParsedDiffLine {
  kind: DiffLineKind;
  /** Null for added lines (no old-file line number). */
  oldNo: number | null;
  /** Null for removed lines (no new-file line number). */
  newNo: number | null;
  /** Line content without the leading +/-/space character. */
  content: string;
}

export interface ParsedDiffHunk {
  header: string;
  lines: ParsedDiffLine[];
}

export interface ParsedDiff {
  filePath: string;
  hunks: ParsedDiffHunk[];
}

/** Parse a unified diff (git diff / diff -u) into structured hunks and lines. */
export function parseDiffPatch(patch: string, fallbackPath = ""): ParsedDiff {
  const rawLines = patch.split("\n");
  let filePath = fallbackPath;
  const hunks: ParsedDiffHunk[] = [];
  let currentHunk: ParsedDiffHunk | null = null;
  let oldNo = 1;
  let newNo = 1;

  for (const raw of rawLines) {
    if (raw.startsWith("---")) {
      const m = /^---\s+(?:a\/)?(.+)$/.exec(raw);
      if (m && !filePath) filePath = m[1].trim();
      continue;
    }
    if (raw.startsWith("+++")) {
      const m = /^\+\+\+\s+(?:b\/)?(.+)$/.exec(raw);
      if (m) filePath = m[1].trim();
      continue;
    }
    if (raw.startsWith("@@")) {
      const m = /^@@\s+-(\d+)(?:,\d+)?\s+\+(\d+)(?:,\d+)?\s+@@/.exec(raw);
      oldNo = m ? parseInt(m[1], 10) : 1;
      newNo = m ? parseInt(m[2], 10) : 1;
      currentHunk = { header: raw, lines: [] };
      hunks.push(currentHunk);
      continue;
    }
    if (!currentHunk) continue;
    // "\ No newline at end of file"
    if (raw.startsWith("\\")) continue;

    if (raw.startsWith("+")) {
      currentHunk.lines.push({
        kind: "add",
        oldNo: null,
        newNo: newNo++,
        content: raw.slice(1),
      });
    } else if (raw.startsWith("-")) {
      currentHunk.lines.push({
        kind: "del",
        oldNo: oldNo++,
        newNo: null,
        content: raw.slice(1),
      });
    } else if (raw.startsWith(" ")) {
      currentHunk.lines.push({
        kind: "ctx",
        oldNo: oldNo++,
        newNo: newNo++,
        content: raw.slice(1),
      });
    }
    // blank lines between hunks are ignored
  }

  return { filePath, hunks };
}

/** All diff lines from all hunks, in order. */
export function flattenDiffLines(parsed: ParsedDiff): ParsedDiffLine[] {
  return parsed.hunks.flatMap((h) => h.lines);
}

/** Total number of content lines across all hunks. */
export function totalDiffLines(parsed: ParsedDiff): number {
  return parsed.hunks.reduce((sum, h) => sum + h.lines.length, 0);
}
