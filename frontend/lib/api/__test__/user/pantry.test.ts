import { describe, test, expect } from "vitest";
import type { ItemOut, ItemUpdate, LocationOut } from "../../types/data-contracts";
import type { UserClient } from "../../user";
import { sharedUserClient } from "../test-utils";

describe("pantry endpoints", () => {
  let increment = 0;

  async function useLocation(api: UserClient): Promise<[LocationOut, () => Promise<void>]> {
    const { response, data } = await api.locations.create({
      parentId: null,
      name: `__test__.pantry.location_${increment}`,
      description: "pantry test location",
    });
    expect(response.status).toBe(201);
    increment++;

    return [
      data,
      async () => {
        await api.locations.delete(data.id);
      },
    ];
  }

  /** Creates an item and applies the pantry fields via the update endpoint. */
  async function usePantryItem(
    api: UserClient,
    location: LocationOut,
    overrides: Partial<ItemUpdate>
  ): Promise<ItemOut> {
    const { response, data: item } = await api.items.create({
      parentId: null,
      name: `__test__.pantry.item_${increment}`,
      labelIds: [],
      description: "pantry test item",
      locationId: location.id,
      barcode: "",
    });
    expect(response.status).toBe(201);
    increment++;

    const { response: updateResp, data: updated } = await api.items.update(item.id, {
      ...item,
      locationId: location.id,
      labelIds: [],
      ...overrides,
    } as ItemUpdate);
    expect(updateResp.status).toBe(200);

    return updated;
  }

  function daysFromNow(days: number): Date {
    const d = new Date();
    d.setDate(d.getDate() + days);
    return d;
  }

  test("expiring endpoint returns items inside the window and skips the rest", async () => {
    const api = await sharedUserClient();
    const [location, cleanup] = await useLocation(api);

    const soon = await usePantryItem(api, location, { expiryDate: daysFromNow(3), quantity: 1 });
    const later = await usePantryItem(api, location, { expiryDate: daysFromNow(120), quantity: 1 });
    const noDate = await usePantryItem(api, location, { quantity: 1 });

    const { response, data } = await api.pantry.expiring(14);
    expect(response.status).toBe(200);

    const ids = data.map(i => i.id);
    expect(ids).toContain(soon.id);
    expect(ids).not.toContain(later.id);
    expect(ids).not.toContain(noDate.id);

    await api.items.delete(soon.id);
    await api.items.delete(later.id);
    await api.items.delete(noDate.id);
    await cleanup();
  });

  test("low stock endpoint returns items at or below their minimum", async () => {
    const api = await sharedUserClient();
    const [location, cleanup] = await useLocation(api);

    const below = await usePantryItem(api, location, { quantity: 1, minStock: 3 });
    const above = await usePantryItem(api, location, { quantity: 9, minStock: 3 });
    const untracked = await usePantryItem(api, location, { quantity: 0, minStock: 0 });

    const { response, data } = await api.pantry.lowStock();
    expect(response.status).toBe(200);

    const ids = data.map(i => i.id);
    expect(ids).toContain(below.id);
    expect(ids).not.toContain(above.id);
    expect(ids).not.toContain(untracked.id);

    await api.items.delete(below.id);
    await api.items.delete(above.id);
    await api.items.delete(untracked.id);
    await cleanup();
  });

  test("barcode lookup finds the matching item", async () => {
    const api = await sharedUserClient();
    const [location, cleanup] = await useLocation(api);

    const code = `__test__${Date.now()}`;
    const item = await usePantryItem(api, location, { barcode: code, quantity: 2 });

    const { response, data } = await api.pantry.byBarcode(code);
    expect(response.status).toBe(200);
    expect(data.map(i => i.id)).toEqual([item.id]);

    const { data: none } = await api.pantry.byBarcode(`${code}-nope`);
    expect(none).toHaveLength(0);

    await api.items.delete(item.id);
    await cleanup();
  });

  test("recording consumption moves the stock and shows up in the log", async () => {
    const api = await sharedUserClient();
    const [location, cleanup] = await useLocation(api);

    const item = await usePantryItem(api, location, { quantity: 10 });

    const { response: consumeResp } = await api.pantry.record(item.id, {
      amount: 4,
      type: "consume",
      note: "dinner",
      date: new Date(),
    });
    expect(consumeResp.status).toBe(201);

    const { data: afterConsume } = await api.items.get(item.id);
    expect(afterConsume.quantity).toBe(6);

    const { response: restockResp } = await api.pantry.record(item.id, {
      amount: 2,
      type: "restock",
      note: "",
      date: new Date(),
    });
    expect(restockResp.status).toBe(201);

    const { data: afterRestock } = await api.items.get(item.id);
    expect(afterRestock.quantity).toBe(8);

    const { response: logResp, data: log } = await api.pantry.log(item.id);
    expect(logResp.status).toBe(200);
    expect(log).toHaveLength(2);

    await api.items.delete(item.id);
    await cleanup();
  });

  test("consuming more than is in stock is rejected and leaves the quantity alone", async () => {
    const api = await sharedUserClient();
    const [location, cleanup] = await useLocation(api);

    const item = await usePantryItem(api, location, { quantity: 2 });

    const { response } = await api.pantry.record(item.id, {
      amount: 5,
      type: "consume",
      note: "",
      date: new Date(),
    });
    // 409: the stock is not there. Not a server fault, so not a 500.
    expect(response.status).toBe(409);

    const { data: after } = await api.items.get(item.id);
    expect(after.quantity).toBe(2);

    await api.items.delete(item.id);
    await cleanup();
  });

  test("deleting a log entry keeps the stock as it is", async () => {
    const api = await sharedUserClient();
    const [location, cleanup] = await useLocation(api);

    const item = await usePantryItem(api, location, { quantity: 5 });

    const { data: entry } = await api.pantry.record(item.id, {
      amount: 2,
      type: "consume",
      note: "",
      date: new Date(),
    });

    const { response: delResp } = await api.pantry.deleteEntry(entry.id);
    expect(delResp.status).toBe(204);

    const { data: log } = await api.pantry.log(item.id);
    expect(log).toHaveLength(0);

    const { data: after } = await api.items.get(item.id);
    expect(after.quantity).toBe(3);

    await api.items.delete(item.id);
    await cleanup();
  });

  test("statistics aggregate consumption per item", async () => {
    const api = await sharedUserClient();
    const [location, cleanup] = await useLocation(api);

    const item = await usePantryItem(api, location, { quantity: 20 });

    await api.pantry.record(item.id, { amount: 3, type: "consume", note: "", date: new Date() });
    await api.pantry.record(item.id, { amount: 2, type: "consume", note: "", date: new Date() });
    await api.pantry.record(item.id, { amount: 6, type: "restock", note: "", date: new Date() });

    const { response, data } = await api.pantry.statistics(30);
    expect(response.status).toBe(200);

    const row = data.find(r => r.itemId === item.id);
    expect(row).toBeTruthy();
    expect(row!.totalConsumed).toBe(5);
    expect(row!.totalRestocked).toBe(6);

    await api.items.delete(item.id);
    await cleanup();
  });
});
