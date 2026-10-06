// Small helpers shared by every view: safe HTML templating, money and date formatting.

const ESC = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };
export const esc = (v) => String(v ?? "").replace(/[&<>"']/g, (c) => ESC[c]);

/** Raw HTML marker: html`` leaves it unescaped. */
export class Raw { constructor(s) { this.s = s; } toString() { return this.s; } }
export const raw = (s) => new Raw(s);

/** Tagged template that escapes every interpolation (arrays are joined). */
export function html(strings, ...vals) {
  let out = strings[0];
  vals.forEach((v, i) => {
    out += render(v) + strings[i + 1];
  });
  return raw(out);
}
function render(v) {
  if (v instanceof Raw) return v.s;
  if (Array.isArray(v)) return v.map(render).join("");
  if (v === false || v === null || v === undefined) return "";
  return esc(v);
}

// ---- money (amounts travel as decimal strings; maths is done in paise) ----

export const paise = (s) => {
  if (s === null || s === undefined || s === "") return 0;
  const n = Number(String(s).replace(/[₹,\s]/g, ""));
  return Number.isFinite(n) ? Math.round(n * 100) : 0;
};
export const fromPaise = (p) => (p < 0 ? "-" : "") + (Math.abs(p) / 100).toFixed(2);

const inr = new Intl.NumberFormat("en-IN", { style: "currency", currency: "INR", minimumFractionDigits: 2, maximumFractionDigits: 2 });
const inr0 = new Intl.NumberFormat("en-IN", { style: "currency", currency: "INR", maximumFractionDigits: 0 });

/** ₹1,23,456.78 */
export const money = (s) => inr.format(paise(s) / 100);
/** ₹1,23,457 (charts, compact places) */
export const money0 = (s) => inr0.format(Math.round(paise(s) / 100));
/** ₹1.2L / ₹3.4Cr / ₹12K for axis labels */
export function short(p) {
  const v = Math.abs(p) / 100, sign = p < 0 ? "-" : "";
  if (v >= 1e7) return `${sign}₹${(v / 1e7).toFixed(v >= 1e8 ? 0 : 1)}Cr`;
  if (v >= 1e5) return `${sign}₹${(v / 1e5).toFixed(v >= 1e6 ? 0 : 1)}L`;
  if (v >= 1e3) return `${sign}₹${(v / 1e3).toFixed(v >= 1e4 ? 0 : 1)}K`;
  return `${sign}₹${Math.round(v)}`;
}

/** An amount span: colour by sign when signed=true. */
export function amt(s, { signed = false, cls = "" } = {}) {
  const p = paise(s);
  const c = signed ? (p > 0 ? "pos" : p < 0 ? "neg" : "") : "";
  const text = signed && p > 0 ? "+" + money(s) : money(s);
  return html`<span class="amt ${c} ${cls}">${text}</span>`;
}

/**
 * Evaluates a typed amount: digits, . , + - * / ( ) — Bluecoins' calculator.
 * Returns paise or NaN. No eval (the page's CSP forbids it anyway).
 */
export function calc(src) {
  const s = String(src).replace(/[₹,\s]/g, "").replace(/×/g, "*").replace(/÷/g, "/");
  if (!s) return NaN;
  let i = 0;
  const peek = () => s[i];
  function num() {
    if (peek() === "(") { i++; const v = expr(); if (s[i++] !== ")") throw 0; return v; }
    if (peek() === "-") { i++; return -num(); }
    const m = /^\d*\.?\d+|^\d+\./.exec(s.slice(i));
    if (!m) throw 0;
    i += m[0].length;
    return parseFloat(m[0]);
  }
  function term() {
    let v = num();
    while (peek() === "*" || peek() === "/") { const op = s[i++]; const r = num(); v = op === "*" ? v * r : v / r; }
    return v;
  }
  function expr() {
    let v = term();
    while (peek() === "+" || peek() === "-") { const op = s[i++]; const r = term(); v = op === "+" ? v + r : v - r; }
    return v;
  }
  try {
    const v = expr();
    if (i !== s.length || !Number.isFinite(v)) return NaN;
    return Math.round(v * 100);
  } catch { return NaN; }
}

// ---- dates (YYYY-MM-DD strings, local calendar) ----

export const pad = (n) => String(n).padStart(2, "0");
export const ymd = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
export const parseDate = (s) => { const [y, m, d] = String(s).slice(0, 10).split("-").map(Number); return new Date(y, (m || 1) - 1, d || 1); };
export const addDays = (s, n) => { const d = parseDate(s); d.setDate(d.getDate() + n); return ymd(d); };
export const todayStr = () => ymd(new Date());
export const nowTime = () => { const d = new Date(); return `${pad(d.getHours())}:${pad(d.getMinutes())}`; };

const dayFmt = new Intl.DateTimeFormat("en-IN", { weekday: "short", day: "numeric", month: "short", year: "numeric" });
const shortFmt = new Intl.DateTimeFormat("en-IN", { day: "numeric", month: "short" });
const longFmt = new Intl.DateTimeFormat("en-IN", { day: "numeric", month: "short", year: "numeric" });
export const fmtDay = (s) => dayFmt.format(parseDate(s));
export const fmtShort = (s) => shortFmt.format(parseDate(s));
export const fmtDate = (s) => (s ? longFmt.format(parseDate(s)) : "");

/** "today", "tomorrow", "in 5 days", "3 days ago" */
export function relDay(s, today = todayStr()) {
  const n = Math.round((parseDate(s) - parseDate(today)) / 864e5);
  if (n === 0) return "today";
  if (n === 1) return "tomorrow";
  if (n === -1) return "yesterday";
  return n > 0 ? `in ${n} days` : `${-n} days ago`;
}

/** Client-side id for offline-created records (accepted by the server as-is). */
export function newId(date) {
  const r = crypto.getRandomValues(new Uint8Array(4));
  return "c-" + String(date || todayStr()).replaceAll("-", "") + "-" + [...r].map((b) => b.toString(16).padStart(2, "0")).join("");
}

export const debounce = (fn, ms = 250) => {
  let t;
  return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); };
};

export const qs = (o) => {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(o)) {
    if (v === undefined || v === null || v === "" || (Array.isArray(v) && !v.length)) continue;
    p.set(k, Array.isArray(v) ? v.join(",") : v);
  }
  const s = p.toString();
  return s ? "?" + s : "";
};
