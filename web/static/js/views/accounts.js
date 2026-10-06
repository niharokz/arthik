// Accounts: grouped list with net worth, account detail (balance, cleared and
// projected balance, credit-card statement info, balance chart, transactions),
// add/edit/archive/delete, and reconciliation against a statement.

import { html, raw, qs, money, amt, paise, fromPaise, calc, fmtShort, fmtDate } from "../util.js";
import { S, sheet, toast, confirmSheet, formData, ro, today, loading } from "../ui.js";
import { get, post, put, del } from "../api.js";
import { line, wireTips } from "../charts.js";
import { grouped, wireRows, effect } from "../txlist.js";
import { openTx } from "../txform.js";

export async function render(root, ctx) {
  if (ctx.name === "account" && ctx.arg) return detail(root, ctx, ctx.arg);
  const st = S.state;
  const showAll = !!S.view.accAll;
  ctx.actions(html`<button class="ghost" data-all>${showAll ? "Hide archived" : "Show all"}</button>${ro() ? "" : html`<button class="primary" data-new>Add</button>`}`);
  const list = st.accounts.filter((a) => showAll || (!a.archived && !a.hidden));
  const s = st.summary;
  root.innerHTML = String(html`
    <div class="grid three">
      <div class="card stat"><div class="label">Net worth</div><div class="value">${amt(s.net_worth)}</div></div>
      <div class="card stat"><div class="label">Assets</div><div class="value pos">${amt(s.assets)}</div></div>
      <div class="card stat"><div class="label">Liabilities</div><div class="value neg">${amt(s.liabilities)}</div></div>
    </div>
    <div class="card" style="margin-top:14px">
      ${list.length ? st.account_types.map((t) => {
        const group = list.filter((a) => a.type === t.id);
        if (!group.length) return "";
        const total = group.reduce((n, a) => n + paise(a.balance), 0);
        return html`<div class="group-h"><span>${t.icon} ${t.label}</span><span class="amt">${money(total / 100)}</span></div>
          <ul class="list">${group.map((a) => html`<li><a class="item" href="#/account/${encodeURIComponent(a.id)}">
            <span class="ic" style="${a.color ? `background:${a.color}22` : ""}">${a.icon}</span>
            <span class="main-t"><span class="t">${a.name} ${a.archived ? html`<span class="badge">archived</span>` : a.hidden ? html`<span class="badge">hidden</span>` : ""}${a.exclude_net_worth ? html` <span class="badge">not in net worth</span>` : ""}</span>
              <span class="s">${a.card ? (a.credit_limit ? `Available ${money(a.card.available)}` : "Credit card") + (a.card.due_date ? ` · due ${fmtShort(a.card.due_date)}` : "") : paise(a.projected_balance) !== paise(a.balance) ? `After scheduled: ${money(a.projected_balance)}` : t.label}</span></span>
            <span class="end">${amt(a.balance, { cls: paise(a.balance) < 0 ? "neg" : "" })}</span></a></li>`)}</ul>`;
      }) : html`<div class="empty">No accounts yet${ro() ? "" : html` — <button class="primary" data-new2>Add your first account</button>`}</div>`}
    </div>`);
  ctx.actionsEl.querySelector("[data-all]").addEventListener("click", () => { S.view.accAll = !showAll; render(root, ctx); });
  const add = () => accountForm(null, ctx);
  ctx.actionsEl.querySelector("[data-new]")?.addEventListener("click", add);
  root.querySelector("[data-new2]")?.addEventListener("click", add);
}

