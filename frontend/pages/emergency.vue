<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { categoryOrder, checklist, formatAmount } from "~~/lib/pantry/emergency";
  import type { EmergencyCategory } from "~~/lib/pantry/emergency";
  import type { EmergencyResult } from "~~/lib/api/types/data-contracts";
  import MdiShieldHome from "~icons/mdi/shield-home";
  import MdiClipboardCheck from "~icons/mdi/clipboard-check-outline";

  /**
   * The pantry measured against the German federal stockpiling recommendation.
   *
   * The figures are the ones published by the Ernährungsvorsorge portal; the
   * non-food list is the BBK's checklist, quoted rather than rewritten. See
   * lib/pantry/emergency.ts for the sources.
   */

  definePageMeta({ middleware: ["auth"] });
  useHead({ title: "Homebox | Emergency stock" });

  const api = useUserApi();
  const toast = useNotifier();
  const { t, locale } = useI18n();

  const result = ref<EmergencyResult | null>(null);
  const people = ref(1);
  const days = ref(10);
  const ticked = ref<Set<string>>(new Set());
  const saving = ref(false);

  const dayOptions = [3, 5, 7, 10];

  async function load() {
    const { data, error } = await api.pantry.emergency();
    if (error || !data) {
      toast.error(t("pantry.emergency.failed"));
      return;
    }

    result.value = data;
    people.value = data.balance.people;
    days.value = data.balance.days;
    ticked.value = new Set(data.checklist ?? []);
  }

  await load();

  /**
   * Saves and reloads. The balance depends on the household figures, so showing
   * the old one next to the new settings would be a lie for as long as it took
   * somebody to notice.
   */
  async function save() {
    saving.value = true;
    const { error } = await api.pantry.setEmergency({
      people: people.value,
      days: days.value,
      checklist: [...ticked.value],
    });
    saving.value = false;

    if (error) {
      toast.error(t("pantry.emergency.save_failed"));
      return;
    }

    await load();
  }

  function toggle(id: string) {
    const next = new Set(ticked.value);
    if (next.has(id)) {
      next.delete(id);
    } else {
      next.add(id);
    }
    ticked.value = next;
    save().catch(() => {});
  }

  /** The checklist is quoted from a German publication; English is a reading aid. */
  function label(entry: { de: string; en: string }): string {
    return locale.value.startsWith("de") ? entry.de : entry.en;
  }

  const lines = computed(() => {
    const byCategory = new Map(result.value?.balance.lines.map(l => [l.category, l]) ?? []);
    return categoryOrder.map(category => ({ category, line: byCategory.get(category) }));
  });

  function tone(percent: number): string {
    if (percent >= 100) return "progress-success";
    if (percent >= 50) return "progress-warning";
    return "progress-error";
  }

  const sectionProgress = computed(() =>
    Object.fromEntries(checklist.map(section => [section.id, section.items.filter(i => ticked.value.has(i.id)).length]))
  );
</script>

