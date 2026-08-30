<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { decodeFrame } from "~~/lib/barcode/decode";
  import type { ItemSummary, LocationOut, ProductlookupProduct } from "~~/lib/api/types/data-contracts";
  import type { ConsumptionType } from "~~/lib/api/classes/pantry";
  import { formatShortDate, monthsFromNow, parseShortDate } from "~~/lib/datelib/shortdate";
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
  const errorMessage = ref<string | null>(null);

  /** Diagnostics, shown on the page so a failure can be read out rather than guessed at. */
  const captureSize = ref<string | null>(null);
  const engine = ref<"native" | "zxing" | null>(null);
  const framesTried = ref(0);
  const lastEngineError = ref<string | null>(null);
  const manualCode = ref("");

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

  // Best-before dates are the point of the whole pantry view, so they are
  // entered here rather than left for a second pass through the edit form.
  // The field takes the short forms printed on packaging: 0327, 03.27, 03/2027,
  // 12.03.2027 - see lib/datelib/shortdate.
  const newExpiry = ref("");
  const newMinStock = ref<number | null>(null);

  const parsedExpiry = computed(() => parseShortDate(newExpiry.value));
  const expiryLooksWrong = computed(() => newExpiry.value.trim() !== "" && parsedExpiry.value === null);

  function setExpiryMonths(months: number) {
    newExpiry.value = formatShortDate(monthsFromNow(months));
  }

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
    newExpiry.value = "";
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

    // Refuse rather than silently store a date we could not read - a wrong
    // best-before date is worse than none.
    if (expiryLooksWrong.value) {
      toast.error(t("pantry.scan.expiry_unreadable"));
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
      expiryDate: parsedExpiry.value ?? "",
      minStock: newMinStock.value ?? 0,
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
    newExpiry.value = "";
    // The minimum stock is deliberately kept: a whole box of tins usually wants
    // the same one, and re-typing it per item is the sort of friction this
    // screen exists to remove.
    busy.value = false;
  }

  /**
   * Typing the digits goes through exactly the same path as a camera read, so a
   * damaged or unreadable code never blocks the work.
   */
  async function submitManual() {
    const code = manualCode.value.trim();
    if (!code) return;
    manualCode.value = "";
    await handleScan(code);
  }

  async function handleScan(text: string) {
    const path = asInternalPath(text);
    if (path) {
      navigateTo(path);
      return;
    }
    await lookupBarcode(text);
  }

  // ---------------------------------------------------------------------------
  // Camera
  //
  // The stream is acquired and attached by hand rather than through zxing's
  // browser helpers. Those create their own hidden 200x200 video element when
  // they are handed anything but a live element, which silently guarantees an
  // unreadable picture, and they hide the capture size so a bad stream looks
  // like "nothing happens".
  // ---------------------------------------------------------------------------

  let stream: MediaStream | null = null;
  let loopHandle: number | null = null;
  let detector: { detect: (src: CanvasImageSource) => Promise<Array<{ rawValue: string }>> } | null = null;
  const canvas = document.createElement("canvas");

  function stopCamera() {
    if (loopHandle !== null) {
      window.clearTimeout(loopHandle);
      loopHandle = null;
    }
    stream?.getTracks().forEach(t => t.stop());
    stream = null;
    if (video.value) {
      video.value.srcObject = null;
    }
  }

  async function startCamera() {
    stopCamera();
    captureSize.value = null;
    framesTried.value = 0;

    if (!navigator?.mediaDevices?.getUserMedia) {
      errorMessage.value = t("scanner.unsupported");
      return;
    }

    // A barcode needs a few hundred pixels across to decode at all, so the
    // resolution is asked for explicitly. Without this the browser hands back
    // whatever it likes, commonly 640x480, which is not enough at arm's length.
    const constraints: MediaStreamConstraints = {
      video: {
        ...(selectedSource.value
          ? { deviceId: { exact: selectedSource.value } }
          : { facingMode: { ideal: "environment" } }),
        width: { ideal: 1920 },
        height: { ideal: 1080 },
      },
    };

    try {
      stream = await navigator.mediaDevices.getUserMedia(constraints);
    } catch (err) {
      handleError(err);
      return;
    }

    if (!video.value) {
      return;
    }

    video.value.srcObject = stream;
    video.value.setAttribute("playsinline", "true");
    video.value.muted = true;
    await video.value.play().catch(handleError);

    const settings = stream.getVideoTracks()[0]?.getSettings();
    captureSize.value = `${settings?.width ?? video.value.videoWidth}\u00D7${settings?.height ?? video.value.videoHeight}`;

    // Now that permission has been granted the device labels are populated, so
    // the picker can finally show something meaningful.
    try {
      sources.value = (await navigator.mediaDevices.enumerateDevices()).filter(d => d.kind === "videoinput");
    } catch {
      // A missing device list is not worth failing the scan over.
    }

    scanLoop();
  }

  async function scanLoop() {
    if (!stream || !video.value) {
      return;
    }

    const el = video.value;

    if (el.readyState >= 2 && el.videoWidth > 0 && !scannedCode.value && !busy.value) {
      framesTried.value++;
      try {
        let found: string | null = null;

        if (detector) {
          // The native detector is what a phone's own scanner app uses: it runs
          // outside JavaScript and copes with far smaller and blurrier codes.
          const hits = await detector.detect(el);
          found = hits[0]?.rawValue ?? null;
        } else {
          canvas.width = el.videoWidth;
          canvas.height = el.videoHeight;
          const ctx = canvas.getContext("2d", { willReadFrequently: true });
          if (ctx) {
            ctx.drawImage(el, 0, 0);
            const frame = ctx.getImageData(0, 0, canvas.width, canvas.height);
            found = decodeFrame(frame.data, canvas.width, canvas.height);
          }
        }

        if (found) {
          busy.value = true;
          errorMessage.value = null;
          await handleScan(found).catch(handleError);
          window.setTimeout(() => {
            busy.value = false;
          }, 800);
        }
      } catch (err) {
        // A single bad frame must not stop the loop.
        lastEngineError.value = err instanceof Error ? err.message : String(err);
      }
    }

    loopHandle = window.setTimeout(scanLoop, 120);
  }

  onMounted(async () => {
    // Prefer the browser's own detector where it exists. It is the same
    // machinery a native scanner app uses and is dramatically more forgiving
    // than decoding frames in JavaScript.
    const w = window as unknown as { BarcodeDetector?: new (o?: unknown) => typeof detector };
    if (w.BarcodeDetector) {
      try {
        detector = new w.BarcodeDetector({
          formats: ["ean_13", "ean_8", "upc_a", "upc_e", "code_128", "code_39", "itf", "qr_code"],
        }) as typeof detector;
        engine.value = "native";
      } catch {
        detector = null;
      }
    }
    if (!detector) {
      engine.value = "zxing";
    }

    await startCamera();
  });

  onBeforeUnmount(stopCamera);

  // Only restart when the user actively picks a different camera.
  watch(selectedSource, () => startCamera());
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
            <p v-if="!location" class="text-xs text-warning">
              {{ $t("pantry.scan.pick_location_first") }}
            </p>
          </div>

          <video ref="video" class="rounded-box shadow-lg" poster="data:image/gif,AAAA"></video>

          <!-- Visible diagnostics: if scanning fails, this can be read out
               instead of guessed at. -->
          <p class="mt-1 text-right text-xs opacity-60">
            <span v-if="engine">{{ $t("pantry.scan.engine_" + engine) }}</span>
            <span v-if="captureSize"> &middot; {{ captureSize }}</span>
            <span v-if="framesTried"> &middot; {{ $t("pantry.scan.frames", { n: framesTried }) }}</span>
          </p>
          <p v-if="lastEngineError" class="text-right text-xs text-error">{{ lastEngineError }}</p>

          <form class="mt-3 flex gap-2" @submit.prevent="submitManual">
            <input
              v-model="manualCode"
              type="text"
              inputmode="numeric"
              class="input input-bordered grow"
              :placeholder="$t('pantry.scan.manual_placeholder')"
            />
            <BaseButton type="submit" size="sm" :disabled="!manualCode.trim()">
              {{ $t("pantry.scan.manual_submit") }}
            </BaseButton>
          </form>

          <select v-model="selectedSource" class="select mt-4 w-full shadow-lg">
            <option disabled selected :value="null">
              {{ t("scanner.select_video_source") }}
            </option>
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

          <details class="mt-2 text-xs">
            <summary class="cursor-pointer">
              {{ $t("pantry.scan.tips_title") }}
            </summary>
            <p class="mt-1 opacity-80">{{ $t("pantry.scan.tips") }}</p>
          </details>

          <BaseCard v-if="scannedCode" class="mt-6">
            <template #title>{{ $t("pantry.scan.title") }}</template>
            <template #subtitle>{{ scannedCode }}</template>

            <div class="border-t border-gray-300 p-4">
              <p v-if="searching" class="text-sm">
                {{ $t("pantry.scan.searching", { code: scannedCode }) }}
              </p>

              <!-- Unknown code: name it and carry on -->
              <form v-else-if="showNewItemForm" class="flex flex-col gap-3" @submit.prevent="createFromScan">
                <p v-if="suggestion?.found" class="text-xs">
                  {{ $t("pantry.scan.suggested_by_openfoodfacts") }}
                  <span v-if="suggestion.amount"> &middot; {{ suggestion.amount }}</span>
                </p>
                <p v-else class="text-sm">
                  {{ $t("pantry.scan.no_match", { code: scannedCode }) }}
                </p>

                <input
                  ref="nameInput"
                  v-model="newName"
                  type="text"
                  class="input input-bordered w-full"
                  :placeholder="$t('pantry.scan.name_placeholder')"
                  maxlength="255"
                />

                <div class="flex flex-wrap gap-2">
                  <div class="min-w-40 grow">
                    <input
                      v-model="newExpiry"
                      type="text"
                      inputmode="numeric"
                      class="input input-bordered w-full"
                      :class="expiryLooksWrong ? 'input-error' : ''"
                      :placeholder="$t('pantry.scan.expiry_placeholder')"
                    />
                    <p v-if="expiryLooksWrong" class="mt-1 text-xs text-error">
                      {{ $t("pantry.scan.expiry_unreadable") }}
                    </p>
                    <p v-else-if="parsedExpiry" class="mt-1 text-xs">
                      {{
                        $t("pantry.scan.expiry_reads_as", {
                          date: formatShortDate(parsedExpiry),
                        })
                      }}
                    </p>
                  </div>
                  <div class="w-28">
                    <input
                      v-model.number="newMinStock"
                      type="number"
                      min="0"
                      class="input input-bordered w-full"
                      :placeholder="$t('items.min_stock')"
                    />
                  </div>
                </div>

                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-xs">{{ $t("pantry.scan.expiry_quick") }}:</span>
                  <button type="button" class="btn btn-ghost btn-xs" @click="setExpiryMonths(6)">+6 M</button>
                  <button type="button" class="btn btn-ghost btn-xs" @click="setExpiryMonths(12)">+1 J</button>
                  <button type="button" class="btn btn-ghost btn-xs" @click="setExpiryMonths(24)">+2 J</button>
                  <button type="button" class="btn btn-ghost btn-xs" @click="newExpiry = ''">
                    {{ $t("pantry.scan.expiry_clear") }}
                  </button>
                </div>

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
