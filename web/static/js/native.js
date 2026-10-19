// Bridge to the Android app. In a normal browser window.ArthikNative is absent and
// every helper here is a harmless no-op, so the web app and the phone app share one UI.

const N = window.ArthikNative || null; // injected by the app's WebView

/** True when running inside the Arthik Android app. */
export const isApp = !!N;

/** Calls a native method; returns undefined in a browser or if it throws. */
export function call(name, ...args) {
  if (!N || typeof N[name] !== "function") return undefined;
  try { return N[name](...args); } catch (e) { console.warn("native", name, e); return undefined; }
}

/** App settings kept on the phone (folder, lock, …). */
export function config() {
  try { return JSON.parse(call("config") || "{}"); } catch { return {}; }
}

/** Parses a JSON reply from a native call. */
export function json(name, ...args) {
  try { return JSON.parse(call(name, ...args) || "{}"); } catch { return {}; }
}

// Native code reports async results (folder picked, unlock …) as events.
const handlers = new Set();
window.arthikNative = { onEvent: (e) => handlers.forEach((fn) => fn(typeof e === "string" ? JSON.parse(e) : e)) };

/** Subscribes to native events; returns an unsubscribe function. */
export function onNative(fn) { handlers.add(fn); return () => handlers.delete(fn); }

/** Light vibration feedback (confirm | error | tick). */
export const haptic = (kind = "tick") => call("haptic", kind);

/** Uses the phone's Material You accent when switched on in Settings → Phone. */
export function applyAccent() {
  if (!N) return;
  const a = json("accent");
  const root = document.documentElement;
  if (a.on && a.light && a.dark) {
    root.style.setProperty("--accent-l", a.light);
    root.style.setProperty("--accent-d", a.dark);
    root.dataset.accent = "you";
  } else {
    delete root.dataset.accent;
  }
}

/** Colours the Android status and navigation bars to match the page. */
export function syncBars() {
  if (!N) return;
  const bg = getComputedStyle(document.body).backgroundColor;
  const m = bg.match(/\d+/g) || [255, 255, 255];
  const hex = "#" + m.slice(0, 3).map((n) => (+n).toString(16).padStart(2, "0")).join("");
  const dark = (0.299 * m[0] + 0.587 * m[1] + 0.114 * m[2]) < 128;
  call("setBars", hex, dark);
}