async function detail(root, ctx, id) {
  const a = S.state.accounts.find((x) => x.id === id);
  if (!a) { root.innerHTML = String(html`<div class="empty">Account not found. <a href="#/accounts">Back</a></div>`); return; }
  ctx.title(a.name);
  ctx.actions(ro() ? "" : html`<button class="ghost" data-rec>Reconcile</button><button data-edit>Edit</button>`);
  root.innerHTML = String(loading());
  const [hist, tx] = await Promise.all([
    get(`/api/accounts/${encodeURIComponent(id)}/history`),
    get("/api/transactions" + qs({ account: id, limit: 400, upcoming: "1" })),
  ]);
  if (!ctx.alive()) return;
  const liab = a.class === "liability";
  root.innerHTML = String(html`
    <div class="grid three">
      <div class="card stat"><div class="label">${liab ? "Balance (owed is negative)" : "Balance"} · today</div><div class="value">${amt(a.balance, { cls: paise(a.balance) < 0 ? "neg" : "" })}</div>
        <div class="sub">${a.type_label}${a.opening_date ? ` · opened ${fmtDate(a.opening_date)}` : ""}</div></div>
      <div class="card stat"><div class="label">Cleared balance</div><div class="value">${amt(a.cleared_balance)}</div><div class="sub">cleared + reconciled only</div></div>
      <div class="card stat"><div class="label">Projected</div><div class="value">${amt(a.projected_balance)}</div><div class="sub">incl. future-dated</div></div>
    </div>
    ${a.card ? html`<div class="card" style="margin-top:14px"><header><h2>Card</h2></header>
      <div class="grid four">
        <div class="stat"><div class="label">Credit limit</div><div class="value" style="font-size:1.05rem">${a.credit_limit ? amt(a.credit_limit) : "—"}</div></div>
        <div class="stat"><div class="label">Available</div><div class="value" style="font-size:1.05rem">${a.credit_limit ? amt(a.card.available) : "—"}</div>
          ${a.credit_limit ? html`<div class="bar ${a.card.utilised > 80 ? "over" : a.card.utilised > 50 ? "near" : ""}" style="margin-top:6px"><i style="width:${Math.min(100, a.card.utilised)}%"></i></div>` : ""}</div>
        <div class="stat"><div class="label">Last statement${a.card.last_statement ? " · " + fmtShort(a.card.last_statement) : ""}</div><div class="value" style="font-size:1.05rem">${amt(a.card.billed)}</div>
          <div class="sub">${a.card.due_date ? `due ${fmtDate(a.card.due_date)}` : "set a due day to see it"}</div></div>
        <div class="stat"><div class="label">Unbilled</div><div class="value" style="font-size:1.05rem">${amt(a.card.unbilled)}</div>
          <div class="sub">${a.card.next_statement ? `next statement ${fmtShort(a.card.next_statement)}` : ""}</div></div>
      </div></div>` : ""}
    <div class="card" style="margin-top:14px"><header><h2>Balance · 12 months</h2></header>${line(hist.map((h) => ({ label: h.label, value: h.balance })))}</div>
    ${a.notes ? html`<div class="card"><p style="margin:0;white-space:pre-wrap">${a.notes}</p></div>` : ""}
    <div class="card"><header><h2>Transactions</h2><a href="#/tx?account=${encodeURIComponent(id)}">Filter & export</a></header>
      <div data-list>${grouped(tx.items, { account: id })}</div></div>`);
  wireTips(root);
  wireRows(root.querySelector("[data-list]"), tx.items, (t) => (t.upcoming ? ctx.go("#/scheduled") : openTx(t, () => ctx.refresh())));
  ctx.actionsEl.querySelector("[data-edit]")?.addEventListener("click", () => accountForm(a, ctx));
  ctx.actionsEl.querySelector("[data-rec]")?.addEventListener("click", () => reconcile(a, tx.items.filter((t) => !t.upcoming && !t.scheduled), ctx));
}

