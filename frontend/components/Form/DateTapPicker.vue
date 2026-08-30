<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { assembleDate, formatShortDate, parseShortDate, pickableYears } from "~~/lib/datelib/shortdate";

  /**
   * A date entered by tapping rather than typing.
   *
   * On a phone a text field brings up the keyboard, which covers half the
   * screen for the sake of four digits. Day, then month, then year, each a grid
   * of targets big enough to hit, is faster and leaves the page visible.
   *
   * Day comes first because that is the order the date is printed in. It can
   * also be skipped: packaging usually gives only a month, and "no day" means
   * the end of that month.
   */

  const props = defineProps<{
    modelValue: Date | null;
    /** Years offered, counting from this one. */
    yearsAhead?: number;
  }>();

  const emit = defineEmits<{ (e: "update:modelValue", value: Date | null): void }>();

  const { t } = useI18n();

  type Step = "day" | "month" | "year";

  const step = ref<Step>("day");
  const day = ref<number | null>(null);
  const month = ref<number | null>(null);

  /** Typing stays available for anyone who prefers it; it is just not the default. */
  const typing = ref(false);
  const typed = ref("");
  const typedDate = computed(() => parseShortDate(typed.value));
  const typedLooksWrong = computed(() => typed.value.trim() !== "" && typedDate.value === null);

  const days = Array.from({ length: 31 }, (_, i) => i + 1);
  const months = Array.from({ length: 12 }, (_, i) => i + 1);
  const years = computed(() => pickableYears(props.yearsAhead ?? 5));

  function reset() {
    step.value = "day";
    day.value = null;
    month.value = null;
  }

  function clear() {
    reset();
    typed.value = "";
    emit("update:modelValue", null);
  }

  function back() {
    if (step.value === "year") {
      step.value = "month";
      month.value = null;
      return;
    }
    if (step.value === "month") {
      step.value = "day";
      day.value = null;
    }
  }

  function pickDay(value: number | null) {
    day.value = value;
    step.value = "month";
  }

  function pickMonth(value: number) {
    month.value = value;
    step.value = "year";
  }

  function pickYear(value: number) {
    if (month.value === null) return;
    emit("update:modelValue", assembleDate(value, month.value, day.value));
    reset();
  }

  function applyTyped() {
    if (typedDate.value) {
      emit("update:modelValue", typedDate.value);
      typed.value = "";
      typing.value = false;
    }
  }

  /** What has been chosen so far, so the steps do not feel like a black box. */
  const progress = computed(() => {
    const d =
      day.value === null
        ? step.value === "day"
          ? "__"
          : t("components.form.date_picker.month_end_short")
        : String(day.value).padStart(2, "0");
    const m = month.value === null ? "__" : String(month.value).padStart(2, "0");
    return `${d}.${m}.____`;
  });
</script>

<template>
  <div>
    <!-- Already chosen: show it and offer a way back in -->
    <div v-if="props.modelValue" class="flex flex-wrap items-center gap-2">
      <span class="badge badge-lg">{{ formatShortDate(props.modelValue) }}</span>
      <button type="button" class="btn btn-ghost btn-xs" @click="clear">
        {{ $t("components.form.date_picker.change") }}
      </button>
    </div>

    <div v-else>
      <!-- Typed entry, for those who prefer it -->
      <div v-if="typing" class="flex flex-col gap-2">
        <div class="flex gap-2">
          <input
            v-model="typed"
            type="text"
            inputmode="numeric"
            class="input input-bordered input-sm grow"
            :class="typedLooksWrong ? 'input-error' : ''"
            :placeholder="$t('pantry.scan.expiry_placeholder')"
            @keyup.enter="applyTyped"
          />
          <BaseButton type="button" size="sm" :disabled="!typedDate" @click="applyTyped">
            {{ $t("components.form.date_picker.apply") }}
          </BaseButton>
        </div>
        <p v-if="typedLooksWrong" class="text-xs text-error">{{ $t("pantry.scan.expiry_unreadable") }}</p>
        <button type="button" class="self-start text-xs underline" @click="typing = false">
          {{ $t("components.form.date_picker.tap_instead") }}
        </button>
      </div>

      <div v-else>
        <div class="mb-2 flex items-center gap-2">
          <span class="text-sm font-medium">
            {{ $t(`components.form.date_picker.step_${step}`) }}
          </span>
          <span class="font-mono text-xs opacity-60">{{ progress }}</span>
          <div class="grow"></div>
          <button v-if="step !== 'day'" type="button" class="btn btn-ghost btn-xs" @click="back">
            {{ $t("components.form.date_picker.back") }}
          </button>
        </div>

        <!-- Day: seven columns mirrors a calendar week, so the numbers land
             where the eye expects them. -->
        <div v-if="step === 'day'">
          <div class="grid grid-cols-7 gap-1">
            <button v-for="d in days" :key="d" type="button" class="btn btn-sm px-0" @click="pickDay(d)">
              {{ d }}
            </button>
          </div>
          <button type="button" class="btn btn-outline btn-sm mt-2 w-full" @click="pickDay(null)">
            {{ $t("components.form.date_picker.month_end") }}
          </button>
        </div>

        <div v-else-if="step === 'month'" class="grid grid-cols-4 gap-1">
          <button v-for="m in months" :key="m" type="button" class="btn btn-sm px-0" @click="pickMonth(m)">
            {{ m }}
          </button>
        </div>

        <div v-else class="grid grid-cols-3 gap-1">
          <button v-for="y in years" :key="y" type="button" class="btn btn-sm px-0" @click="pickYear(y)">
            {{ y }}
          </button>
        </div>

        <button type="button" class="mt-2 text-xs underline" @click="typing = true">
          {{ $t("components.form.date_picker.type_instead") }}
        </button>
      </div>
    </div>
  </div>
</template>
