// Shared UI pieces: the app store, lookups, toasts, sheets (dialogs), confirm and
// the period switcher used by categories, budgets and reports.

import { html, raw, esc, todayStr } from "./util.js";
import { get } from "./api.js";
import { syncBars, haptic, applyAccent } from "./native.js";

/** App-wide state, loaded from GET /api/state. */
export const S = {
  state: null,
  me: null,
  view: {}, // per-view remembered settings (period kind etc.)
};

export async function loadState() {
  S.state = await get("/api/state");
  const st = S.state.settings || {};
  document.documentElement.dataset.theme = st.theme && st.theme !== "auto" ? st.theme : "";
  if (st.theme === "auto") delete document.documentElement.dataset.theme;
  document.body.classList.toggle("hide-amounts", !!st.hide_amounts);
  applyAccent();
  requestAnimationFrame(syncBars); // phone app: match status bar to the theme
  return S.state;
}

export const ro = () => !!(S.state && S.state.read_only);
export const today = () => (S.state && S.state.today) || todayStr();

// ---- lookups ----
export const acc = (id) => (S.state.accounts || []).find((a) => a.id === id);
export const cat = (id) => (S.state.categories || []).find((c) => c.id === id);
export const label = (id) => (S.state.labels || []).find((l) => l.id === id);
export const accName = (id) => acc(id)?.name || id || "—";
export function catName(id, full = false) {
  const c = cat(id);
  if (!c) return id || "—";
  if (full && c.parent) return `${cat(c.parent)?.name || c.parent} › ${c.name}`;
  return c.name;
}
export const catIcon = (id) => { const c = cat(id); return c?.icon || (c?.parent ? cat(c.parent)?.icon : "") || "🏷️"; };
export const accIcon = (id) => acc(id)?.icon || "🏦";
export const labelName = (id) => label(id)?.name || id;
export const topCats = (kind) => (S.state.categories || []).filter((c) => !c.parent && (!kind || c.kind === kind || (kind !== "both" && c.kind === "both")));
export const kids = (id) => (S.state.categories || []).filter((c) => c.parent === id);
export const activeAccounts = () => (S.state.accounts || []).filter((a) => !a.archived);

/** <option>s of categories grouped parent → children, for a transaction kind. */
export function catOptions(kind, selected) {
  const parents = (S.state.categories || []).filter((c) => !c.parent && (c.kind === kind || c.kind === "both"));
  const opt = (c, child) => html`<option value="${c.id}" ${c.id === selected ? raw("selected") : ""}>${child ? "   " : ""}${c.icon ? c.icon + " " : ""}${c.name}</option>`;
  return parents.map((p) => [opt(p, false), kids(p.id).map((k) => opt(k, true))]);
}

/** <option>s of accounts grouped by type. */
export function accOptions(selected, { includeArchived = false } = {}) {
  const types = S.state.account_types || [];
  return types.map((t) => {
    const list = (S.state.accounts || []).filter((a) => a.type === t.id && (includeArchived || !a.archived || a.id === selected));
    if (!list.length) return "";
    return html`<optgroup label="${t.label}">${list.map((a) => html`<option value="${a.id}" ${a.id === selected ? raw("selected") : ""}>${a.name}</option>`)}</optgroup>`;
  });
}

// ---- toast ----
let toastT;
export function toast(msg, err = false) {
  if (err) haptic("error");
  let el = document.querySelector(".toast");
  if (!el) { el = document.createElement("div"); el.className = "toast"; el.setAttribute("role", "status"); document.body.append(el); }
  el.textContent = msg;
  el.classList.toggle("err", err);
  el.hidden = false;
  el.classList.remove("in"); void el.offsetWidth; el.classList.add("in"); // replay slide-in
  clearTimeout(toastT);
  toastT = setTimeout(() => (el.hidden = true), err ? 5000 : 2600);
}

// ---- sheets ----

/**
 * Opens a bottom sheet (centered dialog on desktop).
 * body: html; actions: html for the footer. onMount(dialogEl) wires events.
 * Returns {el, close}.
 */