function accountForm(a, ctx) {
  const st = S.state;
  const isNew = !a;
  a = a || { type: "bank", opening_balance: "0", opening_date: today() };
  let type = a.type;
  const liabOf = (t) => st.account_types.find((x) => x.id === t)?.class === "liability";
  const ob = paise(a.opening_balance);
  const body = () => html`
    <label class="f"><span>Name</span><input name="name" value="${a.name || ""}" autofocus></label>
    <div class="fields two">
      <label class="f"><span>Type</span><select name="type">${st.account_types.map((t) => html`<option value="${t.id}" ${t.id === type ? raw("selected") : ""}>${t.icon} ${t.label}</option>`)}</select></label>
      <label class="f"><span>Icon (emoji, optional)</span><input name="icon" value="${a.icon && a.icon !== (st.account_types.find((t) => t.id === a.type) || {}).icon ? a.icon : ""}" maxlength="4"></label>
    </div>
    <div class="fields two">
      <label class="f"><span>${liabOf(type) ? "Amount owed at the start" : "Opening balance"}</span><input name="opening" inputmode="decimal" value="${fromPaise(liabOf(type) ? -ob : ob)}"></label>
      <label class="f"><span>As of</span><input type="date" name="opening_date" value="${a.opening_date || ""}"></label>
    </div>
    ${type === "credit_card" ? html`<div class="fields two">
      <label class="f"><span>Credit limit</span><input name="credit_limit" inputmode="decimal" value="${a.credit_limit || ""}"></label>
      <label class="f"><span>Statement day / due day</span><span class="row"><input name="statement_day" inputmode="numeric" placeholder="e.g. 18" value="${a.statement_day || ""}"><input name="due_day" inputmode="numeric" placeholder="e.g. 7" value="${a.due_day || ""}"></span></label>
    </div>` : ""}
    <label class="f"><span>Colour</span><input type="color" name="color" value="${a.color || "#3a807a"}" style="height:40px;padding:4px"></label>
    <label class="f"><span>Notes</span><textarea name="notes">${a.notes || ""}</textarea></label>
    <label class="check"><input type="checkbox" name="hidden" ${a.hidden ? raw("checked") : ""}> Hide from the overview</label>
    <label class="check"><input type="checkbox" name="exclude_net_worth" ${a.exclude_net_worth ? raw("checked") : ""}> Leave out of net worth</label>
    ${isNew ? "" : html`<label class="check"><input type="checkbox" name="archived" ${a.archived ? raw("checked") : ""}> Archived (closed account — kept for history)</label>`}`;
  sheet(isNew ? "Add account" : "Edit account", body(),
    html`${isNew ? "" : html`<button class="danger" data-del>Delete</button>`}<span class="grow"></span><button data-close>Cancel</button><button class="primary" data-save>Save</button>`,
    (d, close) => {
      const onType = (e) => {
        const v = formData(d);
        const o = calc(v.opening) || 0;
        Object.assign(a, { name: v.name, icon: v.icon, opening_date: v.opening_date, notes: v.notes, color: v.color,
          credit_limit: v.credit_limit, statement_day: v.statement_day, due_day: v.due_day });
        type = e.target.value;
        a.opening_balance = fromPaise(liabOf(type) ? -Math.abs(o) : Math.abs(o));
        d.querySelector(".sheet-b").innerHTML = String(body());
        d.querySelector('[name="type"]').addEventListener("change", onType);
      };
      d.querySelector('[name="type"]').addEventListener("change", onType);
      d.querySelector("[data-save]").addEventListener("click", async () => {
        const v = formData(d);
        const o = calc(v.opening || "0");
        if (Number.isNaN(o)) return toast("Opening balance is not a number", true);
        const body = {
          name: v.name, type: v.type, icon: v.icon, color: v.color, notes: v.notes, opening_date: v.opening_date,
          opening_balance: fromPaise(liabOf(v.type) ? -Math.abs(o) : o),
          credit_limit: v.credit_limit ? fromPaise(calc(v.credit_limit) || 0) : "0",
          statement_day: parseInt(v.statement_day || "0", 10) || 0, due_day: parseInt(v.due_day || "0", 10) || 0,
          hidden: v.hidden, archived: !!v.archived, exclude_net_worth: v.exclude_net_worth, order: a.order || 0,
        };
        try {
          if (isNew) { const r = await post("/api/accounts", body); close(); toast("Account added"); await ctx.refresh(); ctx.go("#/account/" + encodeURIComponent(r.id)); }
          else { await put("/api/accounts/" + encodeURIComponent(a.id), body); close(); toast("Saved"); ctx.refresh(); }
        } catch (e) { toast(e.message, true); }
      });
      d.querySelector("[data-del]")?.addEventListener("click", async () => {
        if (!(await confirmSheet("Delete account?", `“${a.name}” can only be deleted when no transactions use it. Otherwise archive it.`))) return;
        try { await del("/api/accounts/" + encodeURIComponent(a.id)); close(); toast("Deleted"); await ctx.refresh(); ctx.go("#/accounts"); }
        catch (e) { toast(e.message, true); }
      });
    });
}

