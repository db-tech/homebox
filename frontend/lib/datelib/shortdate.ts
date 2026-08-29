/**
 * Parsing of the short date forms printed on food packaging.
 *
 * A best-before date on a tin usually reads "03.2027" or "MHD 03/27", and on
 * fresh goods "12.03.2027". Typing that into a date picker on a phone while
 * holding the tin is slow, so this accepts the compact forms directly:
 *
 *   0327        -> 31.03.2027   (MMYY)
 *   032027      -> 31.03.2027   (MMYYYY)
 *   03.27       -> 31.03.2027
 *   03/2027     -> 31.03.2027
 *   12.03.2027  -> 12.03.2027
 *   12.03.27    -> 12.03.2027
 *   12032027    -> 12.03.2027   (DDMMYYYY)
 *   2027-03-12  -> 12.03.2027   (ISO)
 *
 * Month-only input resolves to the LAST day of that month, because "mindestens
 * haltbar bis Ende März" is what the packaging means.
 */

/** Two digit years are read as 20xx; food does not carry 19xx best-before dates. */
function expandYear(year: number): number {
  return year < 100 ? 2000 + year : year;
}

function lastDayOfMonth(year: number, month: number): Date {
  // Day 0 of the next month is the last day of this one.
  return new Date(year, month, 0);
}

function isValidYmd(year: number, month: number, day: number): boolean {
  if (month < 1 || month > 12) return false;
  if (day < 1) return false;
  return day <= lastDayOfMonth(year, month).getDate();
}

/**
 * Parses a compact date. Returns null when the input is not a date we
 * recognise, so the caller can leave the field alone rather than guess.
 */
export function parseShortDate(input: string): Date | null {
  const raw = input.trim();
  if (!raw) return null;

  // ISO first: unambiguous, and what a native date input produces.
  const iso = raw.match(/^(\d{4})-(\d{1,2})-(\d{1,2})$/);
  if (iso) {
    const [, y, m, d] = iso.map(Number);
    return isValidYmd(y, m, d) ? new Date(y, m - 1, d) : null;
  }

  // Separated forms: dd.mm.yyyy / mm.yyyy and the / and - variants.
  const parts = raw.split(/[./-]/).filter(p => p !== "");
  if (parts.length >= 2 && parts.every(p => /^\d+$/.test(p))) {
    if (parts.length === 2) {
      const month = Number(parts[0]);
      const year = expandYear(Number(parts[1]));
      return isValidYmd(year, month, 1) ? lastDayOfMonth(year, month) : null;
    }
    if (parts.length === 3) {
      const day = Number(parts[0]);
      const month = Number(parts[1]);
      const year = expandYear(Number(parts[2]));
      return isValidYmd(year, month, day) ? new Date(year, month - 1, day) : null;
    }
    return null;
  }

  // Bare digits.
  if (!/^\d+$/.test(raw)) return null;

  switch (raw.length) {
    case 4: {
      // MMYY
      const month = Number(raw.slice(0, 2));
      const year = expandYear(Number(raw.slice(2)));
      return isValidYmd(year, month, 1) ? lastDayOfMonth(year, month) : null;
    }
    case 6: {
      // MMYYYY
      const month = Number(raw.slice(0, 2));
      const year = Number(raw.slice(2));
      return isValidYmd(year, month, 1) ? lastDayOfMonth(year, month) : null;
    }
    case 8: {
      // DDMMYYYY
      const day = Number(raw.slice(0, 2));
      const month = Number(raw.slice(2, 4));
      const year = Number(raw.slice(4));
      return isValidYmd(year, month, day) ? new Date(year, month - 1, day) : null;
    }
    default:
      return null;
  }
}

/** Formats a parsed date back for display, e.g. "31.03.2027". */
export function formatShortDate(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(date.getDate())}.${pad(date.getMonth() + 1)}.${date.getFullYear()}`;
}

/**
 * The end of the month this many months from today, for the quick buttons.
 *
 * Deliberately the month end rather than the same day-of-month: setMonth on the
 * 29th of a month rolls over into the next month whenever the target February
 * is short, which silently produces a date a few days later than intended. The
 * month end is also what a best-before date means anyway.
 */
export function monthsFromNow(months: number): Date {
  const now = new Date();
  return lastDayOfMonth(now.getFullYear(), now.getMonth() + 1 + months);
}