export function sheet(title, body, actions = "", onMount) {
  const d = document.createElement("dialog");
  d.className = "sheet";
  d.innerHTML = String(html`<div class="sheet-in">
    <div class="sheet-h"><h2>${title}</h2><button class="ghost icon" data-close aria-label="Close">✕</button></div>
    <div class="sheet-b">${body}</div>
    ${actions ? html`<div class="sheet-f">${actions}</div>` : ""}
  </div>`);
  document.body.append(d);
  let closing = false;
  const close = () => { // slide out, then remove
    if (closing) return;
    closing = true;
    d.classList.add("closing");
    const done = () => { if (d.open) d.close(); d.remove(); };
    if (matchMedia("(prefers-reduced-motion: reduce)").matches) return done();
    d.addEventListener("animationend", done, { once: true });
    setTimeout(done, 400); // safety net if no animation runs
  };
  d._close = close; // used by the Android back button
  d.addEventListener("click", (e) => {
    if (e.target === d || e.target.closest("[data-close]")) close();
  });
  d.addEventListener("cancel", (e) => { e.preventDefault(); close(); });
  d.showModal();
  dragToClose(d, close);
  onMount && onMount(d, close);
  const first = d.querySelector("[autofocus]");
  if (first && matchMedia("(min-width: 700px)").matches) first.focus();
  return { el: d, close };
}

// Phones: pull a sheet down by its header to close it; a short pull springs back.
function dragToClose(d, close) {
  const head = d.querySelector(".sheet-h");
  let y0 = null, dy = 0;
  head.addEventListener("pointerdown", (e) => {
    if (e.target.closest("button") || matchMedia("(min-width: 700px)").matches) return;
    y0 = e.clientY; dy = 0;
    head.setPointerCapture(e.pointerId);
    d.style.transition = "none";
  });
  head.addEventListener("pointermove", (e) => {
    if (y0 === null) return;
    dy = Math.max(0, e.clientY - y0);
    d.style.transform = `translateY(${dy}px)`;
  });
  const end = () => {
    if (y0 === null) return;
    y0 = null;
    d.style.transition = "transform .22s cubic-bezier(.2, .8, .2, 1)";
    if (dy > Math.min(120, d.offsetHeight / 3)) { haptic("tick"); close(); } else d.style.transform = "";
  };
  head.addEventListener("pointerup", end);
  head.addEventListener("pointercancel", end);
}

/** Promise-based confirm in a sheet. */
export function confirmSheet(title, text, okLabel = "Delete", danger = true) {
  return new Promise((resolve) => {
    let done = false;
    const s = sheet(title, html`<p>${text}</p>`,
      html`<button data-close class="grow">Cancel</button><button class="${danger ? "danger" : "primary"} grow" data-ok>${okLabel}</button>`,
      (d, close) => {
        d.querySelector("[data-ok]").addEventListener("click", () => { done = true; close(); resolve(true); });
        d.addEventListener("close", () => { if (!done) resolve(false); });
      });
    return s;
  });
}

// ---- colour picker ----

/** Ready-made colours that read well in light and dark mode. */
export const SWATCHES = ["#3a807a", "#0e7c86", "#2f6fb0", "#5b5fc7", "#8a5cc2", "#b4469b",
  "#c2462b", "#d9822b", "#b77700", "#6f8f2f", "#13895f", "#64748b"];

