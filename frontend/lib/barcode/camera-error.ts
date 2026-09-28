/**
 * Saying what actually went wrong when the camera will not start.
 *
 * "Scanner error" is useless: the four common causes need four different things
 * done about them, and one of them - another app holding the camera - is
 * invisible from inside the browser unless somebody says so.
 */

export type CameraFailure = "denied" | "missing" | "in_use" | "unsupported_constraints" | "insecure" | "unknown";

/**
 * Classifies a getUserMedia rejection.
 *
 * The names are the DOMException ones the spec defines; browsers also still use
 * the older aliases, which is why several map to the same outcome.
 */
export function classifyCameraError(error: unknown): CameraFailure {
  const name = error instanceof Error ? error.name : "";

  switch (name) {
    case "NotAllowedError":
    case "PermissionDeniedError":
    case "SecurityError":
      return "denied";
    case "NotFoundError":
    case "DevicesNotFoundError":
      return "missing";
    // The camera exists and is allowed, but something else has it. On a tablet
    // that is usually a kiosk or surveillance app watching for motion.
    case "NotReadableError":
    case "TrackStartError":
    case "AbortError":
      return "in_use";
    case "OverconstrainedError":
    case "ConstraintNotSatisfiedError":
      return "unsupported_constraints";
    default:
      break;
  }

  // Without a secure context the API is simply absent, which surfaces as a
  // TypeError rather than a DOMException.
  if (typeof window !== "undefined" && !window.isSecureContext) {
    return "insecure";
  }

  return "unknown";
}

/** The raw browser text, for when the classification is not enough. */
export function cameraErrorDetail(error: unknown): string {
  if (!(error instanceof Error)) {
    return "";
  }

  return [error.name, error.message].filter(Boolean).join(": ");
}
