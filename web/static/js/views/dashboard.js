// Overview: net worth, the period's income/expense, budgets, upcoming bills,
// credit cards, accounts, recent transactions and a 6-month cash-flow chart.

import { html, money, amt, paise, qs, fmtShort, relDay, parseDate, ymd } from "../util.js";
import { S, periodState, periodQuery, periodBar, wirePeriod, catName, catIcon, today, loading, ro } from "../ui.js";
import { get } from "../api.js";
import { incomeExpense, ranked, wireTips } from "../charts.js";
import { grouped, wireRows, pendingCreates } from "../txlist.js";
import { openTx } from "../txform.js";

export async function render(root, ctx) {
  const st = S.state;
  const p = periodState("dash");
  root.innerHTML = String(loading());
  const sixAgo = parseDate(today());
  sixAgo.setMonth(sixAgo.getMonth() - 5, 1);
  const [tot, bud, up, recent, flow] = await Promise.all([
    get("/api/categories/totals" + qs(periodQuery(p))),
    get("/api/budgets" + qs(periodQuery(p))),
    get("/api/upcoming" + qs({ days: st.settings.upcoming_days })),
    get("/api/transactions" + qs({ limit: 8, when: "posted" })),
    get("/api/reports/cashflow" + qs({ from: ymd(sixAgo), to: today() })),
  ]);
  if (!ctx.alive()) return;
  const s = st.summary;
  const cards = st.accounts.filter((a) => a.card && !a.archived);
  const expTop = st.categories.filter((c) => !c.parent && c.kind === "expense" && paise(tot.totals[c.id]) > 0)
    .map((c) => ({ key: c.id, name: c.name, icon: c.icon, value: tot.totals[c.id] }))
    .sort((a, b) => paise(b.value) - paise(a.value)).slice(0, 6);
  const pending = pendingCreates();
  const recentItems = [...pending, ...recent.items];
  const visible = st.accounts.filter((a) => !a.archived && !a.hidden);

  root.innerHTML = String(html`
    ${periodBar(p, tot.range)}
    <div class="grid four">
      <div class="card stat"><div class="label">Net worth</div><div class="value">${amt(s.net_worth)}</div>
        <div class="sub">Assets ${amt(s.assets)} · Owed ${amt(String(-paise(s.liabilities) / 100))}</div></div>
      <div class="card stat"><div class="label">Income · ${tot.range.label}</div><div class="value pos">${amt(tot.income)}</div></div>
      <div class="card stat"><div class="label">Expense · ${tot.range.label}</div><div class="value neg">${amt(tot.expense)}</div></div>
      <div class="card stat"><div class="label">Net · ${tot.range.label}</div><div class="value">${amt(tot.net, { signed: true })}</div>
        <div class="sub">${paise(tot.income) > 0 ? `Saved ${Math.round((paise(tot.net) * 100) / paise(tot.income))}% of income` : ""}</div></div>
    </div>
    <div class="grid two" style="margin-top:14px">
      <div class="stack">
        <section class="card"><header><h2>Spending · ${tot.range.label}</h2><a href="#/categories">All</a></header>
          ${expTop.length ? html`<div data-cats>${ranked(expTop, tot.expense)}</div>` : html`<div class="empty">Nothing spent in this period</div>`}</section>
        <section class="card"><header><h2>Budgets</h2><a href="#/budgets">All</a></header>
          ${paise(bud.total.budget) > 0 ? html`
            <div class="row between"><span>${amt(bud.total.spent)} of ${amt(bud.total.budget)}</span><span class="${paise(bud.total.left) < 0 ? "neg" : "muted"}">${paise(bud.total.left) < 0 ? "over by " : ""}${amt(String(Math.abs(paise(bud.total.left)) / 100))}${paise(bud.total.left) >= 0 ? " left" : ""}</span></div>
            <div class="bar ${bud.total.percent > 100 ? "over" : bud.total.percent > 85 ? "near" : ""}" style="margin:8px 0 12px"><i style="width:${Math.min(100, bud.total.percent)}%"></i></div>
            ${bud.lines.filter((l) => !l.parent).slice(0, 4).map((l) => html`<div style="margin:8px 0">
              <div class="row between"><span>${catIcon(l.category)} ${catName(l.category)}</span><small>${amt(l.spent)} / ${amt(l.budget)}</small></div>
              <div class="bar ${l.percent > 100 ? "over" : l.percent > 85 ? "near" : ""}"><i style="width:${Math.min(100, l.percent)}%"></i></div></div>`)}`
          : html`<div class="empty">No budgets yet — set one on a category</div>`}</section>
        <section class="card"><header><h2>Cash flow · 6 months</h2><a href="#/reports">Reports</a></header>${incomeExpense(flow.rows)}</section>
      </div>
      <div class="stack">
        <section class="card"><header><h2>Upcoming</h2><a href="#/scheduled">Scheduled</a></header>
          ${up.length ? html`<ul class="list">${up.slice(0, 6).map((t) => html`<li><a class="item" href="#/scheduled">
            <span class="ic">${t.type === "transfer" ? "⇄" : catIcon(t.category)}</span>
            <span class="main-t"><span class="t">${t.title}</span><span class="s">${fmtShort(t.date)} · ${relDay(t.date, today())}</span></span>
            <span class="end"><span class="amt ${t.type === "income" ? "pos" : t.type === "expense" ? "neg" : ""}">${money(t.amount)}</span></span></a></li>`)}</ul>`
          : html`<div class="empty">Nothing due in the next ${st.settings.upcoming_days} days</div>`}</section>
        ${cards.length ? html`<section class="card"><header><h2>Credit cards</h2></header>
          ${cards.map((a) => html`<a class="item" href="#/account/${encodeURIComponent(a.id)}"><span class="ic">${a.icon}</span>
            <span class="main-t"><span class="t">${a.name}</span><span class="s">${a.card.due_date ? `Due ${fmtShort(a.card.due_date)} · ` : ""}${a.credit_limit ? `${a.card.utilised}% used` : ""}</span></span>
            <span class="end"><span class="amt neg">${money(String(-paise(a.balance) / 100))}</span><span class="s">${a.credit_limit ? html`avail ${money(a.card.available)}` : ""}</span></span></a>`)}</section>` : ""}
        <section class="card"><header><h2>Accounts</h2><a href="#/accounts">All</a></header>
          ${visible.length ? html`<ul class="list">${visible.slice(0, 8).map((a) => html`<li><a class="item" href="#/account/${encodeURIComponent(a.id)}">
            <span class="ic">${a.icon}</span><span class="main-t"><span class="t">${a.name}</span><span class="s">${a.type_label}</span></span>
            <span class="end">${amt(a.balance, { signed: false, cls: paise(a.balance) < 0 ? "neg" : "" })}</span></a></li>`)}</ul>`
          : html`<div class="empty">${ro() ? "No accounts" : html`No accounts yet. <a href="#/accounts">Add your first account</a>`}</div>`}</section>
        <section class="card"><header><h2>Recent</h2><a href="#/tx">All</a></header><div data-recent>${grouped(recentItems)}</div></section>
      </div>
    </div>`);
  wirePeriod(root, p, tot, () => render(root, ctx));
  wireTips(root);
  root.querySelectorAll("[data-cats] [data-key]").forEach((el) => el.addEventListener("click", () => ctx.go("#/category/" + encodeURIComponent(el.dataset.key))));
  wireRows(root.querySelector("[data-recent]"), recentItems, (t) => (t._pending ? null : openTx(t, () => ctx.refresh())));
}
