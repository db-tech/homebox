import { describe, expect, test } from "vitest";
import { hasExpiry, pickForConsume } from "./consume-target";
import type { ItemSummary } from "~~/lib/api/types/data-contracts";

/** Only the fields the picker looks at; the rest of ItemSummary is noise here. */
function item(over: Partial<ItemSummary> & { id: string; name: string }): ItemSummary {
  return {
    quantity: 1,
    expiryDate: "0001-01-01T00:00:00Z",
    minStock: 0,
    barcode: "4001234567890",
    ...over,
  } as ItemSummary;
}

describe("hasExpiry", () => {
  test("Go's zero time does not count as a date", () => {
    expect(hasExpiry("0001-01-01T00:00:00Z")).toBe(false);
  });

  test("empty and missing values do not count", () => {
    expect(hasExpiry("")).toBe(false);
    expect(hasExpiry(null)).toBe(false);
    expect(hasExpiry(undefined)).toBe(false);
  });

  test("unparseable values do not count", () => {
    expect(hasExpiry("not a date")).toBe(false);
  });

  test("a real date counts, as a string and as a Date", () => {
    expect(hasExpiry("2027-03-31T00:00:00Z")).toBe(true);
    expect(hasExpiry(new Date("2027-03-31"))).toBe(true);
  });
});

describe("pickForConsume", () => {
  test("nothing to pick from", () => {
    expect(pickForConsume([])).toBeNull();
  });

  test("a single match is taken as it is", () => {
    const only = item({ id: "a", name: "Tomaten" });
    expect(pickForConsume([only])).toEqual({ item: only, alternatives: 0 });
  });

  test("the earliest best-before date wins", () => {
    const late = item({ id: "a", name: "Tomaten Keller", expiryDate: "2028-01-31T00:00:00Z" });
    const soon = item({ id: "b", name: "Tomaten Kammer", expiryDate: "2026-11-30T00:00:00Z" });

    const choice = pickForConsume([late, soon]);
    expect(choice?.item.id).toBe("b");
    expect(choice?.alternatives).toBe(1);
  });

  test("order of the input does not matter", () => {
    const late = item({ id: "a", name: "A", expiryDate: "2028-01-31T00:00:00Z" });
    const soon = item({ id: "b", name: "B", expiryDate: "2026-11-30T00:00:00Z" });

    expect(pickForConsume([soon, late])?.item.id).toBe("b");
    expect(pickForConsume([late, soon])?.item.id).toBe("b");
  });

  test("an item with a date is preferred over one without", () => {
    const dated = item({ id: "a", name: "Zzz", expiryDate: "2029-12-31T00:00:00Z" });
    const undated = item({ id: "b", name: "Aaa" });

    expect(pickForConsume([undated, dated])?.item.id).toBe("a");
  });

  test("without any dates the choice is stable rather than arbitrary", () => {
    const b = item({ id: "2", name: "Bohnen" });
    const a = item({ id: "1", name: "Ananas" });

    expect(pickForConsume([b, a])?.item.id).toBe("1");
    expect(pickForConsume([a, b])?.item.id).toBe("1");
  });

  test("an empty entry is skipped in favour of one that still has stock", () => {
    const empty = item({ id: "a", name: "Tomaten", quantity: 0, expiryDate: "2026-01-31T00:00:00Z" });
    const stocked = item({ id: "b", name: "Tomaten", quantity: 4, expiryDate: "2027-06-30T00:00:00Z" });

    const choice = pickForConsume([empty, stocked]);
    expect(choice?.item.id).toBe("b");
    // The empty one is not an alternative - there is nothing in it to take.
    expect(choice?.alternatives).toBe(0);
  });

  test("everything at zero is reported as nothing to take", () => {
    const empty = item({ id: "a", name: "Tomaten", quantity: 0 });
    expect(pickForConsume([empty])).toBeNull();
  });

  test("the input array is left alone", () => {
    const late = item({ id: "a", name: "A", expiryDate: "2028-01-31T00:00:00Z" });
    const soon = item({ id: "b", name: "B", expiryDate: "2026-11-30T00:00:00Z" });
    const input = [late, soon];

    pickForConsume(input);
    expect(input[0].id).toBe("a");
  });
});
