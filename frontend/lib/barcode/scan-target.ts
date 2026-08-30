/**
 * Decides what a scanned code actually is.
 *
 * Three things can come off a scan:
 *   - a Homebox label, which is a URL pointing at this very instance
 *   - a product barcode, which is a run of digits
 *   - something else entirely, most often a manufacturer's marketing QR code
 *     printed next to the barcode on the packaging
 *
 * The third case is why this exists. Treating any URL as an internal path and
 * navigating to it drops the user on a 404 or back at the start page with no
 * explanation, which is exactly what happens when you scan the QR code on a tin
 * instead of its barcode.
 */

export type ScanTarget =
  /** One of our own labels; navigate to this path. */
  | { kind: "internal"; path: string }
  /** Looks like a product code; look it up. */
  | { kind: "code"; value: string }
  /** A URL belonging to somebody else. Show it, do not follow it. */
  | { kind: "foreign"; value: string };

/** Characters allowed to survive into a path we navigate to. */
const PATH_SAFE = /[^a-zA-Z0-9-_/]/g;

/**
 * Classifies scanned text.
 *
 * `origin` is the origin of the running app; a URL from anywhere else is
 * deliberately not followed.
 */
export function classifyScan(text: string, origin: string): ScanTarget {
  const raw = text.trim();

  let url: URL | null = null;
  try {
    url = new URL(raw);
  } catch {
    url = null;
  }

  if (url) {
    // Only our own labels may drive navigation. Following a foreign URL's path
    // into Homebox produces a route that does not exist, and the user sees an
    // unexplained jump rather than a scan result.
    if (url.origin === origin && url.pathname.startsWith("/")) {
      const path = url.pathname.replace(PATH_SAFE, "");
      // A bare "/" is the start page and carries no information, so it is not
      // worth navigating for.
      if (path && path !== "/") {
        return { kind: "internal", path };
      }
    }

    return { kind: "foreign", value: raw };
  }

  return { kind: "code", value: raw };
}