<template>
  <BaseContainer class="mb-6 flex flex-col gap-8">
    <BaseSectionHeader>
      {{ $t("pantry.emergency.title") }}
    </BaseSectionHeader>

    <BaseCard>
      <template #title>
        <MdiShieldHome class="mr-2 size-6" />
        {{ $t("pantry.emergency.food_title") }}
      </template>
      <template #subtitle>{{ $t("pantry.emergency.food_subtitle") }}</template>

      <div class="flex flex-wrap items-end gap-4 px-6 pb-4">
        <div>
          <label class="label" for="people"
            ><span class="label-text">{{ $t("pantry.emergency.people") }}</span></label
          >
          <input
            id="people"
            v-model.number="people"
            type="number"
            min="1"
            max="50"
            class="input input-bordered w-24"
            @change="save"
          />
        </div>

        <div>
          <p class="label">
            <span class="label-text">{{ $t("pantry.emergency.days") }}</span>
          </p>
          <div class="join">
            <button
              v-for="option in dayOptions"
              :key="option"
              class="join-item btn btn-sm"
              :class="days === option ? 'btn-primary' : 'btn-ghost'"
              :disabled="saving"
              @click="
                days = option;
                save();
              "
            >
              {{ option }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="result" class="border-t border-gray-300 p-6">
        <div class="mb-4 flex items-baseline gap-3">
          <span class="text-4xl font-bold">{{ result.balance.percent }}%</span>
          <span class="text-sm text-base-content/60">{{ $t("pantry.emergency.overall") }}</span>
        </div>

        <div class="flex flex-col gap-3">
          <div v-for="row in lines" :key="row.category">
            <div class="flex flex-wrap items-baseline justify-between gap-2 text-sm">
              <span class="font-semibold">{{ $t(`pantry.emergency.category.${row.category}`) }}</span>
              <span class="tabular-nums">
                {{ formatAmount(row.line?.stockGrams ?? 0, row.category as EmergencyCategory) }} /
                {{ formatAmount(row.line?.targetGrams ?? 0, row.category as EmergencyCategory) }}
                <template v-if="(row.line?.missingGrams ?? 0) > 0">
                  &middot;
                  <span class="text-error">
                    {{ $t("pantry.emergency.missing") }}
                    {{ formatAmount(row.line?.missingGrams ?? 0, row.category as EmergencyCategory) }}
                  </span>
                </template>
              </span>
            </div>
            <progress
              class="progress w-full"
              :class="tone(row.line?.percent ?? 0)"
              :value="row.line?.percent ?? 0"
              max="100"
            />
          </div>
        </div>
      </div>
    </BaseCard>

    <!-- Items that cannot be counted. Saying which ones is the difference
         between a figure you can act on and one you have to distrust. -->
    <BaseCard v-if="result && (result.unweighed.length > 0 || result.uncategorised.length > 0)">
      <template #title>{{ $t("pantry.emergency.gaps_title") }}</template>
      <template #subtitle>{{ $t("pantry.emergency.gaps_subtitle") }}</template>

      <div class="flex flex-col gap-4 border-t border-gray-300 p-6">
        <div v-if="result.unweighed.length">
          <p class="mb-2 text-sm font-semibold">{{ $t("pantry.emergency.unweighed") }}</p>
          <div class="flex flex-wrap gap-2">
            <NuxtLink
              v-for="entry in result.unweighed"
              :key="entry.id"
              :to="`/item/${entry.id}/edit`"
              class="btn btn-outline btn-xs"
            >
              {{ entry.name }}
            </NuxtLink>
          </div>
        </div>

        <div v-if="result.uncategorised.length">
          <p class="mb-2 text-sm font-semibold">{{ $t("pantry.emergency.uncategorised") }}</p>
          <div class="flex flex-wrap gap-2">
            <NuxtLink
              v-for="entry in result.uncategorised"
              :key="entry.id"
              :to="`/item/${entry.id}/edit`"
              class="btn btn-outline btn-xs"
            >
              {{ entry.name }}
            </NuxtLink>
          </div>
        </div>
      </div>
    </BaseCard>

    <BaseCard>
      <template #title>
        <MdiClipboardCheck class="mr-2 size-6" />
        {{ $t("pantry.emergency.checklist_title") }}
      </template>
      <template #subtitle>{{ $t("pantry.emergency.checklist_subtitle") }}</template>

      <div class="flex flex-col gap-6 border-t border-gray-300 p-6">
        <section v-for="section in checklist" :key="section.id">
          <h3 class="mb-2 font-bold">
            {{ label(section) }}
            <span class="text-sm font-normal text-base-content/50">
              {{ sectionProgress[section.id] }} / {{ section.items.length }}
            </span>
          </h3>

          <ul class="flex flex-col gap-1">
            <li v-for="entry in section.items" :key="entry.id">
              <label class="flex cursor-pointer items-start gap-3">
                <input
                  type="checkbox"
                  class="checkbox checkbox-sm mt-0.5"
                  :checked="ticked.has(entry.id)"
                  @change="toggle(entry.id)"
                />
                <span class="text-sm" :class="ticked.has(entry.id) ? 'text-base-content/50 line-through' : ''">
                  {{ label(entry) }}
                </span>
              </label>
            </li>
          </ul>
        </section>

        <p class="text-xs text-base-content/50">{{ $t("pantry.emergency.source") }}</p>
      </div>
    </BaseCard>
  </BaseContainer>
</template>
