// Scheduled: upcoming items (future-dated transactions + reminder occurrences) and
// the reminders themselves — recurring bills, salary, SIPs — which arthik posts
// automatically on each due date.

import { html, raw, qs, money, amt, fmtDay, fmtDate, relDay, calc, fromPaise, paise } from "../util.js";
import { S, sheet, toast, confirmSheet, formData, ro, today, loading, accOptions, catOptions, accName, catName, catIcon, labelName } from "../ui.js";
import { get, post, put, del } from "../api.js";
import { openTx } from "../txform.js";

const REPEAT = { none: "Once", daily: "Daily", weekly: "Weekly", biweekly: "Every 2 weeks", monthly: "Monthly", bimonthly: "Every 2 months", quarterly: "Quarterly", halfyearly: "Half-yearly", yearly: "Yearly" };

export async function render(root, ctx) {
  const st = S.state;
  ctx.actions(ro() ? "" : html`<button class="primary" data-new>Add reminder</button>`);
  root.innerHTML = String(loading());
  const days = S.view.schedDays || 60;
  const up = await get("/api/upcoming" + qs({ days }));
  if (!ctx.alive()) return;
  const active = st.reminders.filter((r) => !r.done);
  const done = st.reminders.filter((r) => r.done);
  const sign = (t) => (t.type === "income" ? "pos" : t.type === "expense" ? "neg" : "");
  const remRow = (r) => html`<li><button class="item" data-rem="${r.id}">
    <span class="ic">${r.type === "transfer" ? "⇄" : catIcon(r.category)}</span>
    <span class="main-t"><span class="t">${r.title} ${r.paused ? html`<span class="badge warn">paused</span>` : ""}${r.done ? html`<span class="badge">finished</span>` : ""}</span>
      <span class="s">${REPEAT[r.repeat] || r.repeat} · ${r.type === "transfer" ? `${accName(r.account)} → ${accName(r.to_account)}` : `${catName(r.category, true)} · ${accName(r.account)}`}${r.done ? "" : ` · next ${fmtDate(r.next)}`}${r.remaining ? ` · ${r.remaining} left` : ""}${r.end_date ? ` · until ${fmtDate(r.end_date)}` : ""}</span></span>
    <span class="end"><span class="amt ${sign(r)}">${money(r.amount)}</span><span class="s">${r.posted ? `${r.posted} posted` : ""}</span></span></button></li>`;
  root.innerHTML = String(html`
    <div class="grid two">
      <div class="card"><header><h2>Upcoming</h2>
        <select data-days style="width:auto;min-height:32px;padding:4px 8px">${[7, 30, 60, 90, 365].map((n) => html`<option value="${n}" ${n === days ? raw("selected") : ""}>${n} days</option>`)}</select></header>
        ${up.length ? html`<ul class="list">${up.map((t) => html`<li><button class="item" data-up="${t.id}">
          <span class="ic">${t.type === "transfer" ? "⇄" : catIcon(t.category)}</span>
          <span class="main-t"><span class="t">${t.title}</span><span class="s">${fmtDay(t.date)} · ${relDay(t.date, today())} · ${t.upcoming ? "reminder" : "future-dated"}</span></span>
          <span class="end"><span class="amt ${sign(t)}">${money(t.amount)}</span></span></button></li>`)}</ul>`
        : html`<div class="empty">Nothing scheduled in the next ${days} days</div>`}
        <p class="muted" style="font-size:.85rem;margin-bottom:0">Future-dated transactions and reminders post automatically on their date and then count in every balance.</p></div>
      <div class="card"><header><h2>Reminders</h2></header>
        ${active.length ? html`<ul class="list">${active.map(remRow)}</ul>` : html`<div class="empty">No reminders yet</div>`}
        ${done.length ? html`<details style="margin-top:10px"><summary class="muted">${done.length} finished</summary><ul class="list">${done.map(remRow)}</ul></details>` : ""}</div>
    </div>`);
  root.querySelector("[data-days]").addEventListener("change", (e) => { S.view.schedDays = +e.target.value; render(root, ctx); });
  ctx.actionsEl.querySelector("[data-new]")?.addEventListener("click", () => reminderForm(null, ctx));
  root.querySelectorAll("[data-rem]").forEach((b) => b.addEventListener("click", () => {
    const r = st.reminders.find((x) => x.id === b.dataset.rem);
    if (r) reminderForm(r, ctx);
  }));
  root.querySelectorAll("[data-up]").forEach((b) => b.addEventListener("click", () => {
    const t = up.find((x) => x.id === b.dataset.up);
    if (!t) return;
    if (t.upcoming) reminderForm(st.reminders.find((r) => r.id === t.reminder), ctx);
    else openTx(t, () => ctx.refresh());
  }));
}

