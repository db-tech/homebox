/**
 * Choosing which item a scan takes from.
 *
 * A barcode can sit on more than one item: the same tinned tomatoes bought
 * twice, or kept in two places. On the scanner page you are asked which one.
 * A terminal on the wall cannot ask - the whole point of it is that taking
 * something out costs one scan and nothing else - so it has to decide.
 *
 * The rule is the one you would follow standing at the shelf: take the one
 * that goes off first. Items without a best-before date come last, because a
 * date is information and the absence of one is not a reason to reach for it.
 */

import type { ItemSummary } from "~~/lib/api/types/data-contracts";

/**
 * Whether a best-before date is actually set.
 *
 * An item without one arrives as Go's zero time ("0001-01-01T00:00:00Z"),
 * which parses perfectly happily and would otherwise sort ahead of every real
 * date - the empty items would be emptied first.
 */
export function hasExpiry(value: Date | string | null | undefined): boolean {
  if (!value) {
    return false;
  }

  const at = new Date(value);
  return !Number.isNaN(at.getTime()) && at.getFullYear() > 1900;
}

export interface ConsumeChoice {
  /** The item to take from. */
  item: ItemSummary;
  /** How many other items also carried the code, so the screen can say so. */
  alternatives: number;
}

/**
 * Picks the item a scanned barcode should be booked against, or null when
 * every match is already at zero.
 *
 * Items at zero are skipped rather than refused: with two tins of the same
 * thing in the cupboard, one empty entry must not block the other.
 */
export function pickForConsume(items: ItemSummary[]): ConsumeChoice | null {
  const available = items.filter(item => item.quantity >= 1);
  if (available.length === 0) {
    return null;
  }

  const ordered = [...available].sort(byExpiryThenName);
  return { item: ordered[0], alternatives: ordered.length - 1 };
}

function byExpiryThenName(a: ItemSummary, b: ItemSummary): number {
  const left = hasExpiry(a.expiryDate) ? new Date(a.expiryDate).getTime() : null;
  const right = hasExpiry(b.expiryDate) ? new Date(b.expiryDate).getTime() : null;

  if (left !== null && right !== null && left !== right) {
    return left - right;
  }
  if (left !== null && right === null) {
    return -1;
  }
  if (left === null && right !== null) {
    return 1;
  }

  // Same date, or neither has one. Ordering by name keeps a repeated scan on
  // the same item instead of following whatever order the API happened to
  // return, which would spread one product's stock across two entries.
  return a.name.localeCompare(b.name) || a.id.localeCompare(b.id);
}
