<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import type { ConsumptionEntry } from "~~/lib/api/types/data-contracts";
  import type { ConsumptionType } from "~~/lib/api/classes/pantry";
  import MdiDelete from "~icons/mdi/delete";
  import MdiMinus from "~icons/mdi/minus";
  import MdiPlus from "~icons/mdi/plus";

  const props = defineProps<{
    itemId: string;
    /** Current stock, used to stop the user consuming more than there is. */
    quantity: number;
  }>();

  const api = useUserApi();
  const toast = useNotifier();
  const confirm = useConfirm();
  const { t } = useI18n();

  const amount = ref(1);
  const note = ref("");
  const type = ref<ConsumptionType>("consume");
  const saving = ref(false);

  const { data: entries, refresh } = useAsyncData(async () => {
    const { data, error } = await api.pantry.log(props.itemId);
    if (error) {
      toast.error(t("pantry.toast.failed_to_load"));
      return [] as ConsumptionEntry[];
    }
    return data ?? [];
  });

  const canConsume = computed(() => props.quantity > 0);

  async function record(kind: ConsumptionType, qty: number = amount.value) {
    if (saving.value || qty < 1) {
      return;
    }
    saving.value = true;

    const { response, error } = await api.pantry.record(props.itemId, {
      amount: qty,
      type: kind,
      note: note.value,
      date: new Date(),
    });

    saving.value = false;

    if (error) {
      // 409 is the server saying the stock is not there, which is a normal
      // thing for a user to run into rather than a failure.
      toast.error(
        response.status === 409 ? t("pantry.consumption.insufficient_stock") : t("pantry.toast.failed_to_record")
      );
      return;
    }

    note.value = "";
    amount.value = 1;
    await refresh();
  }

  async function remove(entry: ConsumptionEntry) {
    const { isCanceled } = await confirm.open(t("pantry.consumption.delete_confirm"));
    if (isCanceled) {
      return;
    }

    const { error } = await api.pantry.deleteEntry(entry.id);
    if (error) {
      toast.error(t("pantry.toast.failed_to_delete"));
      return;
    }

    await refresh();
  }

  function typeLabel(t2: string): string {
    switch (t2) {
      case "consume":
        return t("pantry.consumption.consume");
      case "restock":
        return t("pantry.consumption.restock");
      default:
        return t("pantry.consumption.correction");
    }
  }

  function typeClass(t2: string): string {
    switch (t2) {
      case "consume":
        return "badge-warning";
      case "restock":
        return "badge-success";
      default:
        return "badge-ghost";
    }
  }
</script>

<template>
  <BaseCard>
    <template #title>{{ $t("pantry.consumption.log") }}</template>

    <div class="flex flex-wrap items-end gap-3 px-6 pb-4">
      <!-- One tap for the common case: took one out / put one back. -->
      <BaseButton size="sm" :disabled="!canConsume || saving" @click="record('consume', 1)">
        <template #icon><MdiMinus /></template>
        1
      </BaseButton>
      <BaseButton size="sm" :disabled="saving" @click="record('restock', 1)">
        <template #icon><MdiPlus /></template>
        1
      </BaseButton>

      <div class="w-24">
        <FormTextField v-model.number="amount" type="number" :label="$t('pantry.consumption.amount')" />
      </div>
      <div class="min-w-48 grow">
        <FormTextField v-model="note" :label="$t('pantry.consumption.note')" :max-length="500" />
      </div>
      <div class="w-40">
        <label class="label">
          <span class="label-text">{{ $t("pantry.consumption.title") }}</span>
        </label>
        <select v-model="type" class="select select-bordered select-sm w-full">
          <option value="consume">{{ $t("pantry.consumption.consume") }}</option>
          <option value="restock">{{ $t("pantry.consumption.restock") }}</option>
          <option value="correction">{{ $t("pantry.consumption.correction") }}</option>
        </select>
      </div>
      <BaseButton
        size="sm"
        :disabled="saving || amount < 1 || (type === 'consume' && amount > props.quantity)"
        @click="record(type)"
      >
        {{ $t("pantry.consumption.record") }}
      </BaseButton>
    </div>

    <div class="border-t border-gray-300">
      <p v-if="!entries || entries.length === 0" class="p-6 text-sm">
        {{ $t("pantry.consumption.empty") }}
      </p>
      <table v-else class="table w-full">
        <thead>
          <tr>
            <th>{{ $t("maintenance.modal.completed_date") }}</th>
            <th>{{ $t("pantry.consumption.title") }}</th>
            <th>{{ $t("pantry.consumption.amount") }}</th>
            <th>{{ $t("pantry.consumption.note") }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="entry in entries" :key="entry.id">
            <td>{{ fmtDate(entry.date, "relative") }}</td>
            <td>
              <span class="badge" :class="typeClass(entry.type)">{{ typeLabel(entry.type) }}</span>
            </td>
            <td>{{ entry.amount }}</td>
            <td>{{ entry.note }}</td>
            <td class="text-right">
              <button class="btn btn-square btn-ghost btn-sm" @click="remove(entry)">
                <MdiDelete />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </BaseCard>
</template>
