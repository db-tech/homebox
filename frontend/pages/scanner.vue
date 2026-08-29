<script setup lang="ts">
  import { BrowserMultiFormatReader, NotFoundException } from "@zxing/library";
  import { useI18n } from "vue-i18n";
  import type { ItemSummary, LocationOut, ProductlookupProduct } from "~~/lib/api/types/data-contracts";
  import type { ConsumptionType } from "~~/lib/api/classes/pantry";
  import MdiMinus from "~icons/mdi/minus";
  import MdiPlus from "~icons/mdi/plus";
  import MdiOpenInNew from "~icons/mdi/open-in-new";

  definePageMeta({
    middleware: ["auth"],
  });
  useHead({
    title: "Homebox | Scanner",
  });

  const { t } = useI18n();
  const api = useUserApi();
  const toast = useNotifier();

  const sources = ref<MediaDeviceInfo[]>([]);
  const selectedSource = ref<string | null>(null);
  const busy = ref(false);
  const video = ref<HTMLVideoElement>();
  const codeReader = new BrowserMultiFormatReader();
  const errorMessage = ref<string | null>(null);

  /** What to do automatically once a barcode resolves to exactly one item. */
  type ScanMode = "ask" | "consume" | "restock";
  const scanMode = ref<ScanMode>("restock");

  const scannedCode = ref<string | null>(null);
  const matches = ref<ItemSummary[]>([]);
  const suggestion = ref<ProductlookupProduct | null>(null);
  const searching = ref(false);

  // Unpacking a box means everything lands in the same place, so the location
  // is picked once and then stays put for the whole session.
  const location = ref<LocationOut | null>(null);
  const newName = ref("");
  const nameInput = ref<HTMLInputElement | null>(null);
  const creating = ref(false);

  const showNewItemForm = computed(() => !!scannedCode.value && !searching.value && matches.value.length === 0);

  const handleError = (error: unknown) => {
    console.error("Scanner error:", error);
    errorMessage.value = t("scanner.error");
  };

  /**
   * A Homebox QR code carries a URL back into this app. Anything else - a plain
   * EAN or UPC off a product - is treated as a barcode to look up.
   */
  function asInternalPath(text: string): string | null {
    let url: URL;
    try {
      url = new URL(text);
    } catch {
      return null;
    }
    if (!url.pathname.startsWith("/")) {
      return null;
    }
    return url.pathname.replace(/[^a-zA-Z0-9-_/]/g, "");
  }

  async function lookupBarcode(code: string) {
    scannedCode.value = code;
    matches.value = [];
    suggestion.value = null;
    newName.value = "";
    searching.value = true;

    const { data, error } = await api.pantry.scan(code);
    searching.value = false;

    if (error || !data) {
      handleError(error);
      return;
    }

    matches.value = data.items ?? [];
    suggestion.value = data.suggestion ?? null;

    if (matches.value.length === 1 && scanMode.value !== "ask") {
      await record(matches.value[0], scanMode.value);
      return;
    }

    if (matches.value.length === 0) {
      // Straight into typing: the suggestion is a starting point, not a verdict.
      newName.value = buildSuggestedName(data.suggestion);
      await nextTick();
      nameInput.value?.focus();
    }
  }

  /** Combines brand and product name the way a shelf label would read. */
  function buildSuggestedName(p?: ProductlookupProduct | null): string {
    if (!p?.found) return "";
    return [p.brand, p.name].filter(Boolean).join(" ").trim();
  }

  async function record(item: ItemSummary, kind: ConsumptionType) {
    if (kind === "consume" && item.quantity < 1) {
      toast.error(t("pantry.scan.out_of_stock", { name: item.name }));
      return;
    }

    const { response, error } = await api.pantry.record(item.id, {
      amount: 1,
      type: kind,
      note: "",
      date: new Date(),
    });

    if (error) {
      toast.error(
        response.status === 409
          ? t("pantry.scan.out_of_stock", { name: item.name })
          : t("pantry.toast.failed_to_record")
      );
      return;
    }

    toast.success(
      kind === "consume"
        ? t("pantry.scan.took_one", { name: item.name })
        : t("pantry.scan.added_one", { name: item.name })
    );

    item.quantity += kind === "consume" ? -1 : 1;
  }

  /**
   * Creates the item and goes straight back to scanning. Deliberately does not
   * navigate to the new item: the point is to keep the camera on the next can.
   */
  async function createFromScan() {
    if (!scannedCode.value || creating.value) {
      return;
    }
    if (!location.value) {
      toast.error(t("pantry.scan.pick_location_first"));
      return;
    }
    if (!newName.value.trim()) {
      return;
    }

    creating.value = true;
    const { data, error } = await api.items.create({
      parentId: null,
      name: newName.value.trim(),
      description: "",
      locationId: location.value.id,
      labelIds: [],
      barcode: scannedCode.value,
    });
    creating.value = false;

    if (error || !data) {
      toast.error(t("pantry.scan.create_failed"));
      return;
    }

    toast.success(t("pantry.scan.created", { name: data.name }));
    dismiss();
  }

  function dismiss() {
    scannedCode.value = null;
    matches.value = [];
    suggestion.value = null;
    newName.value = "";
    busy.value = false;
  }

  async function handleScan(text: string) {
    const path = asInternalPath(text);
    if (path) {
      navigateTo(path);
      return;
    }
    await lookupBarcode(text);
  }

  onMounted(async () => {
    if (!(navigator && navigator.mediaDevices && "enumerateDevices" in navigator.mediaDevices)) {
      errorMessage.value = t("scanner.unsupported");
      return;
    }

    try {
      const devices = await codeReader.listVideoInputDevices();
      sources.value = devices;

      if (devices.length > 0) {
        for (let i = 0; i < devices.length; i++) {
          if (devices[i].label.toLowerCase().includes("back")) {
            selectedSource.value = devices[i].deviceId;
          }
        }
        if (!selectedSource.value) {
          selectedSource.value = devices[0].deviceId;
        }
      } else {
        errorMessage.value = t("scanner.no_sources");
      }
    } catch (err) {
      handleError(err);
    }
  });

  onBeforeUnmount(() => codeReader.reset());

  watch(selectedSource, async newSource => {
    codeReader.reset();

    try {
      await codeReader.decodeFromVideoDevice(newSource, video.value!, (result, err) => {
        // While a result is on screen the camera keeps running but is ignored,
        // so typing a name is not interrupted by the next frame.
        if (result && !busy.value && !scannedCode.value) {
          busy.value = true;
          errorMessage.value = null;

          handleScan(result.getText())
            .catch(handleError)
            .finally(() => {
              setTimeout(() => {
                busy.value = false;
              }, 800);
            });
        }
        if (err && !(err instanceof NotFoundException)) {
          console.error(err);
          handleError(err);
        }
      });
    } catch (err) {
      handleError(err);
    }
  });
