// Settings → Look and Settings → Alerts. Both live in settings.md, so every device
// (web and phone) looks the same and alerts at the same time. Look changes preview
// at once and are saved with one button; leaving without saving restores the page.

import { html, raw } from "../util.js";
import { S, toast, ro, applyLook, LOOK_TOKENS, tokenHex } from "../ui.js";
import { put } from "../api.js";
import { isApp, call } from "../native.js";

// Palette → [name, swatch colours for its preview].
const PALETTES = {
  nss: ["nss", ["#0d1412", "#5cbdb1", "#9fb3f8"]],
  amoled: ["AMOLED", ["#000000", "#5cbdb1", "#e6e9e8"]],
  galaxy: ["Galaxy", ["#111325", "#9fb3f8", "#c3b3f2"]],
  white: ["White", ["#ffffff", "#2f6f69", "#161a19"]],
  gold: ["Gold", ["#15120c", "#c9a45c", "#ece5d6"]],
  blackpink: ["Blackpink", ["#0b0a0b", "#e3a1b6", "#ece6e9"]],
};
const CARDS = { none: "None", glass: "Glass", matte: "Matte", opaque: "Opaque" };
const DENSITY = { comfortable: "Comfortable", compact: "Compact" };
const COLOR_NAMES = { accent: "Accent", bg: "Background", surface: "Cards", text: "Text", muted: "Secondary text", line: "Lines", income: "Income", expense: "Expense", warn: "Warning" };
const CHART_NAMES = { spending: "Spending (home)", budgets: "Budgets", cashflow: "Cash flow", networth: "Net worth", reports: "Reports: spending & income" };
const CHART_TYPES = { hbar: "Ranked bars", donut: "Donut", pie: "Pie", bar: "Columns", line: "Line", area: "Area" };
const MODES = [["auto", "Follow the device"], ["light", "Light"], ["dark", "Dark"]];

/** Saves part of the settings (the rest as they are). */
export async function saveSettings(patch, msg) {
  await put("/api/settings", { ...S.state.settings, ...patch });
  Object.assign(S.state.settings, patch);
  if (msg) toast(msg);
}

/** The Look card. */
export function lookCard() {
  const s = S.state.settings, l = s.look || {}, opt = S.state.look_options || {};
  const dis = ro() ? raw("disabled") : "";
  return html`<section class="card set-sec" id="set-look" data-look>
    <header><h2>Look</h2><small class="faint">same on every device</small></header>
    <span class="flabel">Theme</span>
    <div class="pal-row" role="radiogroup" aria-label="Theme">${(opt.palettes || Object.keys(PALETTES)).map((k) => html`
      <button type="button" class="pal ${k === (l.palette || "nss") ? "on" : ""}" data-pal="${k}" role="radio" aria-checked="${k === (l.palette || "nss")}" ${dis}>
        <span class="pal-sw">${PALETTES[k][1].map((c) => html`<i style="background:${c}"></i>`)}</span>${PALETTES[k][0]}</button>`)}</div>
    <div class="fields two">
      <label class="f" data-modef ${(l.palette || "nss") === "nss" ? "" : raw("hidden")}><span>nss mode</span><select data-mode ${dis}>${MODES.map(([k, n]) => html`<option value="${k}" ${k === s.theme ? raw("selected") : ""}>${n}</option>`)}</select></label>
      <div class="f"><span class="flabel">Card background</span>
        <div class="seg" data-cards>${(opt.cards || Object.keys(CARDS)).map((k) => html`<button type="button" data-card="${k}" class="${k === (l.cards || "matte") ? "on" : ""}" ${dis}>${CARDS[k]}</button>`)}</div></div>
      <div class="f"><span class="flabel">Density</span>
        <div class="seg" data-dens>${(opt.densities || Object.keys(DENSITY)).map((k) => html`<button type="button" data-den="${k}" class="${k === (l.density || "comfortable") ? "on" : ""}" ${dis}>${DENSITY[k]}</button>`)}</div></div>
    </div>
    <details class="set-more" ${l.colors && Object.keys(l.colors).length ? raw("open") : ""}><summary>Colours <small class="faint">change any element of the theme</small></summary>
      <div class="col-grid">${(opt.colors || Object.keys(COLOR_NAMES)).map((k) => html`<label class="col-f">
        <input type="color" data-col="${k}" value="${l.colors?.[k] || tokenHex(LOOK_TOKENS[k])}" ${dis}>
        <span class="grow">${COLOR_NAMES[k]}</span>
        <button type="button" class="ghost icon reset" data-uncol="${k}" title="Back to the theme's colour" ${l.colors?.[k] ? "" : raw("hidden")} ${dis}>↺</button></label>`)}</div></details>
    <details class="set-more"><summary>Charts <small class="faint">how each chart is drawn</small></summary>
      <div class="fields two">${Object.entries(opt.charts || {}).map(([k, kinds]) => html`<label class="f"><span>${CHART_NAMES[k] || k}</span>
        <select data-chart="${k}" ${dis}>${kinds.map((t) => html`<option value="${t}" ${(l.charts?.[k] || kinds[0]) === t ? raw("selected") : ""}>${CHART_TYPES[t] || t}</option>`)}</select></label>`)}</div></details>
    ${ro() ? html`<p class="muted">The demo is read-only.</p>` : html`<div class="row set-act"><button class="ghost" data-lookreset>Default look</button><span class="grow"></span><button class="primary" data-looksave disabled>Save look</button></div>`}
  </section>`;
}

