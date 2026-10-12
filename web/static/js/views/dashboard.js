// Overview: net worth with its trend (day/week/month/year), the period's income,
// expense and savings, budgets, drafts waiting in the inbox, future-dated
// transactions, credit cards, accounts, recent transactions and a 6-month cash flow.

import { html, money, money0, amt, paise, qs, fmtShort, relDay, parseDate, ymd, addDays, replay, countUp } from "../util.js";
import { S, periodState, periodQuery, periodBar, wirePeriod, catName, catIcon, today, loading, ro } from "../ui.js";
import { get } from "../api.js";
import { incomeExpense, ranked, spark, wireTips } from "../charts.js";
import { grouped, wireRows, pendingCreates } from "../txlist.js";
import { openTx } from "../txform.js";
import { draftRow } from "./inbox.js";

export async function render(root, ctx) {
  const st = S.state;
  const p = periodState("dash");
  root.innerHTML = String(loading());
  const sixAgo = parseDate(today());
  sixAgo.setMonth(sixAgo.getMonth() - 5, 1);
  const ahead = parseDate(today());
  ahead.setDate(ahead.getDate() + st.settings.upcoming_days);
  const [tot, bud, up, recent, flow, box] = await Promise.all([
    get("/api/categories/totals" + qs(periodQuery(p))),
    get("/api/budgets" + qs(periodQuery(p))),
    get("/api/transactions" + qs({ when: "scheduled", to: ymd(ahead), limit: 50 })),
    get("/api/transactions" + qs({ limit: 8, when: "posted" })),
    get("/api/reports/cashflow" + qs({ from: ymd(sixAgo), to: today() })),
    get("/api/inbox").catch(() => ({ items: [], pending: 0 })),
  ]);
  const soon = up.items.sort((a, b) => (a.date < b.date ? -1 : 1));
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
    <section class="card hero" data-hero>
      <div class="hero-head">
        <div class="hero-main"><div class="label">Net worth</div>
          <div class="hero-value">${amt(s.net_worth)}</div>
          <div class="delta" data-delta></div></div>
        <div class="seg mini" role="group" aria-label="Net worth timeline">${Object.entries(SPANS).map(([k, v]) =>
          html`<button data-span="${k}" class="${k === span() ? "on" : ""}" aria-pressed="${k === span()}">${v.label}</button>`)}</div>
      </div>
      <div class="hero-spark" data-spark><div class="sk spark-sk"></div></div>
      <div class="hero-foot"><span>Assets ${amt(s.assets)}</span><span>Owed ${amt(String(-paise(s.liabilities) / 100))}</span></div>
    </section>
    ${periodBar(p, tot.range)}
    <div class="stat-chips">
      <div class="stat-chip"><span>Income</span>${amt(tot.income, { cls: "pos" })}</div>
      <div class="stat-chip"><span>Expense</span>${amt(tot.expense, { cls: "neg" })}</div>
      <div class="stat-chip"><span>${paise(tot.income) > 0 ? `Saved ${Math.max(0, Math.round((paise(tot.net) * 100) / paise(tot.income)))}%` : "Net"}</span>${amt(tot.net, { signed: true })}</div>
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
        <section class="card"><header><h2>📥 Inbox${box.pending ? html` <span class="badge warn">${box.pending}</span>` : ""}</h2><a href="#/inbox">Review</a></header>
          ${box.items.length ? html`<ul class="list" data-inbox>${box.items.slice(0, 4).map((x) => draftRow(x, false))}</ul>`
          : html`<div class="empty">Nothing to review — SMS and Kronos drafts land here</div>`}</section>
        ${soon.length ? html`<section class="card"><header><h2>Upcoming</h2><a href="#/tx">All</a></header>
          <ul class="list" data-soon>${soon.slice(0, 6).map((t, i) => html`<li><a class="item" href="#" data-i="${i}">
            <span class="ic">${t.type === "transfer" ? "⇄" : catIcon(t.category)}</span>
            <span class="main-t"><span class="t">${t.title}</span><span class="s">${fmtShort(t.date)} · ${relDay(t.date, today())}</span></span>
            <span class="end"><span class="amt ${t.type === "income" ? "pos" : t.type === "expense" ? "neg" : ""}">${money(t.amount)}</span></span></a></li>`)}</ul></section>` : ""}
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
  wirePeriod(root, p, tot, () => render(root, ctx).then(() => countUp(root, ".stat-chip .amt")));
  wireHero(root.querySelector("[data-hero]"), ctx);
  wireTips(root);
  root.querySelectorAll("[data-cats] [data-key]").forEach((el) => el.addEventListener("click", () => ctx.go("#/category/" + encodeURIComponent(el.dataset.key))));
  wireRows(root.querySelector("[data-recent]"), recentItems, (t) => (t._pending ? null : openTx(t, () => ctx.refresh())));
  root.querySelectorAll("[data-soon] [data-i]").forEach((a) => a.addEventListener("click", (e) => { e.preventDefault(); openTx(soon[+a.dataset.i], () => ctx.refresh()); }));
  root.querySelectorAll("[data-inbox] [data-draft]").forEach((a) => a.addEventListener("click", (e) => { e.preventDefault(); ctx.go("#/inbox"); }));
}

