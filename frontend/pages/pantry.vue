<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { ServerEvent, onServerEvent } from "~~/composables/use-server-events";
  import type { ConsumptionSummary, ItemSummary } from "~~/lib/api/types/data-contracts";
  import MdiAlertOutline from "~icons/mdi/alert-outline";
  import MdiTabletDashboard from "~icons/mdi/tablet-dashboard";
  import MdiCartOutline from "~icons/mdi/cart-outline";
  import MdiChartLine from "~icons/mdi/chart-line";
  import MdiContentCopy from "~icons/mdi/content-copy";

  definePageMeta({
    middleware: ["auth"],
  });
  useHead({
    title: "Homebox | Pantry",
  });

  const api = useUserApi();
  const toast = useNotifier();
  const { t } = useI18n();

  const windowOptions = [7, 14, 30, 90];
  const expiryWindow = ref(14);
  const statsPeriod = ref(30);

  const { data: expiring, refresh: refreshExpiring } = useAsyncData(
    async () => {
      const { data, error } = await api.pantry.expiring(expiryWindow.value);
      if (error) {
        toast.error(t("pantry.toast.failed_to_load"));
        return [] as ItemSummary[];
      }
      return data ?? [];
    },
    { watch: [expiryWindow] }
  );

  const { data: lowStock, refresh: refreshLowStock } = useAsyncData(async () => {
    const { data, error } = await api.pantry.lowStock();
    if (error) {
      toast.error(t("pantry.toast.failed_to_load"));
      return [] as ItemSummary[];
    }
    return data ?? [];
  });

  const { data: stats } = useAsyncData(
    async () => {
      const { data, error } = await api.pantry.statistics(statsPeriod.value);
      if (error) {
        toast.error(t("pantry.toast.failed_to_load"));
        return [] as ConsumptionSummary[];
      }
      return data ?? [];
    },
    { watch: [statsPeriod] }
  );

  /** Whole days from today until the item expires. Negative means already expired. */
  function daysUntil(date: Date | string): number {
    const target = new Date(date);
    const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate());
    const diff = startOfDay(target).getTime() - startOfDay(new Date()).getTime();
    return Math.round(diff / 86_400_000);
  }

  function expiryLabel(item: ItemSummary): string {
    const days = daysUntil(item.expiryDate);
    if (days < 0) return t("pantry.expiring.expired");
    if (days === 0) return t("pantry.expiring.expires_today");
    return t("pantry.expiring.days_left", { n: days });
  }

  /** Red once expired, amber inside the last week, neutral otherwise. */
  function expiryClass(item: ItemSummary): string {
    const days = daysUntil(item.expiryDate);
    if (days < 0) return "badge-error";
    if (days <= 7) return "badge-warning";
    return "badge-ghost";
  }

  function shortfall(item: ItemSummary): number {
    return Math.max(0, item.minStock - item.quantity + 1);
  }

  async function copyShoppingList() {
    const lines = (lowStock.value ?? []).map(i => `${shortfall(i)}x ${i.name}`);
    if (lines.length === 0) return;

    try {
      await navigator.clipboard.writeText(lines.join("\n"));
      toast.success(t("pantry.low_stock.copied"));
    } catch {
      toast.error(t("pantry.toast.failed_to_load"));
    }
  }

  // Stock movements recorded elsewhere (item page, scanner) change both lists.
  onServerEvent(ServerEvent.ItemMutation, () => {
    refreshExpiring();
    refreshLowStock();
  });
</script>