/** Wires the Look card: preview on every change, save or restore. */
export function wireLook(root, ctx) {
  const card = root.querySelector("[data-look]");
  if (!card || ro()) return;
  const saved = { theme: S.state.settings.theme, look: structuredClone(S.state.settings.look || {}) };
  const draft = structuredClone(saved);
  const dirty = () => JSON.stringify(draft) !== JSON.stringify(saved);
  const preview = () => {
    applyLook({ ...S.state.settings, ...draft, preview: true }); // widgets change on save
    card.querySelector("[data-looksave]").disabled = !dirty();
    card.querySelectorAll("[data-col]").forEach((x) => { if (!draft.look.colors?.[x.dataset.col]) x.value = tokenHex(LOOK_TOKENS[x.dataset.col]); });
  };
  card.addEventListener("click", (e) => {
    const p = e.target.closest("[data-pal]"), c = e.target.closest("[data-card]"), u = e.target.closest("[data-uncol]"), n = e.target.closest("[data-den]");
    if (n) {
      draft.look.density = n.dataset.den;
      card.querySelectorAll("[data-den]").forEach((x) => x.classList.toggle("on", x === n));
    }
    if (p) {
      draft.look.palette = p.dataset.pal;
      card.querySelectorAll("[data-pal]").forEach((x) => { x.classList.toggle("on", x === p); x.setAttribute("aria-checked", x === p); });
      card.querySelector("[data-modef]").hidden = p.dataset.pal !== "nss";
    }
    if (c) {
      draft.look.cards = c.dataset.card;
      card.querySelectorAll("[data-card]").forEach((x) => x.classList.toggle("on", x === c));
    }
    if (u) {
      delete draft.look.colors?.[u.dataset.uncol];
      u.hidden = true;
    }
    if (p || c || u || n) preview();
  });
  card.addEventListener("input", (e) => {
    const k = e.target.dataset.col;
    if (!k) return;
    draft.look.colors = { ...(draft.look.colors || {}), [k]: e.target.value };
    card.querySelector(`[data-uncol="${k}"]`).hidden = false;
    preview();
  });
  card.addEventListener("change", (e) => {
    if (e.target.matches("[data-mode]")) { draft.theme = e.target.value; preview(); }
    if (e.target.matches("[data-chart]")) { draft.look.charts = { ...(draft.look.charts || {}), [e.target.dataset.chart]: e.target.value }; preview(); }
  });
  // Shows draft in the controls (after "Default look").
  const sync = () => {
    const pal = draft.look.palette;
    card.querySelectorAll("[data-pal]").forEach((x) => { x.classList.toggle("on", x.dataset.pal === pal); x.setAttribute("aria-checked", x.dataset.pal === pal); });
    card.querySelector("[data-modef]").hidden = pal !== "nss";
    card.querySelector("[data-mode]").value = draft.theme;
    card.querySelectorAll("[data-card]").forEach((x) => x.classList.toggle("on", x.dataset.card === draft.look.cards));
    card.querySelectorAll("[data-den]").forEach((x) => x.classList.toggle("on", x.dataset.den === draft.look.density));
    card.querySelectorAll("[data-uncol]").forEach((x) => (x.hidden = !draft.look.colors?.[x.dataset.uncol]));
    card.querySelectorAll("[data-chart]").forEach((x) => (x.value = draft.look.charts?.[x.dataset.chart] || x.options[0].value));
  };
  card.querySelector("[data-lookreset]").addEventListener("click", () => {
    draft.theme = "auto";
    draft.look = { palette: "nss", cards: "matte", density: "comfortable" };
    sync();
    preview();
  });
  card.querySelector("[data-looksave]").addEventListener("click", async () => {
    try {
      await saveSettings({ theme: draft.theme, look: draft.look }, "Look saved — every device follows");
      ctx.refresh();
    } catch (err) { toast(err.message, true); }
  });
  // Leaving unsaved: put the saved look back.
  addEventListener("hashchange", () => { if (dirty()) applyLook(S.state.settings); }, { once: true });
}

/** The Alerts card: the phone's once-a-day alert. */
export function alertsCard() {
  const a = S.state.settings.alerts || { time: "09:00", mismatch: true, count: 30 };
  const dis = ro() ? raw("disabled") : "";
  return html`<form class="card set-sec" id="set-alerts" data-alerts>
    <header><h2>Daily alert</h2><small class="faint">on the phone</small></header>
    <p class="muted set-note">Once a day the Arthik phone app tells you when something needs a look. Switch it on for each phone in <b>Settings → Phone</b>.</p>
    <div class="fields two">
      <label class="f"><span>Time</span><input type="time" name="time" value="${a.time}" required ${dis}></label>
      <label class="f"><span>When inbox + notifications are more than</span><input type="number" name="count" min="0" max="9999" value="${a.count}" ${dis}><small class="faint">0 = never</small></label>
    </div>
    <label class="check"><input type="checkbox" name="mismatch" ${a.mismatch ? raw("checked") : ""} ${dis}> When an account doesn't match the bank (❌)</label>
    ${ro() ? "" : html`<div class="row set-act"><span class="grow"></span><button class="primary" type="submit">Save alert</button></div>`}
  </form>`;
}

export function wireAlerts(root) {
  const f = root.querySelector("[data-alerts]");
  f?.addEventListener("submit", async (e) => {
    e.preventDefault();
    const alerts = { time: f.time.value, count: +f.count.value || 0, mismatch: f.mismatch.checked };
    try {
      await saveSettings({ alerts }, `Daily alert at ${alerts.time}`);
      if (isApp) { const [h, m] = alerts.time.split(":").map(Number); call("setNotifyTime", h, m); } // move this phone's alarm now
    } catch (err) { toast(err.message, true); }
  });
}
