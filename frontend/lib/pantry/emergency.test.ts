import { describe, expect, test } from "vitest";
import { categoryOrder, checklist, checklistIds, formatAmount } from "./emergency";

describe("checklist", () => {
  test("every id is unique", () => {
    const ids = checklist.flatMap(s => s.items.map(i => i.id));
    expect(new Set(ids).size).toBe(ids.length);
  });

  test("section ids are unique too", () => {
    const ids = checklist.map(s => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  test("every item has both languages", () => {
    for (const section of checklist) {
      expect(section.de.length, section.id).toBeGreaterThan(0);
      expect(section.en.length, section.id).toBeGreaterThan(0);

      for (const item of section.items) {
        expect(item.de.length, item.id).toBeGreaterThan(0);
        expect(item.en.length, item.id).toBeGreaterThan(0);
      }
    }
  });

  test("the id set covers every item", () => {
    const counted = checklist.reduce((n, s) => n + s.items.length, 0);
    expect(checklistIds.size).toBe(counted);
  });

  test("the sections the federal checklist prints are all present", () => {
    const ids = checklist.map(s => s.id);
    for (const expected of ["medicine", "hygiene", "light", "information", "gobag", "fire", "documents"]) {
      expect(ids).toContain(expected);
    }
  });
});

describe("categoryOrder", () => {
  test("covers all seven groups of the federal table, biggest target first", () => {
    expect(categoryOrder).toEqual(["drinks", "vegetables", "grains", "fruit", "dairy", "protein", "fats"]);
  });
});

describe("formatAmount", () => {
  test("drinks read as litres, however small", () => {
    expect(formatAmount(20000, "drinks")).toBe("20 l");
    expect(formatAmount(500, "drinks")).toBe("0.5 l");
  });

  test("food reads as kilograms once there is a kilo of it", () => {
    expect(formatAmount(4000, "vegetables")).toBe("4 kg");
    expect(formatAmount(3300, "grains")).toBe("3.3 kg");
  });

  test("and as grams below that, which is where the oil target lives", () => {
    expect(formatAmount(330, "fats")).toBe("330 g");
    expect(formatAmount(0, "fats")).toBe("0 g");
  });
});
