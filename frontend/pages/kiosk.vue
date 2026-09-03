<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { classifyScan } from "~~/lib/barcode/scan-target";
  import { WedgeReader, isEditingTarget } from "~~/lib/barcode/wedge";
  import { pickForConsume } from "~~/lib/pantry/consume-target";
  import MdiUndo from "~icons/mdi/undo-variant";
  import MdiClose from "~icons/mdi/close";

  /**
   * The pantry terminal.
   *
   * A tablet on the wall next to the cupboard with a handheld scanner beside
   * it. Taking something out costs one scan and nothing else - no picking, no
   * confirming, no typing. Everything on this page exists to make that one
   * scan trustworthy: it says out loud what it booked, and it can take it back.
   *
   * Deliberately not on the scanner page. That page is for filling the pantry,
   * where you want the camera, a location and a form. This one is for emptying
   * it, where every one of those is a way to get it wrong.
   */

  definePageMeta({
    middleware: ["auth"],
    layout: "empty",
  });
  useHead({ title: "Homebox | Pantry" });

  const { t } = useI18n();
  const api = useUserApi();

  type StatusKind = "idle" | "ok" | "warn" | "error";

  interface Status {
    kind: StatusKind;
    title: string;
    detail?: string;
    note?: string;
  }

  const status = ref<Status>({ kind: "idle", title: "" });
  const busy = ref(false);
  const started = ref(false);

  /** Bookings that can still be taken back, newest last. */
  const undoStack = ref<Array<{ itemId: string; entryId: string; name: string }>>([]);

  /**
   * Codes that could not be booked, kept on the device.
   *
   * Swallowing them would be the worst behaviour available: the tin leaves the
   * cupboard, the stock stays where it was, and nothing ever says so. Keeping
   * them turns the failure into a short list to work through on the phone.
   */
  const unresolved = useLocalStorage<Array<{ code: string; at: string; reason: string }>>(
    "homebox/kiosk/unresolved",
    []
  );
  const listOpen = ref(false);

  /**
   * How often the same item was scanned in a row.
   *
   * Taking three tins out means three scans of the same code, so counting them
   * down is correct and must not be suppressed. Showing the run is what makes
   * an accidental double scan visible rather than silent.
   */
  const streak = ref(0);
  const lastItemId = ref<string | null>(null);

  // ---------------------------------------------------------------------------
  // Sound
  //
  // The scanner beeps when it *reads* a code, not when the stock actually
  // moved. Those are different events and the difference only shows up when it
  // hurts, so the booking gets its own voice - and then you do not have to look
  // at the screen at all in the normal case.
  // ---------------------------------------------------------------------------

  let audio: AudioContext | null = null;

  function beep(hz: number, ms: number, delay = 0) {
    if (!audio) return;

    const osc = audio.createOscillator();
    const gain = audio.createGain();
    osc.type = "sine";
    osc.frequency.value = hz;
    osc.connect(gain).connect(audio.destination);

    const at = audio.currentTime + delay;
    const until = at + ms / 1000;
    gain.gain.setValueAtTime(0.0001, at);
    gain.gain.exponentialRampToValueAtTime(0.25, at + 0.012);
    gain.gain.exponentialRampToValueAtTime(0.0001, until);
    osc.start(at);
    osc.stop(until + 0.02);
  }

  function sound(kind: StatusKind) {
    switch (kind) {
      case "ok":
        beep(1046, 90);
        break;
      case "warn":
        beep(660, 110);
        beep(660, 110, 0.17);
        break;
      case "error":
        beep(196, 340);
        break;
    }
  }

  function report(next: Status) {
    status.value = next;
    sound(next.kind);
  }

  function park(code: string, reason: string) {
    unresolved.value = [{ code, at: new Date().toISOString(), reason }, ...unresolved.value].slice(0, 100);
  }

  // ---------------------------------------------------------------------------
  // Scanning
  // ---------------------------------------------------------------------------

  async function onScan(text: string) {
    const target = classifyScan(text, window.location.origin);

    // A Homebox label is not a product. Following it would navigate out of the
    // terminal, which is exactly what a wall device must never do on its own.
    if (target.kind !== "code") {
      streak.value = 0;
      lastItemId.value = null;
      report({ kind: "error", title: t("pantry.kiosk.not_a_product"), detail: text });
      return;
    }

    const code = target.value;
    const { data, error } = await api.pantry.scan(code);

    if (error || !data) {
      streak.value = 0;
      lastItemId.value = null;
      park(code, "lookup_failed");
      report({ kind: "error", title: t("pantry.kiosk.lookup_failed"), detail: code });
      return;
    }

    const choice = pickForConsume(data.items ?? []);

    if (!choice) {
      streak.value = 0;
      lastItemId.value = null;

      const known = (data.items ?? []).length > 0;
      park(code, known ? "already_empty" : "unknown");
      report({
        kind: known ? "warn" : "error",
        title: known ? t("pantry.kiosk.already_empty", { name: data.items[0].name }) : t("pantry.kiosk.unknown"),
        detail: code,
        note: t("pantry.kiosk.noted"),
      });
      return;
    }

    await book(choice.item, choice.alternatives);
  }

  async function book(item: { id: string; name: string; quantity: number; minStock: number }, alternatives: number) {
    const { data, error } = await api.pantry.record(item.id, {
      amount: 1,
      type: "consume",
      note: "",
      date: new Date(),
    });

    if (error || !data) {
      streak.value = 0;
      lastItemId.value = null;
      report({ kind: "error", title: t("pantry.kiosk.book_failed"), detail: item.name });
      return;
    }

    undoStack.value = [...undoStack.value, { itemId: item.id, entryId: data.id, name: item.name }].slice(-20);

    streak.value = lastItemId.value === item.id ? streak.value + 1 : 1;
    lastItemId.value = item.id;

    const left = item.quantity - 1;
    // Running out is worth hearing about even for something with no minimum
    // set, because the next person at the cupboard finds an empty shelf.
    const low = left === 0 || (item.minStock > 0 && left <= item.minStock);

    const notes: string[] = [];
    if (low) notes.push(t("pantry.kiosk.below_minimum"));
    if (alternatives > 0) notes.push(t("pantry.kiosk.soonest_of", { n: alternatives + 1 }));

    report({
      kind: low ? "warn" : "ok",
      title: item.name,
      detail: left === 0 ? t("pantry.kiosk.left_none") : t("pantry.kiosk.left", { n: left }),
      note: notes.join(" · ") || undefined,
    });
  }

  /**
   * Takes the last booking back.
   *
   * Deleting a log entry deliberately does not move the stock, so putting the
   * item back is a restock; both entries then go, because a pair that cancels
   * out is noise in a log meant to show what was actually used.
   */
  async function undoLast() {
    const last = undoStack.value[undoStack.value.length - 1];
    if (!last || busy.value) return;

    busy.value = true;
    try {
      const { data: back, error } = await api.pantry.record(last.itemId, {
        amount: 1,
        type: "restock",
        note: "",
        date: new Date(),
      });

      if (error || !back) {
        report({ kind: "error", title: t("pantry.kiosk.undo_failed"), detail: last.name });
        return;
      }

      await api.pantry.deleteEntry(last.entryId);
      await api.pantry.deleteEntry(back.id);

      undoStack.value = undoStack.value.slice(0, -1);
      streak.value = 0;
      lastItemId.value = null;
      report({ kind: "ok", title: t("pantry.kiosk.undone"), detail: last.name });
    } finally {
      busy.value = false;
    }
  }

  // ---------------------------------------------------------------------------
  // Handheld scanner
  // ---------------------------------------------------------------------------

  const wedge = new WedgeReader();

  function onKeyDown(event: KeyboardEvent) {
    if (isEditingTarget(event.target)) {
      return;
    }

    const code = wedge.push(event.key, event.timeStamp);
    if (!code || busy.value) {
      return;
    }

    event.preventDefault();
    busy.value = true;
    onScan(code)
      .catch(() => report({ kind: "error", title: t("pantry.kiosk.lookup_failed"), detail: code }))
      .finally(() => {
        busy.value = false;
      });
  }

  /** Same path as a scan, for when the scanner is flat or a code is damaged. */
  const manualCode = ref("");
  const manualOpen = ref(false);

  async function submitManual() {
    const code = manualCode.value.trim();
    if (!code || busy.value) return;

    manualCode.value = "";
    manualOpen.value = false;
    busy.value = true;
    try {
      await onScan(code);
    } finally {
      busy.value = false;
    }
  }

  // ---------------------------------------------------------------------------
  // Staying alive
  //
  // Two things end a wall terminal quietly: the screen locking, which eats the
  // first scan of every visit, and the session expiring, which turns every scan
  // into a redirect. Both are handled here rather than left to the tablet.
  // ---------------------------------------------------------------------------

  let wakeLock: { release: () => Promise<void> } | null = null;
  let refreshTimer: number | null = null;

  async function keepAwake() {
    try {
      wakeLock =
        (await (
          navigator as Navigator & { wakeLock?: { request: (t: string) => Promise<{ release: () => Promise<void> }> } }
        ).wakeLock?.request("screen")) ?? null;
    } catch {
      // Not supported, or refused because the page was hidden. The terminal
      // still works; the tablet's own screen timeout then applies.
      wakeLock = null;
    }
  }

  function onVisibilityChange() {
    if (document.visibilityState === "visible" && started.value && !wakeLock) {
      keepAwake().catch(() => {});
    }
  }

  async function start() {
    started.value = true;
    status.value = { kind: "idle", title: "" };

    audio ??= new AudioContext();
    await audio.resume().catch(() => {});

    await keepAwake();
    await document.documentElement.requestFullscreen?.().catch(() => {});

    const extend = () => api.user.refresh().catch(() => {});
    extend();
    refreshTimer = window.setInterval(extend, 6 * 60 * 60 * 1000);
  }

  onMounted(() => {
    window.addEventListener("keydown", onKeyDown);
    document.addEventListener("visibilitychange", onVisibilityChange);
  });

  onBeforeUnmount(() => {
    window.removeEventListener("keydown", onKeyDown);
    document.removeEventListener("visibilitychange", onVisibilityChange);
    if (refreshTimer !== null) window.clearInterval(refreshTimer);
    wakeLock?.release().catch(() => {});
  });

  const tone = computed(() => {
    switch (status.value.kind) {
      case "ok":
        return "bg-success/15 text-success";
      case "warn":
        return "bg-warning/15 text-warning";
      case "error":
        return "bg-error/15 text-error";
      default:
        return "bg-base-300/10 text-base-content/40";
    }
  });
