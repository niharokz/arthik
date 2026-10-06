// Shared UI pieces: the app store, lookups, toasts, sheets (dialogs), confirm and
// the period switcher used by categories, budgets and reports.

import { html, raw, esc, todayStr } from "./util.js";
import { get } from "./api.js";

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
  let el = document.querySelector(".toast");
  if (!el) { el = document.createElement("div"); el.className = "toast"; el.setAttribute("role", "status"); document.body.append(el); }
  el.textContent = msg;
  el.classList.toggle("err", err);
  el.hidden = false;
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
  const close = () => { d.close(); d.remove(); };
  d.addEventListener("click", (e) => {
    if (e.target === d || e.target.closest("[data-close]")) close();
  });
  d.addEventListener("cancel", (e) => { e.preventDefault(); close(); });
  d.showModal();
  onMount && onMount(d, close);
  const first = d.querySelector("[autofocus]");
  if (first && matchMedia("(min-width: 700px)").matches) first.focus();
  return { el: d, close };
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

/** Small loading placeholder. */
export const loading = () => html`<div class="empty">Loading…</div>`;
export const errorBox = (e) => html`<div class="card"><p class="neg">${e.message || e}</p></div>`;

export { esc };
