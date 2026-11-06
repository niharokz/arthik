// Notifications: reminders from Kronos (bills and subscriptions due). They never
// become transactions themselves — the bank SMS in the inbox is the proof. Each
// shows whether a matching payment is already recorded or waiting in the inbox.

import { html, money, fmtShort, relDay } from "../util.js";
import { toast, ro, today, catIcon, accName, loading, celebrate } from "../ui.js";
import { get, post } from "../api.js";
import { openTx } from "../txform.js";

let tab = "open"; // open | closed

// Where the payment stands: [class, text].
function state(n) {
  if (n.paid) return ["pos", `Paid ✓ ${n.paid.title} · ${fmtShort(n.paid.date)}`];
  if (n.proof) return ["warn", `SMS waiting in the inbox · ${fmtShort(n.proof.date)}`];
  if (n.date < today()) return ["neg", `Due ${relDay(n.date, today())} — no payment found`];
  return ["", n.date === today() ? "Due today" : `Due ${relDay(n.date, today())}`];
}

function card(n) {
  const [cls, text] = state(n);
  const open = n.status === "notice";
  return html`<li class="dcard notice ${n.paid ? "paid" : ""}" data-n="${n.id}">
    <div class="item"><span class="ic">${n.category ? catIcon(n.category) : "🔔"}</span>
      <span class="main-t"><span class="t">${n.title}</span>
        <span class="s">${n.account ? accName(n.account) + " · " : n.account_hint ? n.account_hint + " · " : ""}${fmtShort(n.date)}</span></span>
      <span class="end"><span class="amt neg">${money(n.amount)}</span><span class="s">Kronos</span></span></div>
    <div class="dc-meta"><span class="conf ${cls === "pos" ? "high" : cls === "warn" ? "medium" : cls === "neg" ? "low" : ""}">${text}</span></div>
    ${ro() ? "" : html`<div class="row draft-actions">
      ${!open ? html`<span class="grow"></span><button class="ghost" data-reopen="${n.id}">Reopen</button>` : html`
        ${n.proof ? html`<a class="primary btn" href="#/inbox">Open inbox</a>` : n.paid ? "" : html`<button data-add="${n.id}">Add payment</button>`}
        <span class="grow"></span><button class="ghost" data-dismiss="${n.id}">${n.paid ? "Done" : "Dismiss"}</button>`}</div>`}
  </li>`;
}

export async function render(root, ctx) {
  root.innerHTML = String(loading());
  const res = await get("/api/notices" + (tab === "closed" ? "?all=1" : ""));
  if (!ctx.alive()) return;
  const items = res.items.filter((n) => (tab === "closed" ? n.status === "dismissed" : n.status === "notice"));
  root.innerHTML = String(html`
    <div class="seg" role="tablist" style="margin-bottom:12px">
      <button data-tab="open" class="${tab === "open" ? "on" : ""}">Open${res.active ? ` · ${res.active}` : ""}</button>
      <button data-tab="closed" class="${tab === "closed" ? "on" : ""}">Closed</button></div>
    ${items.length ? html`<ul class="list drafts">${items.map(card)}</ul>`
      : html`<div class="card empty">${tab === "open" ? "No reminders. Bills from Kronos show up here when they are due." : "Nothing closed yet."}</div>`}
    <p class="muted" style="font-size:.85rem">Reminders come from Kronos. The bank SMS in the inbox is what records the payment; a reminder shows Paid ✓ once a transaction with the same amount is recorded within a week of its due date.</p>`);
  root.querySelectorAll("[data-tab]").forEach((b) => b.addEventListener("click", () => { tab = b.dataset.tab; render(root, ctx); }));
  const act = async (path, msg) => {
    try { await post(path); celebrate(msg === "Dismissed" ? "reject" : "ok"); toast(msg); ctx.refresh(); }
    catch (e) { toast(e.message, true); }
  };
  root.querySelectorAll("[data-dismiss]").forEach((b) => b.addEventListener("click", () => act(`/api/notices/${encodeURIComponent(b.dataset.dismiss)}/dismiss`, "Dismissed")));
  root.querySelectorAll("[data-reopen]").forEach((b) => b.addEventListener("click", () => act(`/api/notices/${encodeURIComponent(b.dataset.reopen)}/reopen`, "Reopened")));
  root.querySelectorAll("[data-add]").forEach((b) => b.addEventListener("click", () => {
    const n = items.find((x) => x.id === b.dataset.add);
    openTx({ type: n.type, title: n.title, amount: n.amount, account: n.account, category: n.category, date: n.date <= today() ? n.date : today(), _prefill: true }, () => ctx.refresh());
  }));
}