</script>

<template>
  <div class="fixed inset-0 flex select-none flex-col bg-black text-base-content">
    <!-- Setup screen. One tap, once, so the tablet may keep the screen on and
         make a sound - browsers grant neither without a gesture. -->
    <div v-if="!started" class="flex flex-1 flex-col items-center justify-center gap-6 p-8 text-center">
      <h1 class="text-3xl font-bold text-base-content">{{ $t("pantry.kiosk.title") }}</h1>
      <p class="max-w-md text-base-content/60">{{ $t("pantry.kiosk.start_hint") }}</p>
      <button class="btn btn-primary btn-lg" @click="start">{{ $t("pantry.kiosk.start") }}</button>
      <NuxtLink to="/pantry" class="link text-sm text-base-content/40">{{ $t("pantry.kiosk.exit") }}</NuxtLink>
    </div>

    <template v-else>
      <!-- The whole point of the screen: what just happened, readable from a
           step away without putting the tin down. -->
      <div
        class="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-center transition-colors"
        :class="tone"
      >
        <template v-if="status.kind === 'idle'">
          <p class="text-4xl font-light">{{ $t("pantry.kiosk.ready") }}</p>
          <p class="text-lg opacity-60">{{ $t("pantry.kiosk.ready_hint") }}</p>
        </template>

        <template v-else>
          <p class="max-w-full break-words text-5xl font-bold leading-tight">{{ status.title }}</p>
          <p v-if="status.detail" class="text-7xl font-black tabular-nums">{{ status.detail }}</p>
          <p v-if="status.note" class="text-xl opacity-80">{{ status.note }}</p>
          <p v-if="streak > 1" class="text-xl opacity-60">{{ $t("pantry.kiosk.streak", { n: streak }) }}</p>
        </template>
      </div>

      <!-- Undo sits permanently on screen and is sized for a thumb: it is what
           makes an accidental scan a non-event rather than a correction later. -->
      <div class="flex items-center gap-2 border-t border-white/10 p-3">
        <button
          class="btn btn-lg flex-1 gap-2"
          :class="undoStack.length ? 'btn-neutral' : 'btn-ghost'"
          :disabled="!undoStack.length || busy"
          @click="undoLast"
        >
          <MdiUndo class="size-6" />
          {{ $t("pantry.kiosk.undo") }}
        </button>

        <button
          class="btn btn-ghost btn-lg"
          :class="unresolved.length ? 'text-error' : 'opacity-40'"
          @click="listOpen = true"
        >
          {{ $t("pantry.kiosk.unresolved_count", { n: unresolved.length }) }}
        </button>
      </div>

      <div class="flex items-center justify-between px-3 pb-2 text-xs text-base-content/30">
        <button class="link" @click="manualOpen = !manualOpen">{{ $t("pantry.kiosk.type_code") }}</button>
        <NuxtLink to="/pantry" class="link">{{ $t("pantry.kiosk.exit") }}</NuxtLink>
      </div>

      <form v-if="manualOpen" class="flex gap-2 p-3" @submit.prevent="submitManual">
        <input v-model="manualCode" class="input input-bordered flex-1" inputmode="numeric" autofocus />
        <button class="btn btn-primary" type="submit">{{ $t("pantry.kiosk.look_up") }}</button>
      </form>
    </template>

    <!-- Unresolved scans. Not fixable here on purpose - a code with no item
         needs a name and a date, and this device has no keyboard worth using. -->
    <div v-if="listOpen" class="absolute inset-0 flex flex-col bg-base-100 p-4">
      <div class="mb-3 flex items-center justify-between">
        <h2 class="text-xl font-bold">{{ $t("pantry.kiosk.list_title") }}</h2>
        <button class="btn btn-ghost btn-sm" @click="listOpen = false"><MdiClose class="size-5" /></button>
      </div>

      <p class="mb-3 text-sm text-base-content/60">{{ $t("pantry.kiosk.list_hint") }}</p>

      <p v-if="!unresolved.length" class="py-8 text-center text-base-content/40">
        {{ $t("pantry.kiosk.list_empty") }}
      </p>

      <ul v-else class="flex-1 overflow-y-auto">
        <li v-for="(entry, i) in unresolved" :key="entry.at + entry.code" class="flex items-center gap-3 border-b py-3">
          <div class="flex-1">
            <p class="font-mono text-lg">{{ entry.code }}</p>
            <p class="text-xs text-base-content/50">{{ $t(`pantry.kiosk.reason_${entry.reason}`) }}</p>
          </div>
          <button class="btn btn-ghost btn-sm" @click="unresolved = unresolved.filter((_, n) => n !== i)">
            {{ $t("pantry.kiosk.list_done") }}
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>
