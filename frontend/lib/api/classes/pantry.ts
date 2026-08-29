import { BaseAPI, route } from "../base";
import type { ConsumptionCreate, ConsumptionEntry, ConsumptionSummary, ItemSummary } from "../types/data-contracts";

export type ConsumptionType = "consume" | "restock" | "correction";

export class PantryAPI extends BaseAPI {
  /** Items whose expiry date falls within the next `within` days, soonest first. */
  expiring(within?: number) {
    return this.http.get<ItemSummary[]>({
      url: route("/pantry/expiring", within ? { within: within.toString() } : {}),
    });
  }

  /** Items whose quantity has dropped to or below their configured minimum stock. */
  lowStock() {
    return this.http.get<ItemSummary[]>({ url: route("/pantry/low-stock") });
  }

  /** Items carrying the given barcode. May return more than one match. */
  byBarcode(barcode: string) {
    return this.http.get<ItemSummary[]>({ url: route("/pantry/barcode", { barcode }) });
  }

  /** Consumption totals per item over the last `days` days, most consumed first. */
  statistics(days?: number) {
    return this.http.get<ConsumptionSummary[]>({
      url: route("/pantry/consumption/statistics", days ? { days: days.toString() } : {}),
    });
  }

  /** The stock movement log of a single item, newest first. */
  log(itemId: string) {
    return this.http.get<ConsumptionEntry[]>({ url: route(`/items/${itemId}/consumption`) });
  }

  /**
   * Record a stock movement. `consume` lowers the item quantity, `restock`
   * raises it, `correction` only annotates the log.
   */
  record(itemId: string, data: ConsumptionCreate) {
    return this.http.post<ConsumptionCreate, ConsumptionEntry>({
      url: route(`/items/${itemId}/consumption`),
      body: data,
    });
  }

  deleteEntry(entryId: string) {
    return this.http.delete<void>({ url: route(`/consumption/${entryId}`) });
  }
}
