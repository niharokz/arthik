// Renders transactions as a day-grouped list (used by the transaction list, account
// and category detail, search and calendar views). The category and account names in a
// row open their screens; an account's list can show the balance after each row.

import { html, money, paise, fmtDay, relDay } from "./util.js";
import { S, accName, catName, catIcon, cat, today } from "./ui.js";
import { queue } from "./api.js";

/** The signed effect of a transaction: on one account when given, else natural. */
export function effect(t, account) {
  if (account) {
    const p = (t.postings || []).filter((x) => x.account === account).reduce((a, x) => a + paise(x.amount), 0);
    return p;
  }
  const a = paise(t.amount);
  if (t.type === "expense") return -a;
  if (t.type === "income") return a;
  return 0;
}

// A name that opens its screen (rows are buttons, so these are spans handled in wireRows).
const goAcc = (id) => html`<span class="go" data-goto="#/account/${encodeURIComponent(id)}">${accName(id)}</span>`;
const goCat = (id, full) => html`<span class="go" data-goto="#/category/${encodeURIComponent(id)}">${catName(id, full)}</span>`;
const arrow = html`<span class="arr">→</span>`;

function subtitle(t, account) {
  if (t.type === "transfer") {
    if (account) return t.account === account ? html`to ${goAcc(t.to_account)}` : html`from ${goAcc(t.account)}`;
    return html`${goAcc(t.account)} ${arrow} ${goAcc(t.to_account)}`;
  }
  if (t.type === "journal") return "journal entry";
  const c = t.splits?.length ? html`Split · ${t.splits.map((x, i) => html`${i ? " + " : ""}${x.account ? html`${arrow} ${goAcc(x.account)}` : goCat(x.category)}`)}`
    : t.category ? goCat(t.category, true) : "";
  return account ? c : html`${c} · ${goAcc(t.account)}`;
}

// Category colour for the icon circle (transfers and splits stay neutral).
const tint = (t) => (t.type === "transfer" || t.splits?.length ? "" : cat(t.category)?.color || "");

/** One row. */
export function row(t, { account, showDate = false, balances } = {}) {
  const e = effect(t, account);
  const icon = t.type === "transfer" ? "⇄" : t.splits?.length ? "✂️" : catIcon(t.category);
  const cls = t.type === "transfer" && !account ? "" : e > 0 ? "pos" : e < 0 ? "neg" : "";
  const shown = t.type === "transfer" && !account ? money(t.amount) : (e > 0 ? "+" : "") + money(e / 100);
  return html`<li class="${S.flash && S.flash === t.id ? "flash" : ""}"><button class="item" data-tx="${t.id}">
    <span class="ic tx-ic" ${tint(t) ? html`style="--c:${tint(t)}"` : ""}>${icon}</span>
    <span class="main-t"><span class="t">${t.title}</span>
      <span class="s">${showDate ? fmtDay(t.date) + " · " : ""}${subtitle(t, account)}</span></span>
    <span class="end"><span class="amt ${cls}">${shown}</span>
      <span class="s">${t._pending ? html`<span class="badge warn">pending sync</span>` : t.invalid ? html`<span class="badge neg" title="${t.invalid}">invalid</span>` :
        t.scheduled ? html`<span class="badge">${relDay(t.date, today())}</span>` : balances?.[t.id] != null ? html`<span class="run" title="Balance after this">${money(balances[t.id])}</span>` : ""}</span></span>
  </button></li>`;
}

/** Day-grouped list with day totals. */
export function grouped(items, opts = {}) {
  if (!items.length) return html`<div class="empty">${opts.empty || "No transactions"}</div>`;
  const days = new Map();
  for (const t of items) {
    if (!days.has(t.date)) days.set(t.date, []);
    days.get(t.date).push(t);
  }
  const out = [];
  for (const [d, list] of days) {
    const sum = list.reduce((a, t) => a + effect(t, opts.account), 0);
    out.push(html`<div class="day"><span>${fmtDay(d)}${d > today() ? " · upcoming" : ""}</span><span class="amt ${sum > 0 ? "pos" : ""}">${sum ? (sum > 0 ? "+" : "") + money(sum / 100) : ""}</span></div>
      <ul class="list">${list.map((t) => row(t, opts))}</ul>`);
  }
  return html`${out}`;
}

/** Offline-queued creates shown as pending rows (newest first). */
export function pendingCreates() {
  return queue().filter((q) => q.method === "POST" && q.url === "/api/transactions").map((q) => ({ ...q.body, _pending: true })).reverse();
}

/** Wires clicks on rows: onOpen(id); a category or account name opens its screen. */
export function wireRows(root, items, onOpen) {
  root.querySelectorAll("[data-tx]").forEach((b) => b.addEventListener("click", (e) => {
    const go = e.target.closest("[data-goto]");
    if (go) { location.hash = go.dataset.goto; return; }
    const t = items.find((x) => x.id === b.dataset.tx);
    if (t) onOpen(t);
  }));
}
