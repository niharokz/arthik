// API client with the offline queue.
//
// Reads go to the network; when offline the service worker answers from its cache.
// Transaction changes made offline are queued in localStorage and replayed in
// order once the connection is back. Creates carry a client-made id, and the
// server treats a repeated create with the same id as already done, so a replay
// can never double-post.

const QKEY = "arthik.queue.v1";
const listeners = new Set();

export class ApiError extends Error {
  constructor(msg, status) { super(msg); this.status = status; }
}

function load() {
  try { return JSON.parse(localStorage.getItem(QKEY) || "[]"); } catch { return []; }
}
function save(q) {
  try { localStorage.setItem(QKEY, JSON.stringify(q)); } catch { /* storage full or blocked */ }
  listeners.forEach((fn) => fn(q.length));
}

/** Pending offline changes. */
export const queue = () => load();
export const onQueue = (fn) => { listeners.add(fn); return () => listeners.delete(fn); };

async function raw(method, url, body, isText) {
  const opt = { method, headers: { "X-Arthik": "1" }, credentials: "same-origin" };
  if (body !== undefined) {
    opt.body = isText ? body : JSON.stringify(body);
    opt.headers["Content-Type"] = isText ? "text/csv" : "application/json";
  }
  const res = await fetch(url, opt);
  const ct = res.headers.get("Content-Type") || "";
  const data = ct.includes("json") ? await res.json() : await res.text();
  if (!res.ok) {
    if (res.status === 401) window.dispatchEvent(new CustomEvent("arthik:signedout"));
    throw new ApiError((data && data.error) || res.statusText, res.status);
  }
  return data;
}

const isNetwork = (e) => e instanceof TypeError; // fetch rejects with TypeError when offline

export const get = (url) => raw("GET", url);
export const post = (url, body) => raw("POST", url, body);
export const put = (url, body) => raw("PUT", url, body);
export const del = (url) => raw("DELETE", url);
export const postText = (url, text) => raw("POST", url, text, true);

/**
 * Transaction writes that may be queued when offline.
 * Returns the server reply, or {queued: true} when stored for later.
 */
export async function txWrite(method, url, body) {
  if (!navigator.onLine) return enqueue(method, url, body);
  if (load().length) { // keep order behind earlier offline changes
    const r = enqueue(method, url, body);
    await flush();
    return load().length ? r : { ok: true, flushed: true };
  }
  try {
    return await raw(method, url, body);
  } catch (e) {
    if (isNetwork(e)) return enqueue(method, url, body);
    throw e;
  }
}

function enqueue(method, url, body) {
  const q = load();
  q.push({ method, url, body, at: Date.now() });
  save(q);
  return { queued: true };
}

let flushing = false;
/** Replays queued changes in order. Stops at the first network failure. */
export async function flush() {
  if (flushing || !navigator.onLine) return { sent: 0, failed: [] };
  flushing = true;
  let sent = 0;
  const failed = [];
  try {
    let q = load();
    while (q.length) {
      const item = q[0];
      try {
        await raw(item.method, item.url, item.body);
        sent++;
      } catch (e) {
        if (isNetwork(e)) break; // still offline: keep the rest
        if (e.status === 401) break; // signed out: keep for after sign-in
        failed.push({ item, error: e.message }); // rejected by the server: drop, but report
      }
      q = load().slice(1);
      save(q);
    }
  } finally {
    flushing = false;
  }
  return { sent, failed };
}
