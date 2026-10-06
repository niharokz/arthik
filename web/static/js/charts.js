// Hand-drawn SVG charts (no library): paired income/expense columns, a single-series
// line with a hover crosshair, single-series columns and ranked horizontal bars.
// Colours come from CSS tokens, so light/dark mode follows the theme. Every chart
// has a hover tooltip and a legend when it shows more than one series.

import { html, raw, paise, short, money } from "./util.js";

let tipEl;
function tip(e, text) {
  if (!tipEl) { tipEl = document.createElement("div"); tipEl.className = "tip"; document.body.append(tipEl); }
  if (!text) { tipEl.hidden = true; return; }
  tipEl.innerHTML = text;
  tipEl.hidden = false;
  const x = Math.min(e.clientX + 14, innerWidth - tipEl.offsetWidth - 8);
  const y = Math.max(8, e.clientY - tipEl.offsetHeight - 12);
  tipEl.style.left = x + "px";
  tipEl.style.top = y + "px";
}

// Charts are drawn at the real pixel width of their box (so text stays 11px on a
// phone and on a wide screen): views insert a placeholder, wireTips() draws it.
const pending = new Map();
let seq = 0;
function box(draw) {
  const id = "c" + ++seq;
  pending.set(id, draw);
  return html`<div class="chartbox" data-cid="${id}"></div>`;
}
export const incomeExpense = (rows, o) => box((w) => incomeExpenseSVG(w, rows, o));
export const line = (points, o) => (points.length ? box((w) => lineSVG(w, points, o)) : html`<div class="empty">No data</div>`);
export const columns = (rows, o) => box((w) => columnsSVG(w, rows, o));

function drawAll(root) {
  root.querySelectorAll(".chartbox[data-cid]").forEach((el) => {
    const draw = el._draw || pending.get(el.dataset.cid);
    if (!draw) return;
    el._draw = draw;
    pending.delete(el.dataset.cid);
    const w = Math.max(280, Math.round(el.clientWidth || 600));
    if (el._w === w) return;
    el._w = w;
    el.innerHTML = String(draw(w));
  });
}
let resizeT;
addEventListener("resize", () => { clearTimeout(resizeT); resizeT = setTimeout(() => drawAll(document), 150); });

/** Draws pending charts in root and attaches hover tooltips. */
export function wireTips(root) {
  drawAll(root);
  root.querySelectorAll(".chartbox").forEach((svg) => {
    if (svg._tips) return;
    svg._tips = true;
    svg.addEventListener("pointermove", (e) => {
      const t = e.target.closest("[data-tip]");
      tip(e, t ? t.dataset.tip : "");
      const i = t?.dataset.i;
      svg.querySelectorAll("[data-cross]").forEach((c) => (c.style.opacity = c.dataset.cross === i ? 1 : 0));
    });
    svg.addEventListener("pointerleave", (e) => {
      tip(e, "");
      svg.querySelectorAll("[data-cross]").forEach((c) => (c.style.opacity = 0));
    });
  });
}

function niceMax(v) {
  if (v <= 0) return 1;
  const p = Math.pow(10, Math.floor(Math.log10(v)));
  for (const m of [1, 1.2, 2, 3, 4, 6, 8, 10]) if (m * p >= v) return m * p;
  return 10 * p;
}

function ticks(min, max, n = 4) {
  const out = [];
  for (let i = 0; i <= n; i++) out.push(min + ((max - min) * i) / n);
  return out;
}

const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);

/**
 * Paired columns: rows = [{label, income, expense}] (decimal strings).
 */
