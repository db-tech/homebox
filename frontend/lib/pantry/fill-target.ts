/**
 * What a scan means while filling the pantry.
 *
 * The awkward case is one product with two best-before dates. An item carries a
 * single date, so six tins until March and two until November are two items -
 * merging them would throw one of the dates away, and the dates are the reason
 * the pantry view exists at all.
 *
 * That makes a scan ambiguous: it could be another tin of a batch already
 * there, or the first tin of a new one. Only the date settles it, so the date
 * is the one thing worth asking for - and only once per batch, because a box of
 * tins is a box of the same date.
 */

import { hasExpiry } from "./consume-target";
import type { ItemSummary } from "~~/lib/api/types/data-contracts";

/**
 * Whether two best-before dates mean the same batch.
 *
 * Compared by calendar day: the pickers and the API disagree about the time of
 * day often enough that comparing instants would split a batch in two for no
 * reason a person could see.
 */
export function sameDay(a: Date | string | null | undefined, b: Date | string | null | undefined): boolean {
  const left = hasExpiry(a) ? new Date(a as Date | string) : null;
  const right = hasExpiry(b) ? new Date(b as Date | string) : null;

  if (left === null || right === null) {
    // Two items that both lack a date are the same batch; one with and one
    // without are not.
    return left === right;
  }

  return (
    left.getFullYear() === right.getFullYear() &&
    left.getMonth() === right.getMonth() &&
    left.getDate() === right.getDate()
  );
}

/** The existing batch a date belongs to, or null when it is new to the product. */
export function findBatch(items: ItemSummary[], date: Date | null): ItemSummary | null {
  return items.find(candidate => sameDay(candidate.expiryDate, date)) ?? null;
}

/** Batches of a product in the order they should be offered: soonest first. */
export function orderBatches(items: ItemSummary[]): ItemSummary[] {
  return [...items].sort((a, b) => {
    const left = hasExpiry(a.expiryDate) ? new Date(a.expiryDate).getTime() : null;
    const right = hasExpiry(b.expiryDate) ? new Date(b.expiryDate).getTime() : null;

    if (left !== null && right !== null && left !== right) return left - right;
    if (left !== null && right === null) return -1;
    if (left === null && right !== null) return 1;
    return a.name.localeCompare(b.name) || a.id.localeCompare(b.id);
  });
}

export type FillPlan =
  /** Another one of a batch already settled during this session. */
  | { kind: "add"; item: ItemSummary }
  /** The product is known but which batch is not. */
  | { kind: "choose"; batches: ItemSummary[] }
  /** Nothing carries this barcode yet. */
  | { kind: "create" };

/**
 * Decides what to do with a scanned barcode.
 *
 * `settled` is the batch this barcode was booked into earlier in the same
 * session. It is what makes unpacking a box cost one scan per tin: the first
 * tin picks the date, the rest of the box needs nothing at all. It is
 * deliberately not remembered across sessions - the next box of the same
 * product is a new date, and silently adding it to last month's batch would
 * put a wrong best-before date on real food.
 */
export function planFill(items: ItemSummary[], settled: string | null): FillPlan {
  const known = items.find(candidate => candidate.id === settled);
  if (settled && known) {
    return { kind: "add", item: known };
  }

  if (items.length > 0) {
    return { kind: "choose", batches: orderBatches(items) };
  }

  return { kind: "create" };
}
