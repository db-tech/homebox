/**
 * Support for handheld barcode scanners.
 *
 * A pistol-grip scanner - USB or Bluetooth - presents itself to the phone or
 * tablet as a keyboard. It "types" the digits of the code and then presses
 * Enter. Nothing needs to be paired with the app itself; it only has to notice
 * that the typing came from a machine rather than from a person.
 *
 * The distinguishing feature is speed. A scanner emits its characters a few
 * milliseconds apart, while a person cannot hold much under a tenth of a second
 * for a whole code. So a run of characters with tiny gaps, closed by Enter, is
 * a scan; anything slower is somebody typing and is left alone.
 */

export interface WedgeOptions {
  /** Longest gap between two keystrokes still counted as machine speed. */
  maxGapMs?: number;
  /** Shortest run accepted, so a stray Enter cannot commit a fragment. */
  minLength?: number;
}

const DEFAULTS = {
  // Cheap scanners are slower than the datasheet suggests, and Bluetooth adds
  // jitter, so this sits well above their real gaps but far below human typing.
  maxGapMs: 120,
  minLength: 4,
};

export class WedgeReader {
  private buffer = "";
  private lastAt = 0;
  private readonly maxGapMs: number;
  private readonly minLength: number;

  constructor(options: WedgeOptions = {}) {
    this.maxGapMs = options.maxGapMs ?? DEFAULTS.maxGapMs;
    this.minLength = options.minLength ?? DEFAULTS.minLength;
  }

  /** True while a machine-speed run is in progress. */
  get active(): boolean {
    return this.buffer.length > 0;
  }

  reset(): void {
    this.buffer = "";
    this.lastAt = 0;
  }

  /**
   * Feeds one keystroke.
   *
   * `key` is the KeyboardEvent key. Returns the completed code when Enter
   * closes a run that looks like a scan, and null otherwise.
   */
  push(key: string, at: number): string | null {
    if (key === "Enter") {
      const done = this.buffer;
      const fastEnough = at - this.lastAt <= this.maxGapMs;
      this.reset();
      return done.length >= this.minLength && fastEnough ? done : null;
    }

    // Only single printable characters belong in a code; modifiers, arrows and
    // the like are not part of one and must not break an in-flight run either.
    if (key.length !== 1) {
      return null;
    }

    // Too slow to be a machine: start over from this character.
    if (this.buffer && at - this.lastAt > this.maxGapMs) {
      this.buffer = "";
    }

    this.buffer += key;
    this.lastAt = at;
    return null;
  }
}

/**
 * Whether keystrokes should be watched at all.
 *
 * While the user is in a text field the keys belong to that field. A scanner
 * fired at that moment would type into it, which is visible and correctable,
 * and is far better than silently stealing what somebody is writing.
 */
export function isEditingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }

  const tag = target.tagName.toLowerCase();
  return tag === "input" || tag === "textarea" || tag === "select" || target.isContentEditable;
}
