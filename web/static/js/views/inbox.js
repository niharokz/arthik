// Inbox: transactions proposed by bank SMS (Arthik on the phone) and Kronos
// (subscriptions due), waiting for review. Accept as-is, edit then accept, mark as
// "same as" a transaction you already entered, or reject. Accepting teaches Arthik
// what that merchant is, so the next draft fills itself in.

import { html, raw, money, fmtShort, relDay, reducedMotion } from "../util.js";
import { S, toast, ro, today, catName, catIcon, accName, loading, confirmSheet } from "../ui.js";
import { get, post, put } from "../api.js";
import { haptic, isApp, config, call, onNative } from "../native.js";
import { openTx } from "../txform.js";

// How the category was chosen: [badge, label].
const VIA = { learned: ["🧠", "Learned"], past: ["↺", "From past"], source: ["📋", "From source"], ai: ["✨", "AI guess"], default: ["?", "Guessed"] };
// How safe it is to accept without opening: [class, label].
const CONF = { high: ["high", "Ready"], medium: ["medium", "Worth a look"], low: ["low", "Check first"] };
const STATUS = { accepted: "Accepted", matched: "Same as existing", rejected: "Rejected" };

const when = (x) => `${fmtShort(x.date)}${x.time ? " " + x.time : ""} · ${relDay(x.date, today())}`;
const source = (x) => (x.source === "sms" ? "SMS" : x.source === "kronos" ? "Kronos" : x.source);

// The account part of a row ("HDFC Card", "Savings → Card", "…4111?").
function where(x) {
  const name = (id) => (id ? accName(id) : x.digits ? `…${x.digits}?` : "account?");
  return x.type === "transfer" ? `${name(x.account)} → ${name(x.to_account)}` : name(x.account);
}

function viaTag(x) {
  if (x.type === "transfer") return html`<span class="tag">⇄ Transfer</span>`;
  if (!x.category) return html`<span class="tag warn">No category</span>`;
  const via = x.edited ? ["✎", "Edited"] : VIA[x.via] || ["", ""];
  const label = x.via === "source" && !x.edited ? `From ${source(x)}` : via[1];
  return html`<span class="tag cat">${catIcon(x.category)} ${catName(x.category)}</span>${via[1] ? html`<span class="via ${x.edited ? "edited" : x.via}" title="How the category was chosen">${via[0]} ${label}</span>` : ""}`;
}