function incomeExpenseSVG(W, rows, { height = 200 } = {}) {
  const H = height, L = 52, R = 8, T = 10, B = 26;
  const max = niceMax(Math.max(1, ...rows.flatMap((r) => [paise(r.income), paise(r.expense)])));
  const n = rows.length || 1, gw = (W - L - R) / n;
  const bw = Math.max(3, Math.min(18, gw / 2 - 3));
  const y = (v) => T + (H - T - B) * (1 - v / max);
  const step = Math.ceil(n / Math.max(2, Math.floor((W - L) / 52)));
  let g = "";
  for (const t of ticks(0, max)) g += `<line class="grid-l" x1="${L}" x2="${W - R}" y1="${y(t)}" y2="${y(t)}"/><text x="${L - 6}" y="${y(t) + 4}" text-anchor="end">${short(t)}</text>`;
  rows.forEach((r, i) => {
    const cx = L + gw * i + gw / 2;
    const inc = paise(r.income), exp = paise(r.expense);
    const tipText = `<b>${esc(r.label)}</b><br>Income ${esc(money(r.income))}<br>Expense ${esc(money(r.expense))}<br>Net ${esc(money(r.net ?? (inc - exp) / 100))}`;
    g += bar(cx - bw - 1, y(inc), bw, y(0) - y(inc), "inc");
    g += bar(cx + 1, y(exp), bw, y(0) - y(exp), "exp");
    g += `<rect class="hit" x="${L + gw * i}" y="${T}" width="${gw}" height="${H - T - B}" data-tip="${esc(tipText)}"/>`;
    if (i % step === 0) g += `<text x="${cx}" y="${H - 8}" text-anchor="middle">${esc(r.label.replace(/ \d{4}$/, ""))}</text>`;
  });
  g += `<line class="zero" x1="${L}" x2="${W - R}" y1="${y(0)}" y2="${y(0)}"/>`;
  return html`<svg class="chart" viewBox="0 0 ${W} ${H}" role="img" aria-label="Income and expense by period">${raw(g)}</svg>
    <div class="legend"><span><i class="inc"></i>Income</span><span><i class="exp"></i>Expense</span></div>`;
}

/** A column with rounded data end anchored on the baseline. */
function bar(x, y, w, h, cls) {
  if (h <= 0.5) return "";
  const r = Math.min(4, w / 2, h);
  return `<path class="${cls}" d="M${x},${y + h} V${y + r} Q${x},${y} ${x + r},${y} H${x + w - r} Q${x + w},${y} ${x + w},${y + r} V${y + h} Z"/>`;
}

/**
 * Single-series line: points = [{label, value}] (decimal strings). Can go negative.
 */
function lineSVG(W, points, { height = 200, name = "Balance" } = {}) {
  const H = height, L = 56, R = 10, T = 12, B = 26;
  if (!points.length) return html`<div class="empty">No data</div>`;
  const vals = points.map((p) => paise(p.value));
  let lo = Math.min(0, ...vals), hi = Math.max(0, ...vals);
  hi = hi > 0 ? niceMax(hi) : 0;
  lo = lo < 0 ? -niceMax(-lo) : 0;
  if (hi === lo) hi = lo + 100;
  const n = points.length;
  const x = (i) => (n === 1 ? (L + W - R) / 2 : L + ((W - L - R) * i) / (n - 1));
  const y = (v) => T + (H - T - B) * (1 - (v - lo) / (hi - lo));
  let g = "";
  for (const t of ticks(lo, hi)) g += `<line class="grid-l" x1="${L}" x2="${W - R}" y1="${y(t)}" y2="${y(t)}"/><text x="${L - 6}" y="${y(t) + 4}" text-anchor="end">${short(t)}</text>`;
  if (lo < 0) g += `<line class="zero" x1="${L}" x2="${W - R}" y1="${y(0)}" y2="${y(0)}"/>`;
  const d = vals.map((v, i) => `${i ? "L" : "M"}${x(i)},${y(v)}`).join(" ");
  g += `<path class="area" d="${d} L${x(n - 1)},${y(Math.max(lo, 0))} L${x(0)},${y(Math.max(lo, 0))} Z"/>`;
  g += `<path class="line" d="${d}"/>`;
  const step = Math.ceil(n / Math.max(2, Math.floor((W - L) / 56)));
  const gw = (W - L - R) / Math.max(1, n - 1);
  points.forEach((p, i) => {
    g += `<g data-cross="${i}" style="opacity:0"><line class="cross" x1="${x(i)}" x2="${x(i)}" y1="${T}" y2="${H - B}"/><circle class="dot" r="5" cx="${x(i)}" cy="${y(vals[i])}"/></g>`;
    g += `<rect class="hit" data-i="${i}" x="${x(i) - gw / 2}" y="${T}" width="${gw}" height="${H - T - B}" data-tip="${esc(`<b>${esc(p.label)}</b><br>${esc(name)} ${esc(money(p.value))}`)}"/>`;
    if (i % step === 0 || i === n - 1) g += `<text x="${x(i)}" y="${H - 8}" text-anchor="middle">${esc(p.label.replace(/ \d{4}$/, ""))}</text>`;
  });
  if (n <= 24) g += `<circle class="dot" r="4" cx="${x(n - 1)}" cy="${y(vals[n - 1])}"/>`;
  return html`<svg class="chart" viewBox="0 0 ${W} ${H}" role="img" aria-label="${name} over time">${raw(g)}</svg>`;
}