<template>
  <BaseContainer class="mb-6 flex flex-col gap-8">
    <BaseSectionHeader>
      {{ $t("pantry.title") }}
    </BaseSectionHeader>

    <!-- The terminal is a full-screen page with no way back into the app, so it
         needs a door somewhere. This is it. -->
    <div>
      <NuxtLink to="/kiosk" class="btn btn-ghost btn-sm gap-2">
        <MdiTabletDashboard class="size-5" />
        {{ $t("pantry.kiosk.open") }}
      </NuxtLink>
    </div>

    <!-- Expiring soon -->
    <BaseCard>
      <template #title>
        <MdiAlertOutline class="mr-2 size-6" />
        {{ $t("pantry.expiring.title") }}
      </template>
      <template #subtitle>{{ $t("pantry.expiring.subtitle") }}</template>

      <div class="flex flex-wrap items-center gap-2 px-6 pb-4">
        <span class="text-sm">{{ $t("pantry.expiring.window") }}:</span>
        <button
          v-for="opt in windowOptions"
          :key="opt"
          class="btn btn-xs"
          :class="expiryWindow === opt ? 'btn-primary' : 'btn-ghost'"
          @click="expiryWindow = opt"
        >
          {{ $t("pantry.expiring.days", { n: opt }) }}
        </button>
      </div>

      <div class="border-t border-gray-300">
        <p v-if="!expiring || expiring.length === 0" class="p-6 text-sm">
          {{ $t("pantry.expiring.empty") }}
        </p>
        <table v-else class="table w-full">
          <thead>
            <tr>
              <th>{{ $t("global.name") }}</th>
              <th>{{ $t("items.location") }}</th>
              <th>{{ $t("items.quantity") }}</th>
              <th>{{ $t("items.expiry_date") }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in expiring" :key="item.id">
              <td>
                <NuxtLink class="hover:underline" :to="`/item/${item.id}`">{{ item.name }}</NuxtLink>
              </td>
              <td>
                <NuxtLink v-if="item.location" class="hover:underline" :to="`/location/${item.location.id}`">
                  {{ item.location.name }}
                </NuxtLink>
              </td>
              <td>{{ item.quantity }}</td>
              <td>
                <span class="badge" :class="expiryClass(item)">{{ expiryLabel(item) }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </BaseCard>

    <!-- Below minimum stock -->
    <BaseCard>
      <template #title>
        <MdiCartOutline class="mr-2 size-6" />
        {{ $t("pantry.low_stock.title") }}
      </template>
      <template #subtitle>{{ $t("pantry.low_stock.subtitle") }}</template>

      <div v-if="lowStock && lowStock.length > 0" class="px-6 pb-4">
        <BaseButton size="sm" @click="copyShoppingList">
          <template #icon><MdiContentCopy /></template>
          {{ $t("pantry.low_stock.copy_list") }}
        </BaseButton>
      </div>

      <div class="border-t border-gray-300">
        <p v-if="!lowStock || lowStock.length === 0" class="p-6 text-sm">
          {{ $t("pantry.low_stock.empty") }}
        </p>
        <table v-else class="table w-full">
          <thead>
            <tr>
              <th>{{ $t("global.name") }}</th>
              <th>{{ $t("items.location") }}</th>
              <th>{{ $t("pantry.low_stock.in_stock") }}</th>
              <th>{{ $t("pantry.low_stock.minimum") }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in lowStock" :key="item.id">
              <td>
                <NuxtLink class="hover:underline" :to="`/item/${item.id}`">{{ item.name }}</NuxtLink>
              </td>
              <td>
                <NuxtLink v-if="item.location" class="hover:underline" :to="`/location/${item.location.id}`">
                  {{ item.location.name }}
                </NuxtLink>
              </td>
              <td>{{ item.quantity }}</td>
              <td>{{ item.minStock }}</td>
              <td>
                <span class="badge badge-warning">{{ $t("pantry.low_stock.shortfall", { n: shortfall(item) }) }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </BaseCard>

    <!-- Consumption statistics -->
    <BaseCard>
      <template #title>
        <MdiChartLine class="mr-2 size-6" />
        {{ $t("pantry.consumption.statistics") }}
      </template>

      <div class="flex flex-wrap items-center gap-2 px-6 pb-4">
        <button
          v-for="opt in windowOptions"
          :key="opt"
          class="btn btn-xs"
          :class="statsPeriod === opt ? 'btn-primary' : 'btn-ghost'"
          @click="statsPeriod = opt"
        >
          {{ $t("pantry.consumption.period", { n: opt }) }}
        </button>
      </div>

      <div class="border-t border-gray-300">
        <p v-if="!stats || stats.length === 0" class="p-6 text-sm">
          {{ $t("pantry.consumption.no_stats") }}
        </p>
        <table v-else class="table w-full">
          <thead>
            <tr>
              <th>{{ $t("global.name") }}</th>
              <th>{{ $t("pantry.consumption.total_consumed") }}</th>
              <th>{{ $t("pantry.consumption.total_restocked") }}</th>
              <th>{{ $t("pantry.consumption.average_per_week") }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in stats" :key="row.itemId">
              <td>
                <NuxtLink class="hover:underline" :to="`/item/${row.itemId}`">{{ row.itemName }}</NuxtLink>
              </td>
              <td>{{ row.totalConsumed }}</td>
              <td>{{ row.totalRestocked }}</td>
              <td>{{ row.averagePerWeek.toFixed(1) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </BaseCard>
  </BaseContainer>
</template>
