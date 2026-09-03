import { describe, expect, test } from "vitest";
import { findBatch, orderBatches, planFill, sameDay } from "./fill-target";
import type { ItemSummary } from "~~/lib/api/types/data-contracts";

function item(over: Partial<ItemSummary> & { id: string; name: string }): ItemSummary {
  return {
    quantity: 1,
    expiryDate: "0001-01-01T00:00:00Z",
    minStock: 0,
    barcode: "4001234567890",
    ...over,
  } as ItemSummary;
}

describe("sameDay", () => {
  test("the same day at different times is one batch", () => {
    expect(sameDay("2027-03-31T00:00:00Z", new Date(2027, 2, 31, 14, 30))).toBe(true);
  });

  test("different days are different batches", () => {
    expect(sameDay("2027-03-31T00:00:00Z", "2027-04-01T00:00:00Z")).toBe(false);
  });

  test("two items without a date are the same batch", () => {
    expect(sameDay("0001-01-01T00:00:00Z", null)).toBe(true);
  });

  test("one with a date and one without are not", () => {
    expect(sameDay("2027-03-31T00:00:00Z", null)).toBe(false);
    expect(sameDay(null, "2027-03-31T00:00:00Z")).toBe(false);
  });
});

describe("findBatch", () => {
  const march = item({ id: "a", name: "Tomaten", expiryDate: "2027-03-31T00:00:00Z" });
  const november = item({ id: "b", name: "Tomaten", expiryDate: "2027-11-30T00:00:00Z" });

  test("a date that already exists finds its batch", () => {
    expect(findBatch([march, november], new Date(2027, 10, 30))?.id).toBe("b");
  });

  test("a date nobody has yet finds nothing", () => {
    expect(findBatch([march, november], new Date(2028, 0, 15))).toBeNull();
  });

  test("no date at all does not match a dated batch", () => {
    expect(findBatch([march, november], null)).toBeNull();
  });

  test("no date matches an existing undated batch", () => {
    const undated = item({ id: "c", name: "Salz" });
    expect(findBatch([undated], null)?.id).toBe("c");
  });
});

describe("orderBatches", () => {
  test("soonest first, undated last", () => {
    const undated = item({ id: "c", name: "C" });
    const late = item({ id: "a", name: "A", expiryDate: "2029-01-31T00:00:00Z" });
    const soon = item({ id: "b", name: "B", expiryDate: "2026-11-30T00:00:00Z" });

    expect(orderBatches([undated, late, soon]).map(i => i.id)).toEqual(["b", "a", "c"]);
  });

  test("the input array is left alone", () => {
    const late = item({ id: "a", name: "A", expiryDate: "2029-01-31T00:00:00Z" });
    const soon = item({ id: "b", name: "B", expiryDate: "2026-11-30T00:00:00Z" });
    const input = [late, soon];

    orderBatches(input);
    expect(input[0].id).toBe("a");
  });
});

describe("planFill", () => {
  const march = item({ id: "a", name: "Tomaten", expiryDate: "2027-03-31T00:00:00Z" });

  test("an unknown barcode is a new item", () => {
    expect(planFill([], null)).toEqual({ kind: "create" });
  });

  test("a known product with nothing settled yet asks which batch", () => {
    const plan = planFill([march], null);
    expect(plan.kind).toBe("choose");
    expect(plan.kind === "choose" && plan.batches.map(i => i.id)).toEqual(["a"]);
  });

  test("the rest of the box goes straight into the settled batch", () => {
    expect(planFill([march], "a")).toEqual({ kind: "add", item: march });
  });

  test("a settled batch that is no longer among the matches falls back to asking", () => {
    // It was deleted, archived or moved out of the group meanwhile. Booking
    // into an item that is not there any more would fail silently.
    expect(planFill([march], "gone").kind).toBe("choose");
  });

  test("batches are offered soonest first", () => {
    const later = item({ id: "b", name: "Tomaten", expiryDate: "2028-01-31T00:00:00Z" });
    const plan = planFill([later, march], null);
    expect(plan.kind === "choose" && plan.batches.map(i => i.id)).toEqual(["a", "b"]);
  });
});
