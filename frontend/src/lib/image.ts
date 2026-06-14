/**
 * Client-side image handling for the AI-assist photo picker.
 *
 * - Reject files larger than 5 MiB before doing any work.
 * - Resize to ~1000px on the long edge before upload to keep payloads small
 *   (the backend only needs enough detail to identify the item).
 */

export const MAX_UPLOAD_BYTES = 5 * 1024 * 1024; // 5 MiB
export const MAX_LONG_EDGE = 1000; // px
export const MAX_KEYWORDS_BYTES = 2 * 1024; // 2 KiB

/** Pure, testable guard: true when the file is within the upload size cap. */
export function isWithinSizeLimit(
  sizeBytes: number,
  limit: number = MAX_UPLOAD_BYTES,
): boolean {
  return sizeBytes <= limit;
}

/** Pure, testable guard: UTF-8 byte length of the keywords string. */
export function keywordsByteLength(keywords: string): number {
  return new TextEncoder().encode(keywords).length;
}

export function isKeywordsWithinLimit(
  keywords: string,
  limit: number = MAX_KEYWORDS_BYTES,
): boolean {
  return keywordsByteLength(keywords) <= limit;
}

/** Compute target dimensions constraining the long edge to `maxLongEdge`. */
export function scaledDimensions(
  width: number,
  height: number,
  maxLongEdge: number = MAX_LONG_EDGE,
): { width: number; height: number } {
  const longEdge = Math.max(width, height);
  if (longEdge <= maxLongEdge) {
    return { width, height };
  }
  const scale = maxLongEdge / longEdge;
  return {
    width: Math.round(width * scale),
    height: Math.round(height * scale),
  };
}

/**
 * Resize an image File to <= MAX_LONG_EDGE on its long edge, returning a JPEG
 * Blob. Browser-only (uses createImageBitmap + canvas). Throws if the file is
 * over the size cap or cannot be decoded.
 */
export async function resizeImage(
  file: File,
  maxLongEdge: number = MAX_LONG_EDGE,
): Promise<Blob> {
  if (!isWithinSizeLimit(file.size)) {
    throw new Error("Image exceeds the 5 MiB limit.");
  }

  const bitmap = await createImageBitmap(file);
  try {
    const { width, height } = scaledDimensions(
      bitmap.width,
      bitmap.height,
      maxLongEdge,
    );

    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext("2d");
    if (!ctx) {
      throw new Error("Canvas 2D context unavailable.");
    }
    ctx.drawImage(bitmap, 0, 0, width, height);

    return await new Promise<Blob>((resolve, reject) => {
      canvas.toBlob(
        (blob) =>
          blob
            ? resolve(blob)
            : reject(new Error("Failed to encode resized image.")),
        "image/jpeg",
        0.85,
      );
    });
  } finally {
    bitmap.close();
  }
}
