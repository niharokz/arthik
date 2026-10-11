// Inbox: transactions proposed by bank SMS (Arthik on the phone) and Kronos
// (subscriptions due), waiting for review. Accept as-is, edit then accept, mark as
// "same as" a transaction you already entered, or reject. Accepting teaches Arthik
// what that merchant is, so the next draft fills itself in.

import { html, money, fmtShort, relDay } from "../util.js";
import { S, toast, ro, today, catName, catIcon, accName, loading, confirmSheet } from "../ui.js";
import { get, post } from "../api.js";
import { haptic } from "../native.js";
import { openTx } from "../txform.js";

const VIA = { learned: ["🧠", "Learned"], past: ["↺", "Past"], source: ["📋", "Kronos"], ai: ["✨", "AI"], default: ["?", "Check"] };
const STATUS = { accepted: "Accepted", matched: "Same as existing", rejected: "Rejected" };

const when = (x) => `${fmtShort(x.date)}${x.time ? " " + x.time : ""} · ${relDay(x.date, today())}`;

// The account part of a row ("HDFC Card", "Savings → Card", "…4275?").
function where(x) {
  const name = (id) => (id ? accName(id) : x.digits ? `…${x.digits}?` : "account?");
  return x.type === "transfer" ? `${name(x.account)} → ${name(x.to_account)}` : name(x.account);
}

/** One draft as a list row (dashboard uses it without actions). */
export function draftRow(x, actions = true) {
  const sign = x.type === "income" ? "pos" : x.type === "expense" ? "neg" : "";
  const via = VIA[x.via];
  const cat = x.type === "transfer" ? html`<span class="tag">⇄ Transfer</span>`
    : x.category ? html`<span class="tag ${x.via || ""}">${via ? via[0] + " " : ""}${catName(x.category)}</span>`
    : html`<span class="tag warn">No category</span>`;
  return html`<li class="draft ${x.status}" data-draft="${x.id}">
    <div class="item">
      <span class="ic">${x.type === "transfer" ? "⇄" : x.category ? catIcon(x.category) : x.source === "sms" ? "✉️" : "📋"}</span>
      <span class="main-t"><span class="t">${x.title}</span>
        <span class="s">${cat} ${where(x)} · ${when(x)}</span></span>
      <span class="end"><span class="amt ${sign}">${money(x.amount)}</span>
        <span class="s">${x.status === "pending" ? (x.source === "sms" ? "SMS" : "Kronos") : STATUS[x.status] || x.status}</span></span>
    </div>
    ${actions && x.warnings?.length ? html`<ul class="warns">${x.warnings.map((w) => html`<li>${w}</li>`)}</ul>` : ""}
    ${actions && x.same ? html`<div class="same">Looks like <b>${x.same.title}</b> · ${money(x.same.amount)} · ${fmtShort(x.same.date)}
      <button class="ghost" data-same="${x.id}">Same — don't add</button></div>` : ""}
    ${actions && x.status === "pending" ? html`<div class="row draft-actions">
      <button class="primary" data-accept="${x.id}">Accept</button>
      <button data-edit="${x.id}">Edit</button>
      ${x.pair ? html`<button class="ghost" data-unpair="${x.id}">Not a transfer</button>` : ""}
      <span class="grow"></span>
      <button class="ghost danger-t" data-reject="${x.id}">Reject</button></div>` : ""}
    ${actions && x.status === "rejected" ? html`<div class="row draft-actions"><span class="grow"></span><button class="ghost" data-restore="${x.id}">Restore</button></div>` : ""}
  </li>`;
}

// A draft can be accepted straight away when nothing is missing.
const ready = (x) => (x.type === "transfer" ? x.account && x.to_account : x.account && x.category);

export async function render(root, ctx) {
  const tab = S.view.inboxTab || "pending";
  root.innerHTML = String(loading());
  const res = await get("/api/inbox?status=" + tab);
  if (!ctx.alive()) return;
  S.state.inbox = { pending: res.pending };
  import("../main.js").then((m) => m.inboxBadge(res.pending));
  const items = res.items;
  const st = res.status || {};
  ctx.actions(html`<div class="seg">
    <button data-tab="pending" class="${tab === "pending" ? "on" : ""}">To review${res.pending ? ` (${res.pending})` : ""}</button>
    <button data-tab="done" class="${tab === "done" ? "on" : ""}">Done</button></div>`);
  ctx.actionsEl.querySelectorAll("[data-tab]").forEach((b) => b.addEventListener("click", () => { S.view.inboxTab = b.dataset.tab; ctx.refresh(); }));

  root.innerHTML = String(html`
    ${st.error ? html`<div class="banner warn">⚠️ Some inbox files could not be read: <code>${st.error}</code></div>` : ""}
    ${st.ai_error ? html`<div class="banner warn">✨ The local AI could not be reached — drafts keep their fallback category. <small>${st.ai_error}</small></div>` : ""}
    <section class="card">
      ${items.length ? html`<ul class="list drafts" data-list>${items.map((x) => draftRow(x))}</ul>`
      : html`<div class="empty">${tab === "pending" ? html`Nothing to review.<br><small>Bank SMS from the phone app and subscriptions from Kronos show up here.</small>` : "No reviewed drafts yet."}</div>`}
    </section>
    <p class="faint" style="font-size:.8rem;text-align:center">${st.ai ? "✨ Local AI suggests categories for new merchants. " : ""}🧠 Accepting teaches Arthik the merchant's category.</p>`);

  const byId = (id) => items.find((x) => x.id === id);
  const act = async (fn, okMsg) => {
    if (ro()) return toast("The demo is read-only");
    try { await fn(); haptic("confirm"); toast(okMsg); ctx.refresh(); } catch (e) { toast(e.message, true); }
  };
  const path = (id, verb) => `/api/inbox/${encodeURIComponent(id)}/${verb}`;
  const edit = (x) => (ro() ? toast("The demo is read-only") : openTx({}, () => ctx.refresh(), { draft: x }));

  root.querySelectorAll("[data-accept]").forEach((b) => b.addEventListener("click", () => {
    const x = byId(b.dataset.accept);
    if (!ready(x)) return edit(x); // something is missing: open the form
    act(() => post(path(x.id, "accept"), {}), "Accepted");
  }));
  root.querySelectorAll("[data-edit]").forEach((b) => b.addEventListener("click", () => edit(byId(b.dataset.edit))));
  root.querySelectorAll("[data-reject]").forEach((b) => b.addEventListener("click", () => act(() => post(path(b.dataset.reject, "reject"), {}), "Rejected")));
  root.querySelectorAll("[data-restore]").forEach((b) => b.addEventListener("click", () => act(() => post(path(b.dataset.restore, "restore"), {}), "Back in the inbox")));
  root.querySelectorAll("[data-unpair]").forEach((b) => b.addEventListener("click", () => act(() => post(path(b.dataset.unpair, "unpair"), {}), "Split into two drafts")));
  root.querySelectorAll("[data-same]").forEach((b) => b.addEventListener("click", async () => {
    const x = byId(b.dataset.same);
    if (ro()) return toast("The demo is read-only");
    if (!(await confirmSheet("Already recorded?", `“${x.title}” will be linked to “${x.same.title}” and nothing new is added.`, "Link", false))) return;
    act(() => post(path(x.id, "match"), { txn: x.same.id }), "Linked");
  }));
}
