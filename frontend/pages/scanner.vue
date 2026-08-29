<script setup lang="ts">
  import { BrowserMultiFormatReader, NotFoundException } from "@zxing/library";
  import { useI18n } from "vue-i18n";
  import type { ItemSummary } from "~~/lib/api/types/data-contracts";
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
  const loading = ref(false);
  const video = ref<HTMLVideoElement>();
  const codeReader = new BrowserMultiFormatReader();
  const errorMessage = ref<string | null>(null);

  /** What to do automatically once a barcode resolves to exactly one item. */
  type ScanMode = "ask" | "consume" | "restock";
  const scanMode = ref<ScanMode>("ask");

  const scannedCode = ref<string | null>(null);
  const createModal = ref(false);
  const matches = ref<ItemSummary[]>([]);
  const searching = ref(false);

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
    searching.value = true;

    const { data, error } = await api.pantry.byBarcode(code);
    searching.value = false;

    if (error) {
      handleError(error);
      return;
    }

    matches.value = data ?? [];

    // With a single match and an automatic mode there is nothing to decide.
    if (matches.value.length === 1 && scanMode.value !== "ask") {
      await record(matches.value[0], scanMode.value);
    }
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

    // Keep the displayed stock honest without a full refetch.
    item.quantity += kind === "consume" ? -1 : 1;
  }

  async function handleScan(text: string) {
    const path = asInternalPath(text);
    if (path) {
      navigateTo(path);
      return;
    }

    await lookupBarcode(text);
  }

  function dismiss() {
    scannedCode.value = null;
    matches.value = [];
    loading.value = false;
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

  // stop the code reader when navigating away
  onBeforeUnmount(() => codeReader.reset());

  watch(selectedSource, async newSource => {
    codeReader.reset();

    try {
      await codeReader.decodeFromVideoDevice(newSource, video.value!, (result, err) => {
        if (result && !loading.value) {
          loading.value = true;
          errorMessage.value = null;

          handleScan(result.getText())
            .catch(handleError)
            .finally(() => {
              // Release the lock so the next product can be scanned, but only
              // after a beat so one barcode is not read several times over.
              setTimeout(() => {
                loading.value = false;
              }, 1200);
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
    <ItemCreateModal v-model="createModal" :barcode="scannedCode ?? ''" />
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
              :class="scanMode === 'ask' ? 'btn-primary' : 'btn-ghost'"
              @click="scanMode = 'ask'"
            >
              {{ $t("pantry.scan.mode_ask") }}
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
              :class="scanMode === 'restock' ? 'btn-primary' : 'btn-ghost'"
              @click="scanMode = 'restock'"
            >
              {{ $t("pantry.scan.mode_restock") }}
            </button>
          </div>

          <p class="mt-2 text-xs">{{ $t("pantry.scan.hint") }}</p>

          <!-- Scan result -->
          <BaseCard v-if="scannedCode" class="mt-6">
            <template #title>{{ $t("pantry.scan.title") }}</template>
            <template #subtitle>{{ scannedCode }}</template>

            <div class="border-t border-gray-300 p-4">
              <p v-if="searching" class="text-sm">{{ $t("pantry.scan.searching", { code: scannedCode }) }}</p>

              <div v-else-if="matches.length === 0" class="flex flex-col gap-3">
                <p class="text-sm">{{ $t("pantry.scan.no_match", { code: scannedCode }) }}</p>
                <div>
                  <BaseButton size="sm" @click="createModal = true">
                    {{ $t("pantry.scan.create_with_barcode") }}
                  </BaseButton>
                </div>
              </div>

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
              </div>

              <div class="mt-4 flex justify-end">
                <BaseButton size="sm" @click="dismiss">{{ $t("global.confirm") }}</BaseButton>
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