function reconcile(a, items, ctx) {
  const open = items.filter((t) => t.status !== "reconciled");
  const reconciledBase = paise(a.opening_balance) + items.filter((t) => t.status === "reconciled").reduce((n, t) => n + effect(t, a.id), 0);
  const chosen = new Set(open.filter((t) => t.status === "cleared").map((t) => t.id));
  const body = html`
    <p class="muted" style="margin-top:0">Tick the transactions that appear on your statement, enter its closing balance, and reconcile when the difference is zero.</p>
    <label class="f"><span>Statement closing balance${a.class === "liability" ? " (owed is negative)" : ""}</span><input name="stmt" inputmode="decimal" placeholder="${fromPaise(paise(a.cleared_balance))}"></label>
    <div class="row between" style="margin-bottom:10px"><span>Ticked balance <b data-tb class="amt"></b></span><span>Difference <b data-diff class="amt"></b></span></div>
    ${open.length ? html`<ul class="list">${open.map((t) => html`<li><label class="item" style="cursor:pointer">
      <input type="checkbox" data-r="${t.id}" ${chosen.has(t.id) ? raw("checked") : ""}>
      <span class="main-t"><span class="t">${t.title}</span><span class="s">${fmtShort(t.date)} · ${t.status}</span></span>
      <span class="end">${amt(fromPaise(effect(t, a.id)), { signed: true })}</span></label></li>`)}</ul>` : html`<div class="empty">Everything is reconciled.</div>`}`;
  sheet("Reconcile " + a.name, body, html`<button data-cleared>Save as cleared</button><span class="grow"></span><button class="primary" data-ok>Reconcile ticked</button>`, (d, close) => {
    const upd = () => {
      const tb = reconciledBase + open.filter((t) => chosen.has(t.id)).reduce((n, t) => n + effect(t, a.id), 0);
      d.querySelector("[data-tb]").textContent = money(tb / 100);
      const sv = calc(d.querySelector('[name="stmt"]').value);
      const el = d.querySelector("[data-diff]");
      el.textContent = Number.isNaN(sv) ? "—" : money((sv - tb) / 100);
      el.className = "amt " + (!Number.isNaN(sv) && sv !== tb ? "neg" : "pos");
    };
    d.querySelectorAll("[data-r]").forEach((c) => c.addEventListener("change", () => { c.checked ? chosen.add(c.dataset.r) : chosen.delete(c.dataset.r); upd(); }));
    d.querySelector('[name="stmt"]').addEventListener("input", upd);
    upd();
    const send = async (status) => {
      try {
        const ids = [...chosen];
        const r = await post(`/api/accounts/${encodeURIComponent(a.id)}/reconcile`, { ids, status });
        // un-ticked "cleared" ones go back to uncleared when saving as cleared
        if (status === "cleared") {
          const back = open.filter((t) => t.status === "cleared" && !chosen.has(t.id)).map((t) => t.id);
          if (back.length) await post(`/api/accounts/${encodeURIComponent(a.id)}/reconcile`, { ids: back, status: "uncleared" });
        }
        close();
        toast(`${r.updated} transaction(s) ${status}`);
        ctx.refresh();
      } catch (e) { toast(e.message, true); }
    };
    d.querySelector("[data-ok]").addEventListener("click", () => send("reconciled"));
    d.querySelector("[data-cleared]").addEventListener("click", () => send("cleared"));
  });
}
