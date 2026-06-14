import { describe, expect, it } from "vitest";

import {
  MAX_KEYWORDS_BYTES,
  MAX_LONG_EDGE,
  MAX_UPLOAD_BYTES,
  isKeywordsWithinLimit,
  isWithinSizeLimit,
  keywordsByteLength,
  scaledDimensions,
} from "./image";

describe("isWithinSizeLimit", () => {
  it("accepts a file at exactly the default cap", () => {
    expect(isWithinSizeLimit(MAX_UPLOAD_BYTES)).toBe(true);
  });

  it("rejects a file one byte over the default cap", () => {
    expect(isWithinSizeLimit(MAX_UPLOAD_BYTES + 1)).toBe(false);
  });

  it("accepts an empty (zero-byte) file", () => {
    expect(isWithinSizeLimit(0)).toBe(true);
  });

  it("honors an explicit limit override", () => {
    expect(isWithinSizeLimit(100, 100)).toBe(true);
    expect(isWithinSizeLimit(101, 100)).toBe(false);
  });
});

describe("keywordsByteLength", () => {
  it("counts ASCII as one byte per character", () => {
    expect(keywordsByteLength("phone")).toBe(5);
  });

  it("counts an empty string as zero", () => {
    expect(keywordsByteLength("")).toBe(0);
  });

  it("counts Sinhala (si) as multi-byte UTF-8", () => {
    // "දුරකථනය" (phone) — Sinhala code points are 3 bytes each in UTF-8,
    // so byte length must exceed the JS string length.
    const si = "දුරකථනය";
    expect(keywordsByteLength(si)).toBeGreaterThan(si.length);
  });

  it("counts Tamil (ta) as multi-byte UTF-8", () => {
    const ta = "தொலைபேசி"; // "phone"
    expect(keywordsByteLength(ta)).toBeGreaterThan(ta.length);
  });

  it("matches TextEncoder byte length for mixed scripts", () => {
    const mixed = "phone දුරකථනය தொலைபேசி";
    expect(keywordsByteLength(mixed)).toBe(
      new TextEncoder().encode(mixed).length,
    );
  });
});

describe("isKeywordsWithinLimit", () => {
  it("accepts keywords at exactly the byte cap", () => {
    const atCap = "a".repeat(MAX_KEYWORDS_BYTES);
    expect(keywordsByteLength(atCap)).toBe(MAX_KEYWORDS_BYTES);
    expect(isKeywordsWithinLimit(atCap)).toBe(true);
  });

  it("rejects keywords one byte over the cap", () => {
    const overCap = "a".repeat(MAX_KEYWORDS_BYTES + 1);
    expect(isKeywordsWithinLimit(overCap)).toBe(false);
  });

  it("counts multi-byte characters toward the cap, not character count", () => {
    // Each Sinhala code point is 3 bytes; a string whose char length is well
    // under the cap can still exceed it in bytes.
    const chars = Math.ceil(MAX_KEYWORDS_BYTES / 2); // < cap by char count
    const si = "ක".repeat(chars);
    expect(si.length).toBeLessThan(MAX_KEYWORDS_BYTES);
    expect(keywordsByteLength(si)).toBeGreaterThan(MAX_KEYWORDS_BYTES);
    expect(isKeywordsWithinLimit(si)).toBe(false);
  });

  it("honors an explicit limit override", () => {
    expect(isKeywordsWithinLimit("abcd", 4)).toBe(true);
    expect(isKeywordsWithinLimit("abcde", 4)).toBe(false);
  });
});

describe("scaledDimensions", () => {
  it("does not upscale an image already within the long-edge cap", () => {
    expect(scaledDimensions(800, 600)).toEqual({ width: 800, height: 600 });
  });

  it("leaves an image exactly at the cap unchanged", () => {
    expect(scaledDimensions(MAX_LONG_EDGE, 500)).toEqual({
      width: MAX_LONG_EDGE,
      height: 500,
    });
  });

  it("clamps a landscape image's long edge to the cap", () => {
    // 2000x1000 -> scale 0.5 -> 1000x500
    expect(scaledDimensions(2000, 1000)).toEqual({
      width: 1000,
      height: 500,
    });
  });

  it("clamps a portrait image's long edge to the cap", () => {
    // 1000x2000 -> scale 0.5 -> 500x1000
    expect(scaledDimensions(1000, 2000)).toEqual({
      width: 500,
      height: 1000,
    });
  });

  it("rounds the short edge to the nearest pixel", () => {
    // 3000x2000, scale = 1000/3000 = 0.3333..., height = 666.67 -> 667
    expect(scaledDimensions(3000, 2000)).toEqual({
      width: 1000,
      height: 667,
    });
  });

  it("handles a square image at the cap exactly", () => {
    expect(scaledDimensions(1000, 1000)).toEqual({
      width: 1000,
      height: 1000,
    });
  });

  it("honors an explicit maxLongEdge override", () => {
    expect(scaledDimensions(400, 200, 100)).toEqual({
      width: 100,
      height: 50,
    });
  });
});
