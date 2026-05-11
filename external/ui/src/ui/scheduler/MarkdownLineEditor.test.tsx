import React from "react";
import { cleanup, render } from "@testing-library/react";
import { afterEach, expect, test } from "vitest";
import {
  MARKDOWN_LINE_EDITOR_MIN_ROWS,
  MarkdownLineEditor,
} from "./MarkdownLineEditor";

afterEach(() => cleanup());

test("default minimum row count is 10", () => {
  expect(MARKDOWN_LINE_EDITOR_MIN_ROWS).toBe(10);
});

test("renders editor chrome", () => {
  render(<MarkdownLineEditor value="a\nb" onChange={() => {}} />);
  expect(document.querySelector(".md-line-editor")).not.toBeNull();
  expect(document.querySelector(".md-line-editor-textarea")).not.toBeNull();
});

test("gutter pads line numbers to minimum rows when text has fewer lines", () => {
  render(<MarkdownLineEditor value="only one line" onChange={() => {}} />);
  const gutter = document.querySelector(".md-line-editor-gutter");
  const lines = gutter?.textContent?.trim().split(/\n/) ?? [];
  expect(lines.length).toBe(MARKDOWN_LINE_EDITOR_MIN_ROWS);
  expect(lines[MARKDOWN_LINE_EDITOR_MIN_ROWS - 1]).toBe(
    String(MARKDOWN_LINE_EDITOR_MIN_ROWS),
  );
});

test("gutter grows past minimum when content has more lines", () => {
  const body = Array.from({ length: 14 }, (_, i) => `L${i + 1}`).join("\n");
  render(<MarkdownLineEditor value={body} onChange={() => {}} />);
  const gutter = document.querySelector(".md-line-editor-gutter");
  const lines = gutter?.textContent?.trim().split(/\n/) ?? [];
  expect(lines.length).toBe(14);
  expect(lines[13]).toBe("14");
});