/** Single-series columns: rows = [{label, value}]. */
function columnsSVG(W, rows, { height = 180, name = "Total" } = {}) {
  const H = height, L = 52, R = 8, T = 10, B = 26;
  const max = niceMax(Math.max(1, ...rows.map((r) => Math.abs(paise(r.value)))));
  const n = rows.length || 1, gw = (W - L - R) / n, bw = Math.max(4, Math.min(28, gw - 6));
  const y = (v) => T + (H - T - B) * (1 - v / max);
  const step = Math.ceil(n / Math.max(2, Math.floor((W - L) / 52)));
  let g = "";
  for (const t of ticks(0, max)) g += `<line class="grid-l" x1="${L}" x2="${W - R}" y1="${y(t)}" y2="${y(t)}"/><text x="${L - 6}" y="${y(t) + 4}" text-anchor="end">${short(t)}</text>`;
  rows.forEach((r, i) => {
    const cx = L + gw * i + gw / 2, v = Math.max(0, paise(r.value));
    g += bar(cx - bw / 2, y(v), bw, y(0) - y(v), "one");
    g += `<rect class="hit" x="${L + gw * i}" y="${T}" width="${gw}" height="${H - T - B}" data-tip="${esc(`<b>${esc(r.label)}</b><br>${esc(name)} ${esc(money(r.value))}`)}"/>`;
    if (i % step === 0) g += `<text x="${cx}" y="${H - 8}" text-anchor="middle">${esc(r.label.replace(/ \d{4}$/, ""))}</text>`;
  });
  g += `<line class="zero" x1="${L}" x2="${W - R}" y1="${y(0)}" y2="${y(0)}"/>`;
  return html`<svg class="chart" viewBox="0 0 ${W} ${H}" role="img" aria-label="${name} by period">${raw(g)}</svg>`;
}

/**
 * Ranked horizontal bars (magnitude, one hue): rows = [{key, name, icon, value, note}].
 * Clicking a row calls data-key handlers set by the caller.
 */
export function ranked(rows, total) {
  const max = Math.max(1, ...rows.map((r) => paise(r.value)));
  return html`${rows.map((r) => {
    const v = paise(r.value);
    const pct = total ? Math.round((v * 1000) / paise(total)) / 10 : 0;
    return html`<div class="hbar" data-key="${r.key}" role="button" tabindex="0">
      <span class="ellipsis">${r.icon ? r.icon + " " : ""}${r.name} <small class="faint">${pct ? pct + "%" : ""}${r.note ? " · " + r.note : ""}</small></span>
      <span class="amt">${money(r.value)}</span>
      <div class="bar"><i style="width:${Math.max(1, (v / max) * 100).toFixed(1)}%"></i></div>
    </div>`;
  })}`;
}