/** One draft: a compact row on the overview (actions=false), a review card in the inbox. */
export function draftRow(x, actions = true) {
  const sign = x.type === "income" ? "pos" : x.type === "expense" ? "neg" : "";
  const conf = x.status === "pending" ? CONF[x.confidence] : null;
  const icon = x.type === "transfer" ? "⇄" : x.category ? catIcon(x.category) : x.source === "sms" ? "✉️" : "📋";
  const head = html`<div class="item">
      <span class="ic">${icon}${conf ? html`<i class="cdot ${conf[0]}" title="${conf[1]}"></i>` : ""}</span>
      <span class="main-t"><span class="t">${x.title}</span>
        <span class="s">${actions ? "" : html`${viaTag(x)} `}${where(x)} · ${when(x)}</span></span>
      <span class="end"><span class="amt ${sign}">${money(x.amount)}</span>
        <span class="s">${x.status === "pending" ? source(x) : STATUS[x.status] || x.status}</span></span>
    </div>`;
  if (!actions) return html`<li class="draft" data-draft="${x.id}">${head}</li>`;
  const raw = [x.raw, x.pair_raw].filter(Boolean);
  return html`<li class="draft dcard ${x.status} ${conf ? "conf-" + conf[0] : ""}" data-draft="${x.id}">
    ${head}
    <div class="dc-meta">${conf ? html`<span class="conf ${conf[0]}">${conf[1]}</span>` : ""}${viaTag(x)}</div>
    ${x.checks?.length && x.confidence !== "high" ? html`<ul class="checks" ${""}>${x.checks.filter((c) => c !== "See the warnings" && c !== "No category yet" && !(c === "Account not known" && x.warnings?.length)).map((c) => html`<li>${c}</li>`)}</ul>` : ""}
    ${x.warnings?.length ? html`<ul class="warns">${x.warnings.map((w) => html`<li>${w}</li>`)}</ul>` : ""}
    ${x.same ? html`<div class="same">Looks like <b>${x.same.title}</b> · ${money(x.same.amount)} · ${fmtShort(x.same.date)}
      <button class="ghost" data-same="${x.id}">Same — don't add</button></div>` : ""}
    ${raw.length ? html`<details class="raw"><summary>${x.source === "sms" ? (raw.length > 1 ? "Original SMS (2)" : "Original SMS") : "Original text"}</summary>
      ${raw.map((r) => html`<p>${r}</p>`)}</details>` : x.notes ? html`<p class="dc-note">${x.notes}</p>` : ""}
    ${x.status === "pending" ? html`<div class="row draft-actions">
      <button class="primary" data-accept="${x.id}">Accept</button>
      <button data-edit="${x.id}">Edit</button>
      ${x.pair ? html`<button class="ghost" data-unpair="${x.id}">Not a transfer</button>` : ""}
      <span class="grow"></span>
      <button class="ghost danger-t" data-reject="${x.id}">Reject</button></div>` : ""}
    ${x.status === "rejected" ? html`<div class="row draft-actions"><span class="grow"></span><button class="ghost" data-restore="${x.id}">Restore</button></div>` : ""}
  </li>`;
}

// ---- when drafts arrive: phone SMS copy → SMS to drafts → Kronos ----

const pad2 = (n) => String(n).padStart(2, "0");
const stamp = (v) => {
  const d = v ? new Date(v) : null;
  if (!d || Number.isNaN(+d) || d.getFullYear() < 2000) return "";
  return `${fmtShort(`${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`)} ${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
};

// The next time the clock shows hh:mm.
function nextAt(hhmm) {
  const [h, m] = hhmm.split(":").map(Number);
  const d = new Date();
  d.setHours(h, m, 0, 0);
  if (d <= new Date()) d.setDate(d.getDate() + 1);
  return d;
}

/** The two timings (and Kronos) with their "now" buttons; res is GET /api/inbox. */
export function timingsCard(res) {
  const st = res.status || {};
  const phones = res.phones || [];
  const main = res.main ?? !!stamp(st.last_import); // only the main device imports
  const at = res.sms_time || "";
  const app = isApp ? config().sms || {} : null;
  const phoneLine = phones.length
    ? phones.map((p) => html`<div>${p.device}: ${p.schedule ? `daily at ${p.schedule}` : "nightly"}${stamp(p.last_run) ? ` · last ${stamp(p.last_run)}` : ""}</div>`)
    : html`<div>No phone has sent SMS yet — switch it on in the Arthik app: Settings → Phone.</div>`;
  return html`<section class="card timings" data-timings>
    <header><h2>⏱ When drafts arrive</h2></header>
    <div class="tstep"><span class="tn">1</span>
      <div class="grow"><div class="thead"><b>Phone copies bank SMS</b>${app && app.enabled ? html`<button data-smsnow>Copy now</button>` : ""}</div>
        <div class="muted">${phoneLine}${app ? html`<div>Change it in Settings → Phone.</div>` : ""}</div></div></div>
    <div class="tstep"><span class="tn">2</span>
      <div class="grow"><div class="thead"><b>SMS become drafts</b>${ro() ? "" : main ? html`<button data-check>Run now</button>` : html`<small class="faint">done by your main device</small>`}</div>
        <div class="muted">${at ? `Every day at ${at}` : "As soon as the SMS file arrives"}${at ? ` · next ${stamp(nextAt(at))}` : ""}${stamp(st.last_sms) ? ` · last ${stamp(st.last_sms)}` : ""}</div>
        ${ro() ? "" : html`<div class="row wrap tset">
          <select data-smsmode aria-label="When SMS become drafts"><option value="" ${at ? "" : raw("selected")}>As soon as they arrive</option><option value="at" ${at ? raw("selected") : ""}>Every day at</option></select>
          <input type="time" data-smstime value="${at || "23:45"}" ${at ? "" : raw("hidden")} aria-label="Time">
          <button data-savetime hidden>Save</button></div>`}</div></div>
    <div class="tstep"><span class="tn">↻</span>
      <div class="grow"><b>Subscriptions from Kronos</b><div class="muted">As soon as Kronos writes them${stamp(st.last_import) ? ` · checked ${stamp(st.last_import)}` : ""}</div></div></div>
  </section>`;
}

/** Wires the timings card's buttons; after() re-renders the screen. */
export function wireTimings(root, after) {
  const card = root.querySelector("[data-timings]");
  if (!card) return;
  const mode = card.querySelector("[data-smsmode]"), time = card.querySelector("[data-smstime]"), save = card.querySelector("[data-savetime]");
  const changed = () => { time.hidden = !mode.value; save.hidden = false; };
  mode?.addEventListener("change", changed);
  time?.addEventListener("input", changed);
  save?.addEventListener("click", async () => {
    try {
      await put("/api/settings", { ...S.state.settings, inbox_sms_time: mode.value ? time.value : "" });
      toast(mode.value ? `SMS become drafts every day at ${time.value}` : "SMS become drafts as soon as they arrive");
      after();
    } catch (e) { toast(e.message, true); }
  });
  card.querySelector("[data-check]")?.addEventListener("click", async (e) => {
    e.target.disabled = true;
    try {
      const r = await post("/api/inbox/import", {});
      const cap = stamp(r.last_capture);
      toast(r.added ? `${r.added} new draft${r.added > 1 ? "s" : ""}` : `Nothing new${cap ? ` · phone SMS file here is from ${cap}` : ""}`);
    } catch (err) { toast(err.message, true); }
    after();
  });
  card.querySelector("[data-smsnow]")?.addEventListener("click", (e) => {
    e.target.disabled = true;
    const off = onNative((ev) => { if (ev.type === "sms") { off(); toast(ev.result); after(); } });
    call("smsRunNow");
  });
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

  const readyOnes = tab === "pending" ? items.filter((x) => x.confidence === "high" && ready(x) && !x.same) : [];
  const toCheck = tab === "pending" ? items.filter((x) => x.confidence === "low").length : 0;
  root.innerHTML = String(html`
    ${tab === "pending" && items.length ? html`<div class="inbox-sum">
      <span><b>${items.length}</b> to review${toCheck ? html` · <span class="warn-t">${toCheck} to check first</span>` : ""}</span>
      ${readyOnes.length > 1 && !ro() ? html`<button class="ghost" data-accept-ready>✓ Accept ${readyOnes.length} ready</button>` : ""}</div>` : ""}
    ${st.error ? html`<div class="banner warn">⚠️ Some inbox files could not be read: <code>${st.error}</code></div>` : ""}
    ${st.ai_error ? html`<div class="banner warn">✨ The local AI could not be reached — drafts keep their fallback category. <small>${st.ai_error}</small></div>` : ""}
    <section class="card">
      ${items.length ? html`<ul class="list drafts" data-list>${items.map((x) => draftRow(x))}</ul>`
      : html`<div class="empty">${tab === "pending" ? html`Nothing to review.<br><small>Bank SMS from the phone app and subscriptions from Kronos show up here.</small>` : "No reviewed drafts yet."}</div>`}
    </section>
    ${timingsCard(res)}
    <p class="inbox-foot faint">${st.ai ? "✨ AI suggests categories for new merchants · " : ""}🧠 Accepting teaches Arthik the merchant's category.</p>`);
  wireTimings(root, () => ctx.refresh());

  const byId = (id) => items.find((x) => x.id === id);
  const act = async (fn, okMsg) => {
    if (ro()) return toast("The demo is read-only");
    try { await fn(); haptic("confirm"); toast(okMsg); } catch (e) { toast(e.message, true); }
    ctx.refresh();
  };
  // Slides a card away before the list reloads (resolves at once with reduced motion).
  const leave = (id) => new Promise((resolve) => {
    const li = root.querySelector(`[data-draft="${CSS.escape(id)}"]`);
    if (!li || reducedMotion()) return resolve();
    li.style.height = li.offsetHeight + "px";
    li.classList.add("leaving");
    setTimeout(resolve, 260);
  });
  const path = (id, verb) => `/api/inbox/${encodeURIComponent(id)}/${verb}`;
  const edit = (x) => (ro() ? toast("The demo is read-only") : openTx({}, () => ctx.refresh(), { draft: x }));

  root.querySelectorAll("[data-accept]").forEach((b) => b.addEventListener("click", () => {
    const x = byId(b.dataset.accept);
    if (!ready(x)) return edit(x); // something is missing: open the form
    act(() => leave(x.id).then(() => post(path(x.id, "accept"), {})), "Accepted");
  }));
  root.querySelector("[data-accept-ready]")?.addEventListener("click", async () => {
    const names = readyOnes.slice(0, 5).map((x) => x.title).join(", ") + (readyOnes.length > 5 ? "…" : "");
    if (!(await confirmSheet(`Accept ${readyOnes.length} drafts?`, `These were filled in by your learned rules or past transactions and nothing looks off: ${names}`, "Accept all", false))) return;
    act(async () => {
      await Promise.all(readyOnes.map((x) => leave(x.id)));
      for (const x of readyOnes) await post(path(x.id, "accept"), {});
    }, `Accepted ${readyOnes.length}`);
  });
  root.querySelectorAll("[data-edit]").forEach((b) => b.addEventListener("click", () => edit(byId(b.dataset.edit))));
  root.querySelectorAll("[data-reject]").forEach((b) => b.addEventListener("click", () => act(() => leave(b.dataset.reject).then(() => post(path(b.dataset.reject, "reject"), {})), "Rejected")));
  root.querySelectorAll("[data-restore]").forEach((b) => b.addEventListener("click", () => act(() => post(path(b.dataset.restore, "restore"), {}), "Back in the inbox")));
  root.querySelectorAll("[data-unpair]").forEach((b) => b.addEventListener("click", () => act(() => post(path(b.dataset.unpair, "unpair"), {}), "Split into two drafts")));
  root.querySelectorAll("[data-same]").forEach((b) => b.addEventListener("click", async () => {
    const x = byId(b.dataset.same);
    if (ro()) return toast("The demo is read-only");
    if (!(await confirmSheet("Already recorded?", `“${x.title}” will be linked to “${x.same.title}” and nothing new is added.`, "Link", false))) return;
    act(() => post(path(x.id, "match"), { txn: x.same.id }), "Linked");
  }));
}
