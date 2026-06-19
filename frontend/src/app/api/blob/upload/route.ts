import { handleUpload, type HandleUploadBody } from "@vercel/blob/client";
import { NextResponse } from "next/server";

// POST /api/blob/upload — issues a short-lived client-upload token for Vercel
// Blob (VER-376). The browser uploads the file directly to Blob with this
// token, then persists the returned public URL on the listing via the backend
// (POST /listings/{id}/images), which enforces listing ownership.
//
// Requires BLOB_READ_WRITE_TOKEN in the environment — auto-added when the
// `elamachan-listings` store is connected to this Vercel project. No S3/Fly
// secrets are involved.
const ALLOWED_CONTENT_TYPES = ["image/jpeg", "image/png", "image/webp"];
const MAX_IMAGE_BYTES = 8 * 1024 * 1024; // 8 MB

export async function POST(request: Request): Promise<NextResponse> {
  const body = (await request.json()) as HandleUploadBody;

  try {
    const jsonResponse = await handleUpload({
      body,
      request,
      onBeforeGenerateToken: async (pathname, clientPayload) => {
        // Constrain what the issued token can upload. The token is short-lived
        // and scoped to these content types / size; the listing-ownership check
        // happens server-side when the URL is attached to the listing.
        return {
          allowedContentTypes: ALLOWED_CONTENT_TYPES,
          maximumSizeInBytes: MAX_IMAGE_BYTES,
          addRandomSuffix: true,
          tokenPayload: clientPayload ?? undefined,
        };
      },
      // Persistence is done client-side after upload() resolves, so this is a
      // no-op. (onUploadCompleted does not fire for localhost uploads anyway.)
      onUploadCompleted: async () => {},
    });

    return NextResponse.json(jsonResponse);
  } catch (error) {
    return NextResponse.json(
      { error: (error as Error).message },
      { status: 400 }
    );
  }
}
