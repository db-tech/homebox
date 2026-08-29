import { describe, expect, test } from "vitest";
import {
  BarcodeFormat,
  BinaryBitmap,
  DecodeHintType,
  HybridBinarizer,
  MultiFormatReader,
  RGBLuminanceSource,
} from "@zxing/library";

/**
 * Why this file exists.
 *
 * The scanner showed a live camera picture but never read a barcode off a tin.
 * The cause was not the decoder but the capture size: zxing's
 * decodeFromVideoDevice asks the browser for a camera and nothing else, so it
 * hands back whatever it likes - often 640x480 - and an EAN-13 does not survive
 * that unless the code almost touches the lens.
 *
 * These tests encode a real EAN-13 and decode it at decreasing sizes, which
 * pins the threshold that made us request a 1920px stream. If somebody later
 * drops the resolution constraint, the numbers here are the argument against it.
 */

// EAN-13 module patterns.
const L = [
  "0001101",
  "0011001",
  "0010011",
  "0111101",
  "0100011",
  "0110001",
  "0101111",
  "0111011",
  "0110111",
  "0001011",
];
const G = [
  "0100111",
  "0110011",
  "0011011",
  "0100001",
  "0011101",
  "0111001",
  "0000101",
  "0010001",
  "0001001",
  "0010111",
];
const R = [
  "1110010",
  "1100110",
  "1101100",
  "1000010",
  "1011100",
  "1001110",
  "1010000",
  "1000100",
  "1001000",
  "1110100",
];

// Which of the first six digits use the G table, selected by the leading digit.
const PARITY = ["LLLLLL", "LLGLGG", "LLGGLG", "LLGGGL", "LGLLGG", "LGGLLG", "LGGGLL", "LGLGLG", "LGLGGL", "LGGLGL"];

/** Encodes a 13 digit EAN into its bar pattern, 1 = bar, 0 = space. */
function encodeEan13(code: string): string {
  if (!/^\d{13}$/.test(code)) {
    throw new Error(`not a 13 digit code: ${code}`);
  }

  const digits = code.split("").map(Number);
  const parity = PARITY[digits[0]];

  let out = "101"; // start guard
  for (let i = 0; i < 6; i++) {
    out += parity[i] === "L" ? L[digits[i + 1]] : G[digits[i + 1]];
  }
  out += "01010"; // centre guard
  for (let i = 7; i < 13; i++) {
    out += R[digits[i]];
  }
  out += "101"; // end guard

  return out;
}

/**
 * Renders the pattern as a luminance buffer. moduleWidth is how many pixels one
 * bar occupies, which is exactly what capture resolution buys you.
 */
function render(pattern: string, moduleWidth: number) {
  const quiet = 11; // EAN-13 requires a quiet zone or the decoder never starts
  const width = (pattern.length + quiet * 2) * moduleWidth;
  const height = 60;
  const data = new Uint8ClampedArray(width * height);
  data.fill(255);

  for (let m = 0; m < pattern.length; m++) {
    if (pattern[m] !== "1") continue;
    const x0 = (quiet + m) * moduleWidth;
    for (let x = x0; x < x0 + moduleWidth; x++) {
      for (let y = 0; y < height; y++) {
        data[y * width + x] = 0;
      }
    }
  }

  return { data, width, height };
}

function shippedHints() {
  const hints = new Map();
  hints.set(DecodeHintType.POSSIBLE_FORMATS, [
    BarcodeFormat.EAN_13,
    BarcodeFormat.EAN_8,
    BarcodeFormat.UPC_A,
    BarcodeFormat.UPC_E,
    BarcodeFormat.CODE_128,
    BarcodeFormat.CODE_39,
    BarcodeFormat.ITF,
    BarcodeFormat.QR_CODE,
  ]);
  hints.set(DecodeHintType.TRY_HARDER, true);
  return hints;
}

/**
 * Box-downsamples a crisp rendering to a target width, which is what a camera
 * sensor does to a barcode in the frame. This matters: a pixel-perfect
 * rendering decodes even at one pixel per bar, while a real captured frame at
 * that scale does not, because averaging smears neighbouring bars together.
 */
