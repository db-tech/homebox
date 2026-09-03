<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { classifyScan } from "~~/lib/barcode/scan-target";
  import { WedgeReader, isEditingTarget } from "~~/lib/barcode/wedge";
  import { useKioskShell } from "~~/composables/use-kiosk-shell";
  import { pickForConsume } from "~~/lib/pantry/consume-target";
  import { findBatch, planFill } from "~~/lib/pantry/fill-target";
  import type { ItemSummary, LocationOut } from "~~/lib/api/types/data-contracts";
  import MdiUndo from "~icons/mdi/undo-variant";
  import MdiClose from "~icons/mdi/close";

  /**
   * The pantry terminal.
   *
   * A tablet on the wall next to the cupboard with a handheld scanner beside
   * it, working in both directions: unpacking a box into the pantry, and taking
   * things back out of it.
   *
   * Deliberately not the scanner page. That page is a form you scroll through,
   * which is fine on a phone you are holding and wrong here: with a handheld
   * scanner the result would land below the fold and you would scroll up after
   * every single tin. Everything on this page fits one screen and never moves.
   *
   * The other rule is that a text field is never focused on its own. Focus in a
   * field means the next scan is typed into it instead of being booked, which
   * is exactly the kind of quiet nonsense a wall device must not produce.
   */

  definePageMeta({
    middleware: ["auth"],
    layout: "empty",
  });
  useHead({ title: "Homebox | Pantry" });

  const { t } = useI18n();
  const api = useUserApi();
  const { started, status, unresolved, tone, start, report, park } = useKioskShell();

  type Mode = "consume" | "fill";
  const mode = useLocalStorage<Mode>("homebox/kiosk/mode", "consume");

  const busy = ref(false);
  const listOpen = ref(false);

  /**
   * Bookings that can still be taken back, newest last.
   *
   * A created item is remembered as such: undoing a creation has to remove the
   * item, not leave an empty one behind that will haunt the barcode later.
   */
  type Booking =
    | { kind: "consume" | "restock"; itemId: string; entryId: string; name: string }
    | { kind: "created"; itemId: string; barcode: string; name: string };
  const undoStack = ref<Booking[]>([]);

  /**
   * How often the same item was scanned in a row.
   *
   * Taking three tins out means three scans of the same code, so counting them
   * down is correct and must not be suppressed. Showing the run is what makes
   * an accidental double scan visible rather than silent.
   */
  const streak = ref(0);
  const lastItemId = ref<string | null>(null);

  function forgetStreak() {
    streak.value = 0;
    lastItemId.value = null;
  }

  function noteStreak(itemId: string) {
    streak.value = lastItemId.value === itemId ? streak.value + 1 : 1;
    lastItemId.value = itemId;
  }

  // ---------------------------------------------------------------------------
  // Taking things out
  // ---------------------------------------------------------------------------

  async function consumeScan(code: string, items: ItemSummary[]) {
    const choice = pickForConsume(items);

    if (!choice) {
      forgetStreak();
      const known = items.length > 0;
      park(code, known ? "already_empty" : "unknown");
      report({
        kind: known ? "warn" : "error",
        title: known ? t("pantry.kiosk.already_empty", { name: items[0].name }) : t("pantry.kiosk.unknown"),
        detail: code,
        note: t("pantry.kiosk.noted"),
      });
      return;
    }

    const item = choice.item;
    const { data, error } = await api.pantry.record(item.id, {
      amount: 1,
      type: "consume",
      note: "",
      date: new Date(),
    });

    if (error || !data) {
      forgetStreak();
      report({ kind: "error", title: t("pantry.kiosk.book_failed"), detail: item.name });
      return;
    }

    const booked: Booking = { kind: "consume", itemId: item.id, entryId: data.id, name: item.name };
    undoStack.value = [...undoStack.value, booked].slice(-20);
    noteStreak(item.id);

    const left = item.quantity - 1;
    // Running out is worth hearing about even for something with no minimum
    // set, because the next person at the cupboard finds an empty shelf.
    const low = left === 0 || (item.minStock > 0 && left <= item.minStock);

    const notes: string[] = [];
    if (low) notes.push(t("pantry.kiosk.below_minimum"));
    if (choice.alternatives > 0) notes.push(t("pantry.kiosk.soonest_of", { n: choice.alternatives + 1 }));

    report({
      kind: low ? "warn" : "ok",
      title: item.name,
      detail: left === 0 ? t("pantry.kiosk.left_none") : t("pantry.kiosk.left", { n: left }),
      note: notes.join(" · ") || undefined,
    });
  }

  // ---------------------------------------------------------------------------
  // Putting things in
  //
  // The whole difficulty is that one product can have two best-before dates and
  // an item can only hold one, so they become two items. A scan therefore does
  // not say which of them it belongs to - only the date does. It is asked for
  // once per batch and then remembered, so a box of identical tins costs one
  // scan each and nothing more.
  // ---------------------------------------------------------------------------

  const location = ref<LocationOut | null>(null);
  const locationId = useLocalStorage<string>("homebox/kiosk/location", "");

  /** Barcode to the item it was booked into earlier in this session. */
  const settled = ref<Record<string, string>>({});

  type FillStep = "none" | "choose" | "date" | "name";
  const fillStep = ref<FillStep>("none");

  const pendingCode = ref("");
  const pendingItems = ref<ItemSummary[]>([]);
  const pendingName = ref("");
  const editingName = ref(false);

  function resetFill() {
    fillStep.value = "none";
    pendingCode.value = "";
    pendingItems.value = [];
    pendingName.value = "";
    editingName.value = false;
  }

  async function fillScan(code: string, items: ItemSummary[], suggestion?: string) {
    if (!location.value) {
      report({ kind: "error", title: t("pantry.kiosk.pick_location") });
      return;
    }

    const plan = planFill(items, settled.value[code] ?? null);

    if (plan.kind === "add") {
      await addToBatch(code, plan.item);
      return;
    }

    pendingCode.value = code;
    pendingItems.value = items;

    if (plan.kind === "choose") {
      fillStep.value = "choose";
      report({ kind: "ok", title: items[0].name, detail: t("pantry.kiosk.which_batch") });
      return;
    }

    // Nothing carries this code yet. The name is whatever the product lookup
    // offered; it is shown rather than focused, because focusing it would send
    // the next scan into the field instead of booking it.
    pendingName.value = suggestion ?? "";
    fillStep.value = pendingName.value ? "date" : "name";
    report({
      kind: pendingName.value ? "ok" : "warn",
      title: pendingName.value || t("pantry.kiosk.unknown"),
      detail: pendingName.value ? t("pantry.kiosk.which_date") : t("pantry.kiosk.needs_name"),
    });
  }

  /** One more of a batch that is already there. */
  async function addToBatch(code: string, item: ItemSummary) {
    const { data, error } = await api.pantry.record(item.id, {
      amount: 1,
      type: "restock",
      note: "",
      date: new Date(),
    });

    if (error || !data) {
      forgetStreak();
      report({ kind: "error", title: t("pantry.kiosk.book_failed"), detail: item.name });
      return;
    }

    settled.value = { ...settled.value, [code]: item.id };
    const booked: Booking = { kind: "restock", itemId: item.id, entryId: data.id, name: item.name };
    undoStack.value = [...undoStack.value, booked].slice(-20);
    noteStreak(item.id);
    resetFill();

    report({
      kind: "ok",
      title: item.name,
      detail: t("pantry.kiosk.now", { n: item.quantity + 1 }),
      note: expiryNote(item),
    });
  }

  function expiryNote(item: ItemSummary): string | undefined {
    const at = new Date(item.expiryDate);
    if (Number.isNaN(at.getTime()) || at.getFullYear() <= 1900) return undefined;
    return t("pantry.kiosk.best_before", { date: at.toLocaleDateString() });
  }

  /** The date settles which batch the scan belongs to, existing or new. */
  async function pickDate(date: Date | null) {
    const code = pendingCode.value;
    const existing = findBatch(pendingItems.value, date);

    if (existing) {
      await addToBatch(code, existing);
      return;
    }

    await createBatch(code, date);
  }

  async function createBatch(code: string, date: Date | null) {
    if (!location.value) {
      report({ kind: "error", title: t("pantry.kiosk.pick_location") });
      return;
    }

    // A new batch of a product already in the pantry keeps its name, so the two
    // batches read as the same thing on the shelf list.
    const name = pendingItems.value[0]?.name || pendingName.value.trim();
    if (!name) {
      fillStep.value = "name";
      return;
    }

    const { data, error } = await api.items.create({
      parentId: null,
      name,
      description: "",
      locationId: location.value.id,
      labelIds: [],
      barcode: code,
      expiryDate: date ?? "",
      // The minimum belongs to the product rather than to one batch of it, so a
      // batch created by a scan does not carry one. The low-stock view totals
      // the batches of a barcode, so a minimum set on any of them still counts.
      minStock: 0,
    });

    if (error || !data) {
      forgetStreak();
      park(code, "create_failed");
      report({ kind: "error", title: t("pantry.kiosk.create_failed"), detail: code, note: t("pantry.kiosk.noted") });
      return;
    }

    settled.value = { ...settled.value, [code]: data.id };
    const booked: Booking = { kind: "created", itemId: data.id, barcode: code, name };
    undoStack.value = [...undoStack.value, booked].slice(-20);
    noteStreak(data.id);
    resetFill();

    report({
      kind: "ok",
      title: name,
      detail: t("pantry.kiosk.created"),
      note: date ? t("pantry.kiosk.best_before", { date: date.toLocaleDateString() }) : undefined,
    });
  }

  /** Abandons the batch this barcode was settled into, to enter a new date. */
  function differentDate() {
    const code = pendingCode.value || lastBarcode.value;
    if (!code) return;

    const { [code]: _dropped, ...rest } = settled.value;
    settled.value = rest;
    pendingCode.value = code;
    fillStep.value = "date";
  }

  const lastBarcode = ref("");

  // ---------------------------------------------------------------------------
  // Undo
  // ---------------------------------------------------------------------------

  const canUndo = computed(() => undoStack.value.length > 0 && !busy.value);

  /**
   * Takes the last booking back.
   *
   * Deleting a log entry deliberately does not move the stock, so reversing one
   * is a movement the other way; both entries then go, because a pair that
   * cancels out is noise in a log meant to show what was actually used.
   */
  async function undoLast() {
    const last = undoStack.value[undoStack.value.length - 1];
    if (!last || busy.value) return;

    busy.value = true;
    try {
      if (last.kind === "created") {
        const { error } = await api.items.delete(last.itemId);
        if (error) {
          report({ kind: "error", title: t("pantry.kiosk.undo_failed"), detail: last.name });
          return;
        }

        const { [last.barcode]: _dropped, ...rest } = settled.value;
        settled.value = rest;
      } else {
        const { data: back, error } = await api.pantry.record(last.itemId, {
          amount: 1,
          type: last.kind === "consume" ? "restock" : "consume",
          note: "",
          date: new Date(),
        });

        if (error || !back) {
          report({ kind: "error", title: t("pantry.kiosk.undo_failed"), detail: last.name });
          return;
        }

        await api.pantry.deleteEntry(last.entryId);
        await api.pantry.deleteEntry(back.id);
      }

      undoStack.value = undoStack.value.slice(0, -1);
      forgetStreak();
      resetFill();
      report({ kind: "ok", title: t("pantry.kiosk.undone"), detail: last.name });
    } finally {
      busy.value = false;
    }
  }

  // ---------------------------------------------------------------------------
  // Scanning
  // ---------------------------------------------------------------------------

  async function onScan(text: string) {
    const target = classifyScan(text, window.location.origin);

    // A Homebox label is not a product. Following it would navigate out of the
    // terminal, which is exactly what a wall device must never do on its own.
    if (target.kind !== "code") {
      forgetStreak();
      report({ kind: "error", title: t("pantry.kiosk.not_a_product"), detail: text });
      return;
    }

    const code = target.value;
    lastBarcode.value = code;

    const { data, error } = await api.pantry.scan(code);
    if (error || !data) {
      forgetStreak();
      park(code, "lookup_failed");
      report({ kind: "error", title: t("pantry.kiosk.lookup_failed"), detail: code });
      return;
    }

    const items = data.items ?? [];

    if (mode.value === "consume") {
      resetFill();
      await consumeScan(code, items);
      return;
    }

    const suggestion = data.suggestion?.found
      ? [data.suggestion.brand, data.suggestion.name].filter(Boolean).join(" ").trim()
      : "";
    await fillScan(code, items, suggestion);
  }

  const wedge = new WedgeReader();

  function onKeyDown(event: KeyboardEvent) {
    // Keys typed into a field belong to that field. A scan fired while a text
    // box has focus lands there visibly, which is correctable; silently taking
    // what somebody is writing would not be.
    if (isEditingTarget(event.target)) {
      return;
    }

    const code = wedge.push(event.key, event.timeStamp);
    if (!code || busy.value) {
      return;
    }

    event.preventDefault();
    run(code);
  }

  function run(code: string) {
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

  function submitManual() {
    const code = manualCode.value.trim();
    if (!code || busy.value) return;

    manualCode.value = "";
    manualOpen.value = false;
    run(code);
  }

  function dismissUnresolved(index: number) {
    unresolved.value = unresolved.value.filter((_, at) => at !== index);
  }

  function confirmName() {
    if (!pendingName.value.trim()) return;
    editingName.value = false;
    fillStep.value = "date";
  }

  onMounted(async () => {
    window.addEventListener("keydown", onKeyDown);

    if (locationId.value) {
      const { data } = await api.locations.get(locationId.value);
      if (data) location.value = data;
    }
  });

  onBeforeUnmount(() => window.removeEventListener("keydown", onKeyDown));

  watch(location, next => (locationId.value = next?.id ?? ""));

  // Switching direction ends whatever was half-entered, and forgets the batches
  // settled while filling: coming back later is a new box with a new date.
  watch(mode, () => {
    resetFill();
    forgetStreak();
    settled.value = {};
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
      <!-- Direction, and where a filled item lands. Both are set once and then
           stay out of the way. -->
      <div class="flex items-center gap-2 border-b border-white/10 p-2">
        <div class="join">
          <button
            class="join-item btn btn-sm"
            :class="mode === 'consume' ? 'btn-primary' : 'btn-ghost'"
            @click="mode = 'consume'"
          >
            {{ $t("pantry.kiosk.mode_consume") }}
          </button>
          <button
            class="join-item btn btn-sm"
            :class="mode === 'fill' ? 'btn-primary' : 'btn-ghost'"
            @click="mode = 'fill'"
          >
            {{ $t("pantry.kiosk.mode_fill") }}
          </button>
        </div>

        <div v-if="mode === 'fill'" class="min-w-0 flex-1">
          <LocationSelector v-model="location" />
        </div>
      </div>

      <!-- The whole point of the screen: what just happened, readable from a
           step away without putting the tin down. -->
      <div
        class="flex flex-1 flex-col items-center justify-center gap-3 overflow-y-auto p-4 text-center transition-colors"
        :class="tone"
      >
        <!-- Which batch does this tin belong to? Only asked once per barcode
             per session; the rest of the box goes in without a tap. -->
        <template v-if="fillStep === 'choose'">
          <p class="text-3xl font-bold">{{ pendingItems[0]?.name }}</p>
          <p class="text-lg opacity-70">{{ $t("pantry.kiosk.which_batch") }}</p>
          <div class="flex w-full max-w-xl flex-col gap-2">
            <button
              v-for="batch in pendingItems"
              :key="batch.id"
              class="btn btn-lg justify-between"
              @click="addToBatch(pendingCode, batch)"
            >
              <span>{{ expiryNote(batch) ?? $t("pantry.kiosk.no_date") }}</span>
              <span class="opacity-60">{{ batch.quantity }}</span>
            </button>
            <button class="btn btn-outline btn-lg" @click="fillStep = 'date'">
              {{ $t("pantry.kiosk.other_date") }}
            </button>
          </div>
        </template>

        <template v-else-if="fillStep === 'name'">
          <p class="text-2xl font-bold">{{ $t("pantry.kiosk.needs_name") }}</p>
          <p class="font-mono opacity-60">{{ pendingCode }}</p>
          <form class="flex w-full max-w-xl gap-2" @submit.prevent="confirmName">
            <input v-model="pendingName" class="input input-bordered input-lg flex-1" autofocus />
            <button class="btn btn-primary btn-lg" type="submit">{{ $t("pantry.kiosk.next") }}</button>
          </form>
        </template>

        <template v-else-if="fillStep === 'date'">
          <p class="text-2xl font-bold">{{ pendingItems[0]?.name || pendingName }}</p>
          <p class="text-lg opacity-70">{{ $t("pantry.kiosk.which_date") }}</p>
          <div class="w-full max-w-xl">
            <FormDateTapPicker :model-value="null" @update:model-value="pickDate" />
          </div>
          <button class="btn btn-ghost btn-sm" @click="pickDate(null)">{{ $t("pantry.kiosk.no_date") }}</button>
        </template>

        <template v-else-if="status.kind === 'idle'">
          <p class="text-4xl font-light">{{ $t("pantry.kiosk.ready") }}</p>
          <p class="text-lg opacity-60">
            {{ mode === "fill" ? $t("pantry.kiosk.ready_fill") : $t("pantry.kiosk.ready_hint") }}
          </p>
        </template>

        <template v-else>
          <p class="max-w-full break-words text-5xl font-bold leading-tight">{{ status.title }}</p>
          <p v-if="status.detail" class="text-6xl font-black tabular-nums">{{ status.detail }}</p>
          <p v-if="status.note" class="text-xl opacity-80">{{ status.note }}</p>
          <p v-if="streak > 1" class="text-xl opacity-60">{{ $t("pantry.kiosk.streak", { n: streak }) }}</p>

          <!-- The escape hatch for the one tin in the box with a different
               date: it was just booked into the settled batch, so undo that
               and ask again. -->
          <button
            v-if="mode === 'fill' && lastBarcode && status.kind === 'ok'"
            class="btn btn-outline btn-sm mt-2"
            @click="undoLast().then(differentDate)"
          >
            {{ $t("pantry.kiosk.other_date") }}
          </button>
        </template>
      </div>

      <!-- Undo sits permanently on screen and is sized for a thumb: it is what
           makes an accidental scan a non-event rather than a correction later. -->
      <div class="flex items-center gap-2 border-t border-white/10 p-3">
        <button
          class="btn btn-lg flex-1 gap-2"
          :class="canUndo ? 'btn-neutral' : 'btn-ghost'"
          :disabled="!canUndo"
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
          <button class="btn btn-ghost btn-sm" @click="dismissUnresolved(i)">
            {{ $t("pantry.kiosk.list_done") }}
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>
