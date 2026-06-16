import { afterEach, describe, expect, it, vi } from "vitest";

import {
  AI_DRAFT_ENDPOINT,
  emptyDraft,
  streamAiDraft,
  type ListingDraft,
  type StreamCallbacks,
} from "./ai-draft";

const encoder = new TextEncoder();

function streamingResponse(
  chunks: Uint8Array[],
  init: { ok?: boolean; status?: number } = {},
) {
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(chunk);
      controller.close();
    },
  });
  const response = {
    ok: init.ok ?? true,
    status: init.status ?? 200,
    body,
  } as unknown as Response;
  return stubFetch(response);
}

function ndjson(lines: string[], trailingNewline = true): Uint8Array {
  return encoder.encode(lines.join("\n") + (trailingNewline ? "\n" : ""));
}

function recordingCallbacks() {
  const meta = vi.fn();
  const delta = vi.fn();
  const done = vi.fn();
  const error = vi.fn();
  const callbacks: StreamCallbacks = {
    onMeta: meta,
    onDescriptionDelta: delta,
    onDone: done,
    onError: error,
  };
  return { callbacks, meta, delta, done, error };
}

function stubFetch(response: Response) {
  const fetchMock = vi.fn(
    (_input: RequestInfo | URL, _init?: RequestInit): Promise<Response> =>
      Promise.resolve(response),
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

const DRAFT: ListingDraft = {
  ...emptyDraft(),
  category_suggestion: "mobile_phones",
  title: { en: "Phone", si: "දුරකථනය", ta: "தொலைபேசி" },
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("streamAiDraft", () => {
  it("dispatches well-formed frames to the matching callbacks", async () => {
    streamingResponse([
      ndjson([
        JSON.stringify({
          type: "meta",
          category_suggestion: "mobile_phones",
          title: DRAFT.title,
          needs_human_review: false,
          review_note: "",
        }),
        JSON.stringify({ type: "description_delta", lang: "en", delta: "Hi " }),
        JSON.stringify({ type: "description_delta", lang: "en", delta: "there" }),
        JSON.stringify({ type: "done", draft: DRAFT }),
      ]),
    ]);
    const { callbacks, meta, delta, done, error } = recordingCallbacks();

    await streamAiDraft(new FormData(), callbacks);

    expect(meta).toHaveBeenCalledTimes(1);
    expect(meta.mock.calls[0][0].category_suggestion).toBe("mobile_phones");
    expect(delta).toHaveBeenCalledTimes(2);
    expect(delta).toHaveBeenNthCalledWith(1, "en", "Hi ");
    expect(delta).toHaveBeenNthCalledWith(2, "en", "there");
    expect(done).toHaveBeenCalledTimes(1);
    expect(done).toHaveBeenCalledWith(DRAFT);
    expect(error).not.toHaveBeenCalled();
  });

  it("reassembles a frame split across multiple reads", async () => {
    const line =
      JSON.stringify({ type: "description_delta", lang: "ta", delta: "x" }) +
      "\n";
    const bytes = encoder.encode(line);
    const cut = 8;
    streamingResponse([
      bytes.slice(0, cut),
      bytes.slice(cut),
    ]);
    const { callbacks, delta } = recordingCallbacks();

    await streamAiDraft(new FormData(), callbacks);

    expect(delta).toHaveBeenCalledTimes(1);
    expect(delta).toHaveBeenCalledWith("ta", "x");
  });

  it("reassembles a multi-byte UTF-8 character split across reads", async () => {
    const line =
      JSON.stringify({ type: "description_delta", lang: "si", delta: "ක" }) +
      "\n";
    const bytes = encoder.encode(line);
    const cut = bytes.length - 4;
    streamingResponse([
      bytes.slice(0, cut),
      bytes.slice(cut),
    ]);
    const { callbacks, delta, error } = recordingCallbacks();

    await streamAiDraft(new FormData(), callbacks);

    expect(error).not.toHaveBeenCalled();
    expect(delta).toHaveBeenCalledTimes(1);
    expect(delta).toHaveBeenCalledWith("si", "ක");
  });

  it("skips malformed lines without aborting the stream", async () => {
    streamingResponse([
      ndjson([
        "this is not json",
        JSON.stringify({ type: "description_delta", lang: "en", delta: "ok" }),
        "{ also: not, valid }",
        JSON.stringify({ type: "done", draft: DRAFT }),
      ]),
    ]);
    const { callbacks, delta, done, error } = recordingCallbacks();

    await streamAiDraft(new FormData(), callbacks);

    expect(delta).toHaveBeenCalledTimes(1);
    expect(delta).toHaveBeenCalledWith("en", "ok");
    expect(done).toHaveBeenCalledTimes(1);
    expect(error).not.toHaveBeenCalled();
  });

  it("ignores blank lines between frames", async () => {
    streamingResponse([
      ndjson([
        "",
        JSON.stringify({ type: "description_delta", lang: "en", delta: "a" }),
        "   ",
        JSON.stringify({ type: "done", draft: DRAFT }),
      ]),
    ]);
    const { callbacks, delta, done } = recordingCallbacks();

    await streamAiDraft(new FormData(), callbacks);

    expect(delta).toHaveBeenCalledTimes(1);
    expect(delta).toHaveBeenCalledWith("en", "a");
    expect(done).toHaveBeenCalledTimes(1);
  });

  it("flushes a trailing line that has no newline terminator", async () => {
    streamingResponse([
      ndjson(
        [JSON.stringify({ type: "done", draft: DRAFT })],
        /* trailingNewline */ false,
      ),
    ]);
    const { callbacks, done } = recordingCallbacks();

    await streamAiDraft(new FormData(), callbacks);

    expect(done).toHaveBeenCalledTimes(1);
    expect(done).toHaveBeenCalledWith(DRAFT);
  });

  it("routes an error frame to onError without throwing", async () => {
    streamingResponse([
      ndjson([JSON.stringify({ type: "error", message: "model unavailable" })]),
    ]);
    const { callbacks, error, done } = recordingCallbacks();

    await expect(streamAiDraft(new FormData(), callbacks)).resolves.toBeUndefined();
    expect(error).toHaveBeenCalledTimes(1);
    expect(error).toHaveBeenCalledWith("model unavailable");
    expect(done).not.toHaveBeenCalled();
  });

  it("calls onError and throws on a non-ok response", async () => {
    streamingResponse([], { ok: false, status: 503 });
    const { callbacks, error } = recordingCallbacks();

    await expect(streamAiDraft(new FormData(), callbacks)).rejects.toThrow(
      /503/,
    );
    expect(error).toHaveBeenCalledTimes(1);
    expect(error).toHaveBeenCalledWith("Draft request failed (503)");
  });

  it("calls onError and throws when the response has no body", async () => {
    stubFetch({
      ok: true,
      status: 200,
      body: null,
    } as unknown as Response);
    const { callbacks, error } = recordingCallbacks();

    await expect(streamAiDraft(new FormData(), callbacks)).rejects.toThrow(
      /200/,
    );
    expect(error).toHaveBeenCalledTimes(1);
  });

  it("forwards the abort signal to fetch and propagates an aborted fetch", async () => {
    const controller = new AbortController();
    controller.abort();
    const fetchMock = vi.fn(
      (_input: RequestInfo | URL, _init?: RequestInit): Promise<Response> => {
        throw new DOMException("The operation was aborted.", "AbortError");
      },
    );
    vi.stubGlobal("fetch", fetchMock);
    const { callbacks } = recordingCallbacks();

    await expect(
      streamAiDraft(new FormData(), callbacks, controller.signal),
    ).rejects.toMatchObject({ name: "AbortError" });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][1]).toMatchObject({
      method: "POST",
      signal: controller.signal,
    });
  });

  it("posts to the default endpoint with the supplied FormData", async () => {
    const fetchMock = streamingResponse([
      ndjson([JSON.stringify({ type: "done", draft: DRAFT })]),
    ]);
    const formData = new FormData();
    formData.set("keywords", "phone");
    const { callbacks } = recordingCallbacks();

    await streamAiDraft(formData, callbacks);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe(AI_DRAFT_ENDPOINT);
    expect(fetchMock.mock.calls[0][1]).toMatchObject({
      method: "POST",
      body: formData,
    });
  });

  it("targets an explicit endpoint override", async () => {
    const fetchMock = streamingResponse([
      ndjson([JSON.stringify({ type: "done", draft: DRAFT })]),
    ]);
    const { callbacks } = recordingCallbacks();

    await streamAiDraft(new FormData(), callbacks, undefined, "/custom/path");

    expect(fetchMock.mock.calls[0][0]).toBe("/custom/path");
  });
});