function downsample(
  src: { data: Uint8ClampedArray; width: number; height: number },
  targetWidth: number
): { data: Uint8ClampedArray; width: number; height: number } {
  const scale = src.width / targetWidth;
  const targetHeight = Math.max(1, Math.round(src.height / scale));
  const out = new Uint8ClampedArray(targetWidth * targetHeight);

  for (let y = 0; y < targetHeight; y++) {
    for (let x = 0; x < targetWidth; x++) {
      const x0 = Math.floor(x * scale);
      const x1 = Math.min(src.width, Math.floor((x + 1) * scale));
      const y0 = Math.floor(y * scale);
      const y1 = Math.min(src.height, Math.floor((y + 1) * scale));

      let sum = 0;
      let n = 0;
      for (let sy = y0; sy < Math.max(y0 + 1, y1); sy++) {
        for (let sx = x0; sx < Math.max(x0 + 1, x1); sx++) {
          sum += src.data[sy * src.width + sx];
          n++;
        }
      }
      out[y * targetWidth + x] = Math.round(sum / n);
    }
  }

  return { data: out, width: targetWidth, height: targetHeight };
}

/** Decodes a crisp rendering at the given bar width. */
function decodeAt(code: string, moduleWidth: number): string | null {
  const { data, width, height } = render(encodeEan13(code), moduleWidth);
  const bitmap = new BinaryBitmap(new HybridBinarizer(new RGBLuminanceSource(data, width, height)));

  const reader = new MultiFormatReader();
  reader.setHints(shippedHints());

  try {
    return reader.decode(bitmap).getText();
  } catch {
    return null;
  }
}

// A real product code, so a broken encoder shows up as a wrong payload rather
// than as a silent NotFound.
const NUTELLA = "3017620422003";

describe("EAN-13 fixture", () => {
  test("the encoder produces something the decoder reads back correctly", () => {
    expect(decodeAt(NUTELLA, 4)).toBe(NUTELLA);
  });

  test("the pattern has the expected length", () => {
    // 3 + 6*7 + 5 + 6*7 + 3
    expect(encodeEan13(NUTELLA)).toHaveLength(95);
  });
});

/** Decodes the code as it would appear occupying `capturedWidth` pixels. */
function decodeAsCaptured(code: string, capturedWidth: number): string | null {
  const crisp = render(encodeEan13(code), 8);
  const frame = downsample(crisp, capturedWidth);
  const bitmap = new BinaryBitmap(new HybridBinarizer(new RGBLuminanceSource(frame.data, frame.width, frame.height)));

  const reader = new MultiFormatReader();
  reader.setHints(shippedHints());

  try {
    return reader.decode(bitmap).getText();
  } catch {
    return null;
  }
}

describe("capture size is what makes scanning work", () => {
  // A 640x480 stream with the tin at a comfortable distance puts the code in
  // roughly this many pixels. This is the case that looked like "nothing
  // happens" - the decoder was running fine, it just had nothing to work with.
  test("a code captured at ~150px cannot be decoded", () => {
    expect(decodeAsCaptured(NUTELLA, 150)).toBeNull();
  });

  // A 1920px stream at the same distance lands here comfortably.
  test("a code captured at ~400px decodes", () => {
    expect(decodeAsCaptured(NUTELLA, 400)).toBe(NUTELLA);
  });

  // Locks in the threshold that justifies the resolution constraint. If this
  // shifts a lot after a zxing upgrade, the constraint deserves a fresh look.
  test("the usable floor sits between 150 and 400 captured pixels", () => {
    const widths = [120, 150, 200, 250, 300, 400, 600];
    const results = widths.map(w => ({ w, ok: decodeAsCaptured(NUTELLA, w) === NUTELLA }));

    const firstOk = results.findIndex(r => r.ok);
    expect(firstOk).toBeGreaterThan(0);
    expect(results[firstOk].w).toBeGreaterThan(150);
    expect(results[firstOk].w).toBeLessThanOrEqual(400);

    // Once it decodes, more pixels must not make it worse.
    for (const r of results.slice(firstOk)) {
      expect(r.ok).toBe(true);
    }
  });
});