function reminderForm(r, ctx) {
  if (ro()) {
    if (r) sheet(r.title, html`<p>${REPEAT[r.repeat]} ${r.type} of <b>${money(r.amount)}</b>, next on ${fmtDate(r.next)}.</p><p class="muted">The demo is read-only.</p>`);
    return;
  }
  const isNew = !r;
  r = r || { type: "expense", repeat: "monthly", next: today(), status: "uncleared", account: S.state.accounts.find((a) => !a.archived)?.id };
  let type = r.type;
  const body = () => html`
    <div class="seg full" style="margin-bottom:14px">${["expense", "income", "transfer"].map((t) => html`<button type="button" data-type="${t}" class="${t === type ? "on" : ""}">${t[0].toUpperCase() + t.slice(1)}</button>`)}</div>
    <label class="f"><span>Title</span><input name="title" value="${r.title || ""}" autofocus></label>
    <label class="f"><span>Amount</span><input name="amount" inputmode="decimal" value="${r.amount ? fromPaise(paise(r.amount)) : ""}"></label>
    ${type === "transfer" ? html`<div class="fields two">
      <label class="f"><span>From</span><select name="account">${accOptions(r.account)}</select></label>
      <label class="f"><span>To</span><select name="to_account"><option value="">Choose…</option>${accOptions(r.to_account)}</select></label></div>`
    : html`<div class="fields two">
      <label class="f"><span>Account</span><select name="account">${accOptions(r.account)}</select></label>
      <label class="f"><span>Category</span><select name="category"><option value="">Choose…</option>${catOptions(type, r.category)}</select></label></div>`}
    <div class="fields two">
      <label class="f"><span>Repeat</span><select name="repeat">${Object.entries(REPEAT).map(([k, l]) => html`<option value="${k}" ${k === r.repeat ? raw("selected") : ""}>${l}</option>`)}</select></label>
      <label class="f"><span>${isNew ? "First date" : "Next date"}</span><input type="date" name="next" value="${r.next}"></label>
    </div>
    <div class="fields two">
      <label class="f"><span>End date (optional)</span><input type="date" name="end_date" value="${r.end_date || ""}"></label>
      <label class="f"><span>Times left (optional)</span><input name="remaining" inputmode="numeric" value="${r.remaining || ""}" placeholder="unlimited"></label>
    </div>
    <div class="fields two">
      <label class="f"><span>Posted as</span><select name="status">${["uncleared", "cleared", "reconciled"].map((s) => html`<option value="${s}" ${s === (r.status || "uncleared") ? raw("selected") : ""}>${s}</option>`)}</select></label>
      <label class="f"><span>Labels (comma separated)</span><input name="labels" value="${(r.labels || []).map(labelName).join(", ")}"></label>
    </div>
    <label class="f"><span>Notes</span><textarea name="notes">${r.notes || ""}</textarea></label>
    <label class="check"><input type="checkbox" name="paused" ${r.paused ? raw("checked") : ""}> Paused</label>
    <p class="muted" style="font-size:.85rem;margin:0">A first date in the past back-fills every occurrence up to today.</p>`;
  sheet(isNew ? "Add reminder" : "Edit reminder", body(),
    html`${isNew ? "" : html`<button class="danger" data-del>Delete</button><button data-skip>Skip next</button>`}<span class="grow"></span><button data-close>Cancel</button><button class="primary" data-save>Save</button>`,
    (d, close) => {
      const wireType = () => d.querySelectorAll("[data-type]").forEach((b) => b.addEventListener("click", () => {
        Object.assign(r, formData(d));
        r.labels = String(r.labels || "").split(",").map((x) => x.trim()).filter(Boolean);
        type = b.dataset.type;
        d.querySelector(".sheet-b").innerHTML = String(body());
        wireType();
      }));
      wireType();
      d.querySelector("[data-save]").addEventListener("click", async () => {
        const v = formData(d);
        const a = calc(v.amount);
        if (Number.isNaN(a) || a <= 0) return toast("Enter a positive amount", true);
        const body = { title: v.title, type, amount: fromPaise(a), account: v.account, to_account: v.to_account || "", category: v.category || "",
          repeat: v.repeat, next: v.next, end_date: v.end_date, remaining: parseInt(v.remaining || "0", 10) || 0, status: v.status,
          labels: v.labels.split(",").map((x) => x.trim()).filter(Boolean), notes: v.notes, paused: v.paused };
        try {
          if (isNew) await post("/api/reminders", body); else await put("/api/reminders/" + encodeURIComponent(r.id), body);
          close();
          toast(v.next <= today() ? "Saved — due occurrences were posted" : "Saved");
          ctx.refresh();
        } catch (e) { toast(e.message, true); }
      });
      d.querySelector("[data-skip]")?.addEventListener("click", async () => {
        try { await post(`/api/reminders/${encodeURIComponent(r.id)}/skip`, {}); close(); toast("Skipped to the next date"); ctx.refresh(); }
        catch (e) { toast(e.message, true); }
      });
      d.querySelector("[data-del]")?.addEventListener("click", async () => {
        if (!(await confirmSheet("Delete reminder?", "Transactions it already posted stay in your books."))) return;
        try { await del("/api/reminders/" + encodeURIComponent(r.id)); close(); toast("Deleted"); ctx.refresh(); }
        catch (e) { toast(e.message, true); }
      });
    });
}

export { amt };
