import { describe, expect, test } from "vitest";
import { centreCrop, decodeFrame, decodeLuminance, toLuminance } from "../lib/barcode/decode";

/**
 * Covers the frame decoding used by the scanner. The camera plumbing cannot be
 * tested here, but everything downstream of "we have pixels" can, and that is
 * where the scanner kept failing.
 */

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
const PARITY = ["LLLLLL", "LLGLGG", "LLGGLG", "LLGGGL", "LGLLGG", "LGGLLG", "LGGGLL", "LGLGLG", "LGLGGL", "LGGLGL"];

function encodeEan13(code: string): string {
  const d = code.split("").map(Number);
  const parity = PARITY[d[0]];
  let out = "101";
  for (let i = 0; i < 6; i++) out += parity[i] === "L" ? L[d[i + 1]] : G[d[i + 1]];
  out += "01010";
  for (let i = 7; i < 13; i++) out += R[d[i]];
  return out + "101";
}

/** Renders the code into an RGBA frame, placed as requested inside the picture. */
function frameWithBarcode(
  code: string,
  opts: { moduleWidth: number; frameWidth: number; frameHeight: number; centred?: boolean; left?: number }
): { rgba: Uint8ClampedArray; width: number; height: number } {
  const { moduleWidth, frameWidth, frameHeight, centred = true } = opts;
  const pattern = encodeEan13(code);

  const rgba = new Uint8ClampedArray(frameWidth * frameHeight * 4).fill(255);

  const barsWidth = pattern.length * moduleWidth;
  const barsHeight = Math.round(frameHeight * 0.3);
  // Off-centre still keeps a quiet zone: EAN-13 needs clear space either side
  // or the decoder cannot find where the code starts.
  const x0 = centred ? Math.floor((frameWidth - barsWidth) / 2) : opts.left ?? 80;
  const y0 = Math.floor((frameHeight - barsHeight) / 2);

  for (let m = 0; m < pattern.length; m++) {
    if (pattern[m] !== "1") continue;
    for (let x = x0 + m * moduleWidth; x < x0 + (m + 1) * moduleWidth; x++) {
      for (let y = y0; y < y0 + barsHeight; y++) {
        const i = (y * frameWidth + x) * 4;
        rgba[i] = rgba[i + 1] = rgba[i + 2] = 0;
      }
    }
  }

  return { rgba, width: frameWidth, height: frameHeight };
}

const NUTELLA = "3017620422003";

describe("toLuminance", () => {
  test("white stays white and black stays black", () => {
    const rgba = new Uint8ClampedArray([255, 255, 255, 255, 0, 0, 0, 255]);
    const lum = toLuminance(rgba, 2, 1);
    expect(lum[0]).toBe(255);
    expect(lum[1]).toBe(0);
  });

  test("produces one value per pixel", () => {
    const rgba = new Uint8ClampedArray(10 * 4).fill(128);
    expect(toLuminance(rgba, 5, 2)).toHaveLength(10);
  });
});

describe("centreCrop", () => {
  test("returns the middle band at the requested proportions", () => {
    const lum = new Uint8ClampedArray(100 * 100).fill(255);
    const crop = centreCrop(lum, 100, 100, 0.5, 0.4);
    expect(crop.width).toBe(50);
    expect(crop.height).toBe(40);
    expect(crop.data).toHaveLength(50 * 40);
  });

  test("keeps the pixels that were actually in the middle", () => {
    const lum = new Uint8ClampedArray(10 * 10).fill(255);
    lum[5 * 10 + 5] = 0; // one dark pixel dead centre
    const crop = centreCrop(lum, 10, 10, 0.6, 0.6);
    expect(Array.from(crop.data)).toContain(0);
  });

  test("never returns an empty region", () => {
    const lum = new Uint8ClampedArray(4).fill(255);
    const crop = centreCrop(lum, 2, 2, 0.01, 0.01);
    expect(crop.width).toBeGreaterThan(0);
    expect(crop.height).toBeGreaterThan(0);
  });
});

describe("decodeLuminance", () => {
  test("reads a rendered EAN-13", () => {
    const { rgba, width, height } = frameWithBarcode(NUTELLA, {
      moduleWidth: 4,
      frameWidth: 95 * 4 + 200,
      frameHeight: 200,
    });
    const lum = toLuminance(rgba, width, height);
    expect(decodeLuminance(lum, width, height)).toBe(NUTELLA);
  });

  test("returns null on a blank frame rather than throwing", () => {
    const lum = new Uint8ClampedArray(200 * 200).fill(255);
    expect(decodeLuminance(lum, 200, 200)).toBeNull();
  });

  test("returns null on a degenerate size rather than throwing", () => {
    expect(decodeLuminance(new Uint8ClampedArray(0), 0, 0)).toBeNull();
  });

  // State left over from one frame must not corrupt the next.
  test("repeated calls stay consistent", () => {
    const { rgba, width, height } = frameWithBarcode(NUTELLA, {
      moduleWidth: 4,
      frameWidth: 95 * 4 + 200,
      frameHeight: 200,
    });
    const lum = toLuminance(rgba, width, height);
    const blank = new Uint8ClampedArray(width * height).fill(255);

    for (let i = 0; i < 5; i++) {
      expect(decodeLuminance(blank, width, height)).toBeNull();
      expect(decodeLuminance(lum, width, height)).toBe(NUTELLA);
    }
  });
});

describe("decodeFrame", () => {
  test("finds a centred code", () => {
    const { rgba, width, height } = frameWithBarcode(NUTELLA, {
      moduleWidth: 3,
      frameWidth: 1280,
      frameHeight: 720,
    });
    expect(decodeFrame(rgba, width, height)).toBe(NUTELLA);
  });

  // The crop is an optimisation, not a restriction: a code off to one side must
  // still be found by the full-frame pass.
  test("finds a code that sits off to the side", () => {
    const { rgba, width, height } = frameWithBarcode(NUTELLA, {
      moduleWidth: 3,
      frameWidth: 1280,
      frameHeight: 720,
      centred: false,
    });
    expect(decodeFrame(rgba, width, height)).toBe(NUTELLA);
  });

  test("returns null when there is no code", () => {
    const rgba = new Uint8ClampedArray(640 * 480 * 4).fill(255);
    expect(decodeFrame(rgba, 640, 480)).toBeNull();
  });

  // Worth pinning because it is the practical advice given on the page: a code
  // pressed against the edge of the picture loses its quiet zone and cannot be
  // read, however sharp the rest of the frame is.
  test("a code flush against the frame edge cannot be read", () => {
    const { rgba, width, height } = frameWithBarcode(NUTELLA, {
      moduleWidth: 3,
      frameWidth: 1280,
      frameHeight: 720,
      centred: false,
      left: 2,
    });
    expect(decodeFrame(rgba, width, height)).toBeNull();
  });
});
