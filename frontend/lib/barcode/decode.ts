import {
  BarcodeFormat,
  BinaryBitmap,
  DecodeHintType,
  HybridBinarizer,
  MultiFormatReader,
  RGBLuminanceSource,
} from "@zxing/library";

/**
 * Frame decoding, separated from the camera plumbing so it can be tested
 * without a browser.
 *
 * The formats are restricted to what actually turns up here: the 1D codes on
 * food packaging plus QR for Homebox's own labels. Fewer formats means each
 * frame costs less, so more frames get examined per second.
 */
function buildHints(): Map<DecodeHintType, unknown> {
  const hints = new Map();
  hints.set(DecodeHintType.POSSIBLE_FORMATS, [
    BarcodeFormat.EAN_13,
    BarcodeFormat.EAN_8,
    BarcodeFormat.UPC_A,
    BarcodeFormat.UPC_E,
    BarcodeFormat.CODE_128,
    BarcodeFormat.CODE_39,
    BarcodeFormat.ITF,
    BarcodeFormat.QR_CODE,
  ]);
  hints.set(DecodeHintType.TRY_HARDER, true);
  return hints;
}

let reader: MultiFormatReader | null = null;

function getReader(): MultiFormatReader {
  if (!reader) {
    reader = new MultiFormatReader();
    reader.setHints(buildHints());
  }
  return reader;
}

/** Converts RGBA pixels to the luminance buffer zxing wants. */
export function toLuminance(rgba: Uint8ClampedArray, width: number, height: number): Uint8ClampedArray {
  const out = new Uint8ClampedArray(width * height);
  for (let i = 0; i < width * height; i++) {
    const r = rgba[i * 4];
    const g = rgba[i * 4 + 1];
    const b = rgba[i * 4 + 2];
    out[i] = (r * 0.299 + g * 0.587 + b * 0.114) | 0;
  }
  return out;
}

/** Decodes a luminance buffer. Returns the text, or null when nothing is found. */
export function decodeLuminance(lum: Uint8ClampedArray, width: number, height: number): string | null {
  if (width < 1 || height < 1) {
    return null;
  }

  const bitmap = new BinaryBitmap(new HybridBinarizer(new RGBLuminanceSource(lum, width, height)));

  try {
    return getReader().decode(bitmap).getText();
  } catch {
    return null;
  } finally {
    // The reader keeps per-decode state that must not leak into the next frame.
    getReader().reset();
  }
}

/**
 * The centre band of a frame, which is where someone naturally holds a barcode.
 *
 * Cropping raises the pixels-per-bar ratio for the region that matters and
 * skips the rest, which is why a native scanner showing a small window feels so
 * much quicker than one scanning the whole picture.
 */
export function centreCrop(
  lum: Uint8ClampedArray,
  width: number,
  height: number,
  widthFraction = 0.9,
  heightFraction = 0.45
): { data: Uint8ClampedArray; width: number; height: number } {
  const cropW = Math.max(1, Math.round(width * widthFraction));
  const cropH = Math.max(1, Math.round(height * heightFraction));
  const x0 = Math.floor((width - cropW) / 2);
  const y0 = Math.floor((height - cropH) / 2);

  const out = new Uint8ClampedArray(cropW * cropH);
  for (let y = 0; y < cropH; y++) {
    const src = (y0 + y) * width + x0;
    out.set(lum.subarray(src, src + cropW), y * cropW);
  }

  return { data: out, width: cropW, height: cropH };
}

/**
 * Tries the centre band first and the whole frame second.
 *
 * A 1D code that sits off to one side is missed by the crop but found by the
 * full frame, so both are worth the attempt; the crop simply succeeds far more
 * often and costs less.
 */
export function decodeFrame(rgba: Uint8ClampedArray, width: number, height: number): string | null {
  const lum = toLuminance(rgba, width, height);

  const crop = centreCrop(lum, width, height);
  const fromCrop = decodeLuminance(crop.data, crop.width, crop.height);
  if (fromCrop) {
    return fromCrop;
  }

  return decodeLuminance(lum, width, height);
}
