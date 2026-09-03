/**
 * The parts of the pantry terminal that both directions share.
 *
 * Filling the cupboard and emptying it are different jobs with different
 * screens, but the shell around them is the same: say out loud what happened,
 * keep the tablet awake and logged in, and never drop a code on the floor.
 */

export type KioskStatusKind = "idle" | "ok" | "warn" | "error";

export interface KioskStatus {
  kind: KioskStatusKind;
  title: string;
  detail?: string;
  note?: string;
}

export interface UnresolvedScan {
  code: string;
  at: string;
  reason: string;
}

export function useKioskShell() {
  const api = useUserApi();

  const started = ref(false);
  const status = ref<KioskStatus>({ kind: "idle", title: "" });

  /**
   * Codes that could not be booked, kept on the device.
   *
   * Swallowing them would be the worst behaviour available: the tin leaves the
   * cupboard, the stock stays where it was, and nothing ever says so. Keeping
   * them turns the failure into a short list to work through on the phone.
   */
  const unresolved = useLocalStorage<UnresolvedScan[]>("homebox/kiosk/unresolved", []);

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

  function sound(kind: KioskStatusKind) {
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

  /** Sets the status and says it out loud. */
  function report(next: KioskStatus) {
    status.value = next;
    sound(next.kind);
  }

  function park(code: string, reason: string) {
    unresolved.value = [{ code, at: new Date().toISOString(), reason }, ...unresolved.value].slice(0, 100);
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

  type WakeLockCapable = Navigator & {
    wakeLock?: { request: (type: string) => Promise<{ release: () => Promise<void> }> };
  };

  async function keepAwake() {
    try {
      wakeLock = (await (navigator as WakeLockCapable).wakeLock?.request("screen")) ?? null;
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

  /**
   * The one tap the terminal needs. Browsers grant neither an audio context nor
   * a wake lock without a gesture, so there is no way around it - but there is
   * a way around doing it more than once.
   */
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

  onMounted(() => document.addEventListener("visibilitychange", onVisibilityChange));

  onBeforeUnmount(() => {
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

  return { started, status, unresolved, tone, start, report, park };
}
