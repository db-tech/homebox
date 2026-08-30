import { describe, expect, test } from "vitest";
import { WedgeReader } from "../lib/barcode/wedge";

/**
 * Time is passed in rather than read from the clock, so these run
 * deterministically and can describe timings that would be impractical to
 * produce for real.
 */

/** Types a string at a fixed gap between characters, then presses Enter. */
function type(reader: WedgeReader, text: string, gapMs: number, startAt = 1000): string | null {
  let at = startAt;

  for (const ch of text) {
    // A character never completes a run; only Enter does.
    expect(reader.push(ch, at)).toBeNull();
    at += gapMs;
  }

  return reader.push("Enter", at);
}

describe("machine speed", () => {
  test("a fast run closed by Enter is a scan", () => {
    expect(type(new WedgeReader(), "4001234567890", 8)).toBe("4001234567890");
  });

  test("a slightly jittery Bluetooth scanner still counts", () => {
    const reader = new WedgeReader();
    let at = 1000;
    for (const ch of "4001234567890") {
      reader.push(ch, at);
      at += 40;
    }
    expect(reader.push("Enter", at)).toBe("4001234567890");
  });

  test("codes with letters are accepted", () => {
    expect(type(new WedgeReader(), "ABC-12345", 10)).toBe("ABC-12345");
  });
});

describe("human speed", () => {
  // The whole point: somebody typing must never be mistaken for a scanner.
  test("typing at a human pace is ignored", () => {
    expect(type(new WedgeReader(), "4001234567890", 180)).toBeNull();
  });

  test("a pause in the middle discards what came before", () => {
    const reader = new WedgeReader();
    reader.push("4", 1000);
    reader.push("0", 1008);
    // Long think, then the rest at machine speed.
    reader.push("1", 5000);
    reader.push("2", 5008);
    expect(reader.push("Enter", 5016)).toBeNull();
  });

  test("a pause immediately before Enter discards the run", () => {
    const reader = new WedgeReader();
    let at = 1000;
    for (const ch of "4001234567890") {
      reader.push(ch, at);
      at += 8;
    }
    // The user walked away and pressed Enter much later.
    expect(reader.push("Enter", at + 3000)).toBeNull();
  });
});

describe("guards", () => {
  test("a bare Enter commits nothing", () => {
    expect(new WedgeReader().push("Enter", 1000)).toBeNull();
  });

  test("a run shorter than the minimum is rejected", () => {
    expect(type(new WedgeReader(), "123", 8)).toBeNull();
  });

  test("the minimum length is configurable", () => {
    expect(type(new WedgeReader({ minLength: 2 }), "12", 8)).toBe("12");
  });

  test("modifier and navigation keys neither contribute nor break a run", () => {
    const reader = new WedgeReader();
    let at = 1000;
    for (const ch of "4001") {
      reader.push(ch, at);
      at += 8;
    }
    for (const key of ["Shift", "Control", "ArrowLeft", "F5"]) {
      expect(reader.push(key, at)).toBeNull();
      at += 8;
    }
    for (const ch of "234567890") {
      reader.push(ch, at);
      at += 8;
    }
    expect(reader.push("Enter", at)).toBe("4001234567890");
  });

  test("reset drops an in-flight run", () => {
    const reader = new WedgeReader();
    reader.push("4", 1000);
    reader.push("0", 1008);
    expect(reader.active).toBe(true);
    reader.reset();
    expect(reader.active).toBe(false);
    expect(reader.push("Enter", 1016)).toBeNull();
  });

  test("two scans in a row both come through", () => {
    const reader = new WedgeReader();
    expect(type(reader, "4001234567890", 8, 1000)).toBe("4001234567890");
    expect(type(reader, "8076809513692", 8, 9000)).toBe("8076809513692");
  });

  test("a scan straight after a discarded human run still works", () => {
    const reader = new WedgeReader();
    expect(type(reader, "slowly typed", 300)).toBeNull();
    expect(type(reader, "4001234567890", 8, 20000)).toBe("4001234567890");
  });
});
