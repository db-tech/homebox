import { describe, expect, test } from "vitest";
import { assembleDate, formatShortDate, monthsFromNow, parseShortDate, pickableYears } from "./shortdate";

/** Helper: assert a parsed date matches y/m/d without timezone noise. */
function expectDate(got: Date | null, year: number, month: number, day: number) {
  expect(got).not.toBeNull();
  expect([got!.getFullYear(), got!.getMonth() + 1, got!.getDate()]).toEqual([year, month, day]);
}

describe("parseShortDate", () => {
  test("MMYY resolves to the last day of that month", () => {
    expectDate(parseShortDate("0327"), 2027, 3, 31);
  });

  test("MMYYYY resolves to the last day of that month", () => {
    expectDate(parseShortDate("032027"), 2027, 3, 31);
  });

  test("DDMMYYYY is taken as an exact day", () => {
    expectDate(parseShortDate("12032027"), 2027, 3, 12);
  });

  test("separated month and year forms", () => {
    expectDate(parseShortDate("03.27"), 2027, 3, 31);
    expectDate(parseShortDate("03.2027"), 2027, 3, 31);
    expectDate(parseShortDate("03/2027"), 2027, 3, 31);
  });

  test("separated full dates", () => {
    expectDate(parseShortDate("12.03.2027"), 2027, 3, 12);
    expectDate(parseShortDate("12.03.27"), 2027, 3, 12);
    expectDate(parseShortDate("12/03/2027"), 2027, 3, 12);
  });

  test("ISO form", () => {
    expectDate(parseShortDate("2027-03-12"), 2027, 3, 12);
  });

  test("month ends are correct, including February in a leap year", () => {
    expectDate(parseShortDate("0227"), 2027, 2, 28);
    expectDate(parseShortDate("0228"), 2028, 2, 29);
    expectDate(parseShortDate("0427"), 2027, 4, 30);
    expectDate(parseShortDate("1227"), 2027, 12, 31);
  });

  test("surrounding whitespace is ignored", () => {
    expectDate(parseShortDate("  0327  "), 2027, 3, 31);
  });

  // Anything we cannot read must return null rather than a wrong date - a
  // silently wrong best-before date is worse than an empty field.
  test.each([
    ["", "empty"],
    ["   ", "blank"],
    ["abc", "letters"],
    ["1327", "month 13"],
    ["0027", "month 0"],
    ["32.03.2027", "day 32"],
    ["30.02.2027", "30 February"],
    ["3", "too short"],
    ["12345", "odd length"],
    ["1234567", "odd length"],
    ["123456789", "too long"],
    ["MHD 0327", "prefixed text"],
  ])("rejects %s (%s)", input => {
    expect(parseShortDate(input)).toBeNull();
  });

  // Fast typing on a phone produces stray separators. The intent is
  // unambiguous, so these are tolerated on purpose rather than rejected.
  test("tolerates doubled separators", () => {
    expectDate(parseShortDate("03..2027"), 2027, 3, 31);
    expectDate(parseShortDate("12..03.2027"), 2027, 3, 12);
  });

  test("a rejected input never becomes an accidental date", () => {
    // 30 February must not roll over into 1 or 2 March.
    expect(parseShortDate("30.02.2027")).toBeNull();
  });
});

describe("formatShortDate", () => {
  test("pads day and month", () => {
    expect(formatShortDate(new Date(2027, 2, 5))).toBe("05.03.2027");
    expect(formatShortDate(new Date(2027, 11, 31))).toBe("31.12.2027");
  });

  test("round trips with parseShortDate", () => {
    const parsed = parseShortDate("0327")!;
    expect(formatShortDate(parsed)).toBe("31.03.2027");
    expectDate(parseShortDate(formatShortDate(parsed)), 2027, 3, 31);
  });
});

describe("monthsFromNow", () => {
  test("lands on the end of the target month", () => {
    const now = new Date();
    const got = monthsFromNow(6);
    const expected = new Date(now.getFullYear(), now.getMonth() + 7, 0);
    expectDate(got, expected.getFullYear(), expected.getMonth() + 1, expected.getDate());
  });

  test("crosses a year boundary", () => {
    const now = new Date();
    const got = monthsFromNow(24);
    expect(got.getFullYear()).toBe(now.getFullYear() + 2);
    expect(got.getMonth()).toBe(now.getMonth());
  });

  // Run from every day of a year: the result must always stay in the month we
  // asked for. The naive setMonth version overflows whenever the source day
  // does not exist in the target month.
  test("never overflows into the following month", () => {
    for (let offset = 0; offset < 400; offset++) {
      const got = monthsFromNow(offset % 37);
      expect(got.getDate()).toBe(new Date(got.getFullYear(), got.getMonth() + 1, 0).getDate());
    }
  });
});

describe("assembleDate", () => {
  test("builds the chosen day", () => {
    expectDate(assembleDate(2027, 3, 12), 2027, 3, 12);
  });

  test("a missing day means the end of the month", () => {
    expectDate(assembleDate(2027, 3, null), 2027, 3, 31);
    expectDate(assembleDate(2027, 2, null), 2027, 2, 28);
    expectDate(assembleDate(2028, 2, null), 2028, 2, 29);
  });

  // The picker asks for the day before the month, so an impossible pair has to
  // resolve to something sensible rather than roll into the next month.
  test("a day the month does not have is clamped, not rolled over", () => {
    expectDate(assembleDate(2027, 2, 31), 2027, 2, 28);
    expectDate(assembleDate(2028, 2, 30), 2028, 2, 29);
    expectDate(assembleDate(2027, 4, 31), 2027, 4, 30);
  });

  test("valid days are left alone", () => {
    expectDate(assembleDate(2027, 1, 31), 2027, 1, 31);
    expectDate(assembleDate(2028, 2, 29), 2028, 2, 29);
  });

  test("never returns a date outside the chosen month", () => {
    for (let m = 1; m <= 12; m++) {
      for (let d = 1; d <= 31; d++) {
        const got = assembleDate(2027, m, d);
        expect(got.getMonth() + 1).toBe(m);
        expect(got.getFullYear()).toBe(2027);
      }
    }
  });
});

describe("pickableYears", () => {
  test("starts at the given year and runs forward", () => {
    expect(pickableYears(5, new Date(2026, 7, 30))).toEqual([2026, 2027, 2028, 2029, 2030, 2031]);
  });

  test("always includes the current year first", () => {
    const years = pickableYears();
    expect(years[0]).toBe(new Date().getFullYear());
  });
});