/** "#abc", "abc", "#AABBCC" → "#aabbcc"; "" when it isn't a colour. */
export function hexColor(v) {
  let h = String(v || "").trim().replace(/^#/, "").toLowerCase();
  if (/^[0-9a-f]{3}$/.test(h)) h = [...h].map((c) => c + c).join("");
  return /^[0-9a-f]{6}$/.test(h) ? "#" + h : "";
}

/** A colour field: swatches to tap, a hex code to type and the system picker. */
export function colorField(name, value, label = "Colour") {
  const v = hexColor(value) || SWATCHES[0];
  return html`<div class="f color-f" data-color>
    <span class="flabel">${label}</span>
    <div class="sw-row" role="radiogroup" aria-label="${label}">${SWATCHES.map((c) =>
      html`<button type="button" class="sw ${c === v ? "on" : ""}" style="--c:${c}" data-sw="${c}" role="radio" aria-checked="${c === v}" aria-label="${c}"></button>`)}</div>
    <div class="hex-row">
      <span class="hex-prev" style="--c:${v}"></span>
      <span class="hash">#</span><input class="hex" value="${v.slice(1)}" maxlength="7" spellcheck="false" autocomplete="off" autocapitalize="off" aria-label="Hex code">
      <label class="native" title="More colours"><input type="color" value="${v}" aria-label="More colours">🎨</label>
      <input type="hidden" name="${name}" value="${v}">
    </div>
  </div>`;
}

/** Wires every colour field inside root (also ones re-rendered later); each change fires a "color" event. */
export function wireColorFields(root) {
  if (root._colors) return;
  root._colors = true;
  const set = (f, hex, from) => {
    f.querySelector("input[type=hidden]").value = hex;
    f.querySelector(".hex-prev").style.setProperty("--c", hex);
    if (from !== "hex") f.querySelector(".hex").value = hex.slice(1);
    if (from !== "native") f.querySelector("input[type=color]").value = hex;
    f.querySelector(".hex").classList.remove("bad");
    f.querySelectorAll("[data-sw]").forEach((b) => { const on = b.dataset.sw === hex; b.classList.toggle("on", on); b.setAttribute("aria-checked", on); });
    f.dispatchEvent(new CustomEvent("color", { bubbles: true, detail: hex })); // for live previews
  };
  root.addEventListener("click", (e) => {
    const b = e.target.closest("[data-sw]");
    if (b) { set(b.closest("[data-color]"), b.dataset.sw); haptic("tick"); }
  });
  root.addEventListener("input", (e) => {
    const f = e.target.closest("[data-color]");
    if (!f) return;
    if (e.target.classList.contains("hex")) {
      const hex = hexColor(e.target.value);
      if (hex) set(f, hex, "hex"); else e.target.classList.add("bad");
    } else if (e.target.type === "color") set(f, e.target.value, "native");
  });
  root.addEventListener("focusout", (e) => { // tidy "abc" into "aabbcc" when leaving the box
    if (!e.target.classList?.contains("hex")) return;
    const f = e.target.closest("[data-color]");
    e.target.value = f.querySelector("input[type=hidden]").value.slice(1);
    e.target.classList.remove("bad");
  });
  root.addEventListener("paste", (e) => { // pasting "#a1b2c3" into the hex box
    if (!e.target.classList?.contains("hex")) return;
    const hex = hexColor(e.clipboardData?.getData("text"));
    if (hex) { e.preventDefault(); set(e.target.closest("[data-color]"), hex); }
  });
}

/** Reads a form's named fields into an object (checkboxes → booleans). */
export function formData(root) {
  const o = {};
  root.querySelectorAll("[name]").forEach((el) => {
    if (el.type === "checkbox") o[el.name] = el.checked;
    else if (el.type === "radio") { if (el.checked) o[el.name] = el.value; }
    else o[el.name] = el.value;
  });
  return o;
}

// ---- period switcher ----

export const PERIODS = [
  ["weekly", "Week"], ["monthly", "Month"], ["quarterly", "Quarter"],
  ["halfyearly", "Half-year"], ["yearly", "Year"], ["custom", "Custom"],
];

/** Period state for a view: {period, date, from, to}. */
export function periodState(key) {
  if (!S.view[key]) S.view[key] = { period: S.state.settings.default_period || "monthly", date: today(), from: "", to: "" };
  return S.view[key];
}

export const periodQuery = (p) => (p.period === "custom" ? { period: "custom", from: p.from, to: p.to } : { period: p.period, date: p.date });

/** Renders the switcher; range = server's {label, from, to}; prev/next = shifted ranges. */
export function periodBar(p, range) {
  return html`<div class="period" data-period>
    <select data-pkind aria-label="Period">${PERIODS.map(([v, l]) => html`<option value="${v}" ${p.period === v ? raw("selected") : ""}>${l}</option>`)}</select>
    ${p.period === "custom"
      ? html`<input type="date" data-pfrom value="${p.from || range?.from || ""}" aria-label="From"> – <input type="date" data-pto value="${p.to || range?.to || ""}" aria-label="To">`
      : html`<button class="icon" data-pstep="-1" aria-label="Previous">‹</button><span class="label">${range?.label || ""}</span><button class="icon" data-pstep="1" aria-label="Next">›</button>
        ${range && !(range.from <= today() && today() <= range.to) ? html`<button class="ghost" data-ptoday>Today</button>` : ""}`}
  </div>`;
}

/** Wires a periodBar inside root; prev/next ranges come from the server reply. */
export function wirePeriod(root, p, reply, rerender) {
  const bar = root.querySelector("[data-period]");
  if (!bar) return;
  bar.querySelector("[data-pkind]").addEventListener("change", (e) => {
    p.period = e.target.value;
    if (p.period === "custom") { p.from = p.from || reply?.range?.from; p.to = p.to || reply?.range?.to; }
    rerender();
  });
  bar.querySelectorAll("[data-pstep]").forEach((b) => b.addEventListener("click", () => {
    const r = b.dataset.pstep === "-1" ? reply.prev : reply.next;
    if (r) p.date = r.from;
    rerender();
  }));
  bar.querySelector("[data-ptoday]")?.addEventListener("click", () => { p.date = today(); rerender(); });
  const f = bar.querySelector("[data-pfrom]"), t = bar.querySelector("[data-pto]");
  const upd = () => { if (f.value && t.value) { p.from = f.value; p.to = t.value; rerender(); } };
  f?.addEventListener("change", upd);
  t?.addEventListener("change", upd);
}

// ---- segmented controls: a thumb that glides to the chosen option ----

function placeThumb(seg, target, animate) {
  const on = target || seg.querySelector(":scope > button.on");
  let t = seg.querySelector(":scope > .thumb");
  if (!on || !on.offsetWidth) { t?.remove(); return; }
  if (!t) { t = document.createElement("i"); t.className = "thumb"; seg.prepend(t); animate = false; }
  const pos = `${on.offsetLeft},${on.offsetTop},${on.offsetWidth},${on.offsetHeight}`;
  if (t.dataset.pos === pos) return; // already there (or gliding there): leave it alone
  t.dataset.pos = pos;
  t.classList.toggle("anim", !!animate && !matchMedia("(prefers-reduced-motion: reduce)").matches);
  t.style.setProperty("--tx", on.offsetLeft + "px");
  t.style.setProperty("--ty", on.offsetTop + "px");
  t.style.setProperty("--tw", on.offsetWidth + "px");
  t.style.setProperty("--th", on.offsetHeight + "px");
}

/** Keeps every segmented control's thumb under its chosen option (call once). */
export function watchSegments() {
  let queued = false;
  const all = () => { queued = false; document.querySelectorAll(".seg").forEach((s) => placeThumb(s, null, false)); };
  const soon = () => { if (!queued) { queued = true; requestAnimationFrame(all); } };
  new MutationObserver(soon).observe(document.body, { childList: true, subtree: true });
  addEventListener("resize", soon);
  // glide to the tapped option at once, before the view re-renders
  document.addEventListener("click", (e) => {
    const b = e.target.closest(".seg > button");
    if (b && !b.disabled) placeThumb(b.parentElement, b, true);
  });
  all();
}

/** Shimmering placeholder cards while a view loads. */
export const loading = () => html`<div class="skeleton" aria-label="Loading"><div class="sk sk-row"></div><div class="sk sk-card"></div><div class="sk sk-card"></div><div class="sk sk-card short"></div></div>`;

/** Closes the top-most open sheet; returns false when none is open. */
export function closeTopSheet() {
  const open = [...document.querySelectorAll("dialog.sheet[open]")].pop();
  if (!open) return false;
  open._close ? open._close() : open.close();
  return true;
}
export const errorBox = (e) => html`<div class="card"><p class="neg">${e.message || e}</p></div>`;

export { esc };