</script>

<template>
  <div class="flex flex-col gap-12 pb-16">
    <section>
      <div class="mx-auto">
        <div class="max-w-screen-md">
          <div v-if="errorMessage" role="alert" class="alert alert-error mb-5 shadow-lg">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="size-6 shrink-0 stroke-current"
              fill="none"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span class="text-sm">{{ errorMessage }}</span>
          </div>

          <!-- Set once, stays for the whole session -->
          <div class="mb-4">
            <LocationSelector v-model="location" />
            <p v-if="!location" class="text-xs text-warning">{{ $t("pantry.scan.pick_location_first") }}</p>
          </div>

          <video ref="video" class="rounded-box shadow-lg" poster="data:image/gif,AAAA"></video>

          <select v-model="selectedSource" class="select mt-4 w-full shadow-lg">
            <option disabled selected :value="null">{{ t("scanner.select_video_source") }}</option>
            <option v-for="source in sources" :key="source.deviceId" :value="source.deviceId">
              {{ source.label }}
            </option>
          </select>

          <div class="mt-4 flex flex-wrap items-center gap-2">
            <span class="text-sm">{{ $t("pantry.scan.mode") }}:</span>
            <button
              class="btn btn-xs"
              :class="scanMode === 'restock' ? 'btn-primary' : 'btn-ghost'"
              @click="scanMode = 'restock'"
            >
              {{ $t("pantry.scan.mode_restock") }}
            </button>
            <button
              class="btn btn-xs"
              :class="scanMode === 'consume' ? 'btn-primary' : 'btn-ghost'"
              @click="scanMode = 'consume'"
            >
              {{ $t("pantry.scan.mode_consume") }}
            </button>
            <button
              class="btn btn-xs"
              :class="scanMode === 'ask' ? 'btn-primary' : 'btn-ghost'"
              @click="scanMode = 'ask'"
            >
              {{ $t("pantry.scan.mode_ask") }}
            </button>
          </div>

          <p class="mt-2 text-xs">{{ $t("pantry.scan.hint") }}</p>

          <BaseCard v-if="scannedCode" class="mt-6">
            <template #title>{{ $t("pantry.scan.title") }}</template>
            <template #subtitle>{{ scannedCode }}</template>

            <div class="border-t border-gray-300 p-4">
              <p v-if="searching" class="text-sm">{{ $t("pantry.scan.searching", { code: scannedCode }) }}</p>

              <!-- Unknown code: name it and carry on -->
              <form v-else-if="showNewItemForm" class="flex flex-col gap-3" @submit.prevent="createFromScan">
                <p v-if="suggestion?.found" class="text-xs">
                  {{ $t("pantry.scan.suggested_by_openfoodfacts") }}
                  <span v-if="suggestion.amount"> &middot; {{ suggestion.amount }}</span>
                </p>
                <p v-else class="text-sm">{{ $t("pantry.scan.no_match", { code: scannedCode }) }}</p>

                <input
                  ref="nameInput"
                  v-model="newName"
                  type="text"
                  class="input input-bordered w-full"
                  :placeholder="$t('pantry.scan.name_placeholder')"
                  maxlength="255"
                />

                <div class="flex flex-wrap justify-end gap-2">
                  <BaseButton type="button" class="btn-ghost" size="sm" @click="dismiss">
                    {{ $t("pantry.scan.skip") }}
                  </BaseButton>
                  <BaseButton type="submit" size="sm" :disabled="creating || !newName.trim() || !location">
                    {{ $t("pantry.scan.create_and_continue") }}
                  </BaseButton>
                </div>
              </form>

              <!-- Known code -->
              <div v-else class="flex flex-col gap-3">
                <p v-if="matches.length > 1" class="text-sm">
                  {{ $t("pantry.scan.several_matches", { n: matches.length }) }}
                </p>
                <div
                  v-for="item in matches"
                  :key="item.id"
                  class="flex flex-wrap items-center gap-2 rounded border border-gray-300 p-3"
                >
                  <div class="grow">
                    <p class="font-medium">{{ item.name }}</p>
                    <p class="text-xs">
                      {{ $t("pantry.low_stock.in_stock") }}: {{ item.quantity }}
                      <template v-if="item.location"> &middot; {{ item.location.name }}</template>
                    </p>
                  </div>
                  <BaseButton size="sm" :disabled="item.quantity < 1" @click="record(item, 'consume')">
                    <template #icon><MdiMinus /></template>
                    1
                  </BaseButton>
                  <BaseButton size="sm" @click="record(item, 'restock')">
                    <template #icon><MdiPlus /></template>
                    1
                  </BaseButton>
                  <NuxtLink class="btn btn-ghost btn-sm" :to="`/item/${item.id}`">
                    <MdiOpenInNew />
                  </NuxtLink>
                </div>

                <div class="flex justify-end">
                  <BaseButton size="sm" @click="dismiss">{{ $t("pantry.scan.continue_scanning") }}</BaseButton>
                </div>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
    </section>
  </div>
</template>

<style lang="css" scoped>
  video {
    width: 100%;
    object-fit: cover;
    margin-left: auto;
    margin-right: auto;
  }
</style>
