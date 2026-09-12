<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import MdiMicrophone from "~icons/mdi/microphone";
  import MdiStop from "~icons/mdi/stop";

  /**
   * A record button.
   *
   * Click to start, click to stop - rather than hold-to-talk, which on a
   * touchscreen means a finger that must not slip while you are also holding
   * the thing you are describing.
   *
   * Emits the recording and nothing else. What happens to it is the caller's
   * business, so this stays usable anywhere a description gets typed.
   */

  const emit = defineEmits<{ (e: "recorded", audio: Blob, filename: string): void }>();

  const props = defineProps<{ busy?: boolean }>();

  const { t } = useI18n();

  const recording = ref(false);
  const seconds = ref(0);
  const error = ref<string | null>(null);

  let recorder: MediaRecorder | null = null;
  let chunks: Blob[] = [];
  let ticker: number | null = null;

  /**
   * The longest recording accepted. Well under the server's size limit, and a
   * forgotten open microphone stops itself rather than uploading a minute of a
   * kitchen.
   */
  const maxSeconds = 30;

  /** webm is what Chrome records; Safari only does mp4. Let the browser pick. */
  function pickMimeType(): string | undefined {
    const candidates = ["audio/webm", "audio/mp4", "audio/ogg"];
    return candidates.find(type => MediaRecorder.isTypeSupported?.(type));
  }

  function extensionFor(mime: string): string {
    if (mime.includes("mp4")) return "m4a";
    if (mime.includes("ogg")) return "ogg";
    return "webm";
  }

  async function start() {
    error.value = null;

    if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === "undefined") {
      error.value = t("components.voice.unsupported");
      return;
    }

    let stream: MediaStream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch {
      // Refused, or no microphone. Both look the same from here and both mean
      // the same thing to the user.
      error.value = t("components.voice.no_microphone");
      return;
    }

    const mimeType = pickMimeType();
    recorder = new MediaRecorder(stream, mimeType ? { mimeType } : undefined);
    chunks = [];

    recorder.ondataavailable = event => {
      if (event.data.size > 0) chunks.push(event.data);
    };

    recorder.onstop = () => {
      stream.getTracks().forEach(track => track.stop());
      stopTicker();
      recording.value = false;

      const type = recorder?.mimeType || mimeType || "audio/webm";
      const audio = new Blob(chunks, { type });
      recorder = null;

      if (audio.size > 0) {
        emit("recorded", audio, `recording.${extensionFor(type)}`);
      }
    };

    recorder.start();
    recording.value = true;
    seconds.value = 0;

    ticker = window.setInterval(() => {
      seconds.value += 1;
      if (seconds.value >= maxSeconds) stop();
    }, 1000);
  }

  function stop() {
    recorder?.stop();
  }

  function stopTicker() {
    if (ticker !== null) {
      window.clearInterval(ticker);
      ticker = null;
    }
  }

  onBeforeUnmount(() => {
    stopTicker();
    // Leaving a page with the microphone still open is the sort of thing people
    // rightly get upset about.
    recorder?.stop();
  });
</script>

<template>
  <div class="flex flex-wrap items-center gap-2">
    <button
      type="button"
      class="btn btn-sm gap-2"
      :class="recording ? 'btn-error' : 'btn-outline'"
      :disabled="props.busy"
      @click="recording ? stop() : start()"
    >
      <MdiStop v-if="recording" class="size-4" />
      <MdiMicrophone v-else class="size-4" />
      {{ recording ? $t("components.voice.stop", { n: maxSeconds - seconds }) : $t("components.voice.start") }}
    </button>

    <span v-if="props.busy" class="text-sm opacity-60">{{ $t("components.voice.working") }}</span>
    <span v-else-if="error" class="text-sm text-error">{{ error }}</span>
    <span v-else-if="!recording" class="text-sm opacity-60">{{ $t("components.voice.hint") }}</span>
  </div>
</template>