// ---- net worth trend ----

// timeline → bucket size, how many buckets back and how the change is worded
const SPANS = {
  day: { label: "Day", bucket: "daily", back: (t) => addDays(t, -29), since: "today" },
  week: { label: "Week", bucket: "weekly", back: (t) => addDays(t, -7 * 11), since: "this week" },
  month: { label: "Month", bucket: "monthly", back: (t) => monthsBack(t, 11), since: "this month" },
  year: { label: "Year", bucket: "yearly", back: (t) => `${parseDate(t).getFullYear() - 4}-01-01`, since: "this year" },
};
const SPAN_KEY = "arthik.nwSpan";

function monthsBack(t, n) {
  const d = parseDate(t);
  d.setMonth(d.getMonth() - n, 1);
  return ymd(d);
}

// The chosen timeline, remembered on this device.
function span() {
  if (!S.view.nwSpan) {
    let saved = "";
    try { saved = localStorage.getItem(SPAN_KEY) || ""; } catch { /* storage blocked */ }
    S.view.nwSpan = SPANS[saved] ? saved : "month";
  }
  return S.view.nwSpan;
}

function setSpan(k) {
  S.view.nwSpan = k;
  try { localStorage.setItem(SPAN_KEY, k); } catch { /* storage blocked */ }
}

function wireHero(el, ctx) {
  if (!el) return;
  el.querySelectorAll("[data-span]").forEach((b) => b.addEventListener("click", () => {
    if (b.dataset.span === span()) return;
    setSpan(b.dataset.span);
    el.querySelectorAll("[data-span]").forEach((x) => { x.classList.toggle("on", x === b); x.setAttribute("aria-pressed", x === b); });
    drawTrend(el, ctx);
  }));
  drawTrend(el, ctx);
}

// Loads the series for the chosen timeline and draws the sparkline and the change.
async function drawTrend(el, ctx) {
  const sp = SPANS[span()];
  const t = today();
  const asked = span();
  let rows = [];
  try {
    rows = (await get("/api/reports/networth" + qs({ bucket: sp.bucket, from: sp.back(t), to: t }))).rows;
  } catch (e) {
    el.querySelector("[data-spark]").innerHTML = String(html`<div class="spark-none neg">${e.message}</div>`);
    return;
  }
  if (!ctx.alive() || asked !== span() || !el.isConnected) return;
  const points = rows.map((r) => ({ label: r.label, value: r.net_worth }));
  const box = el.querySelector("[data-spark]");
  box.innerHTML = String(spark(points));
  wireTips(box);
  replay(box, "appear");
  const delta = el.querySelector("[data-delta]");
  if (points.length < 2) { delta.innerHTML = ""; return; }
  const change = paise(points.at(-1).value) - paise(points.at(-2).value);
  const cls = change > 0 ? "pos" : change < 0 ? "neg" : "muted";
  delta.innerHTML = String(change === 0 ? html`<span class="muted">No change ${sp.since}</span>`
    : html`<span class="${cls}">${change > 0 ? "▲" : "▼"} <span class="amt">${money0(String(Math.abs(change) / 100))}</span> ${sp.since}</span>`);
}
