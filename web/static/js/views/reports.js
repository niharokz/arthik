// Reports: cash flow, spending and income by category (with drill-down), net worth
// over time, balance sheet, account flow and labels. Every chart has a table view.

import { html, raw, qs, money, amt, paise, parseDate, ymd, fromPaise } from "../util.js";
import { S, today, loading, periodState, periodQuery, periodBar, wirePeriod, catName, catIcon, accName, labelName, cat } from "../ui.js";
import { get } from "../api.js";
import { incomeExpense, line, ranked, wireTips } from "../charts.js";

const TABS = [["cashflow", "Cash flow"], ["spending", "Spending"], ["income", "Income"], ["networth", "Net worth"],
  ["sheet", "Balance sheet"], ["accounts", "Accounts"], ["labels", "Labels"]];

const PRESETS = [["6m", "Last 6 months"], ["12m", "Last 12 months"], ["ytd", "This year"], ["ly", "Last year"], ["24m", "Last 24 months"], ["custom", "Custom"]];

function span(r) {
  const t = parseDate(today());
  const first = (y, m) => ymd(new Date(y, m, 1));
  const last = (y, m) => ymd(new Date(y, m + 1, 0));
  switch (r.preset) {
    case "6m": return { from: first(t.getFullYear(), t.getMonth() - 5), to: last(t.getFullYear(), t.getMonth()) };
    case "24m": return { from: first(t.getFullYear(), t.getMonth() - 23), to: last(t.getFullYear(), t.getMonth()) };
    case "ytd": return { from: first(t.getFullYear(), 0), to: last(t.getFullYear(), 11) };
    case "ly": return { from: first(t.getFullYear() - 1, 0), to: last(t.getFullYear() - 1, 11) };
    case "custom": return { from: r.from, to: r.to };
    default: return { from: first(t.getFullYear(), t.getMonth() - 11), to: last(t.getFullYear(), t.getMonth()) };
  }
}

function rangeBar(r) {
  return html`<div class="period" data-range>
    <select data-preset>${PRESETS.map(([k, l]) => html`<option value="${k}" ${k === r.preset ? raw("selected") : ""}>${l}</option>`)}</select>
    ${r.preset === "custom" ? html`<input type="date" data-rfrom value="${r.from}"> – <input type="date" data-rto value="${r.to}">` : ""}
    <select data-bucket aria-label="Group by">${[["weekly", "by week"], ["monthly", "by month"], ["quarterly", "by quarter"], ["halfyearly", "by half-year"], ["yearly", "by year"]]
      .map(([k, l]) => html`<option value="${k}" ${k === r.bucket ? raw("selected") : ""}>${l}</option>`)}</select></div>`;
}

function wireRange(root, r, rerender) {
  const el = root.querySelector("[data-range]");
  if (!el) return;
  el.querySelector("[data-preset]").addEventListener("change", (e) => {
    const cur = span(r);
    r.preset = e.target.value;
    if (r.preset === "custom") Object.assign(r, cur);
    rerender();
  });
  el.querySelector("[data-bucket]").addEventListener("change", (e) => { r.bucket = e.target.value; rerender(); });
  const f = el.querySelector("[data-rfrom]"), t = el.querySelector("[data-rto]");
  [f, t].forEach((x) => x?.addEventListener("change", () => { if (f.value && t.value) { r.from = f.value; r.to = t.value; rerender(); } }));
}

export async function render(root, ctx) {
  const tab = ctx.arg && TABS.some(([k]) => k === ctx.arg) ? ctx.arg : S.view.repTab || "cashflow";
  S.view.repTab = tab;
  S.view.repRange = S.view.repRange || { preset: "12m", bucket: "monthly", from: "", to: "" };
  const r = S.view.repRange;
  const p = periodState("reports");
  root.innerHTML = String(html`<div class="seg" style="margin-bottom:14px;overflow-x:auto;flex-wrap:nowrap;max-width:100%">
    ${TABS.map(([k, l]) => html`<button data-tab="${k}" class="${k === tab ? "on" : ""}" style="white-space:nowrap">${l}</button>`)}</div><div data-body>${loading()}</div>`);
  root.querySelectorAll("[data-tab]").forEach((b) => b.addEventListener("click", () => ctx.go("#/reports/" + b.dataset.tab)));
  const body = root.querySelector("[data-body]");
  const rerender = () => render(root, ctx);
  const s = span(r);

  if (tab === "cashflow") {
    const d = await get("/api/reports/cashflow" + qs({ ...s, bucket: r.bucket }));
    if (!ctx.alive()) return;
    body.innerHTML = String(html`${rangeBar(r)}
      <div class="grid three"><div class="card stat"><div class="label">Income</div><div class="value pos">${amt(d.total.income)}</div></div>
        <div class="card stat"><div class="label">Expense</div><div class="value neg">${amt(d.total.expense)}</div></div>
        <div class="card stat"><div class="label">Net</div><div class="value">${amt(d.total.net, { signed: true })}</div>
          <div class="sub">${paise(d.total.income) > 0 ? `savings rate ${Math.round((paise(d.total.net) * 100) / paise(d.total.income))}%` : ""}</div></div></div>
      <div class="card" style="margin-top:14px">${incomeExpense(d.rows, { height: 240 })}</div>
      <div class="card scroll-x"><table class="tbl"><thead><tr><th>Period</th><th class="r">Income</th><th class="r">Expense</th><th class="r">Net</th></tr></thead>
        <tbody>${d.rows.map((x) => html`<tr><td><a href="#/tx?from=${x.from}&to=${x.to}">${x.label}</a></td><td class="r">${amt(x.income)}</td><td class="r">${amt(x.expense)}</td><td class="r">${amt(x.net, { signed: true })}</td></tr>`)}</tbody></table></div>`);
    wireRange(body, r, rerender);
  }

  if (tab === "spending" || tab === "income") {
    const kind = tab === "spending" ? "expense" : "income";
    const parent = S.view.repParent && cat(S.view.repParent)?.kind === kind ? S.view.repParent : "";
    const d = await get("/api/reports/categories" + qs({ ...periodQuery(p), kind, parent }));
    if (!ctx.alive()) return;
    const rows = d.rows.map((x) => ({ key: x.category, name: x.category === parent ? catName(x.category) + " (itself)" : catName(x.category), icon: catIcon(x.category), value: x.total, note: x.has_children ? "tap to open" : "" }));
    body.innerHTML = String(html`${periodBar(p, d.range)}
      ${parent ? html`<p><button class="ghost" data-up>‹ All categories</button> <b>${catIcon(parent)} ${catName(parent)}</b></p>` : ""}
      <div class="card"><header><h2>${kind === "expense" ? "Spent" : "Received"} · ${d.range.label}</h2><b class="amt">${money(d.total)}</b></header>
        ${rows.length ? html`<div data-rows>${ranked(rows, d.total)}</div>` : html`<div class="empty">Nothing in this period</div>`}</div>`);
    wirePeriod(body, p, d, rerender);
    body.querySelector("[data-up]")?.addEventListener("click", () => { S.view.repParent = ""; rerender(); });
    body.querySelectorAll("[data-rows] [data-key]").forEach((el) => el.addEventListener("click", () => {
      const row = d.rows.find((x) => x.category === el.dataset.key);
      if (row?.has_children && !parent) { S.view.repParent = row.category; rerender(); } else ctx.go("#/category/" + encodeURIComponent(el.dataset.key));
    }));
  }

  if (tab === "networth") {
    const d = await get("/api/reports/networth" + qs({ ...s, bucket: r.bucket }));
    if (!ctx.alive()) return;
    const lastRow = d.rows[d.rows.length - 1], firstRow = d.rows[0];
    const change = lastRow && firstRow ? paise(lastRow.net_worth) - paise(firstRow.net_worth) : 0;
    body.innerHTML = String(html`${rangeBar(r)}
      <div class="grid three"><div class="card stat"><div class="label">Net worth</div><div class="value">${amt(lastRow?.net_worth || "0")}</div></div>
        <div class="card stat"><div class="label">Change over the range</div><div class="value">${amt(fromPaise(change), { signed: true })}</div></div>
        <div class="card stat"><div class="label">Owed</div><div class="value neg">${amt(fromPaise(-paise(lastRow?.liabilities || "0")))}</div></div></div>
      <div class="card" style="margin-top:14px">${line(d.rows.map((x) => ({ label: x.label, value: x.net_worth })), { height: 240, name: "Net worth" })}</div>
      <div class="card scroll-x"><table class="tbl"><thead><tr><th>At end of</th><th class="r">Assets</th><th class="r">Liabilities</th><th class="r">Net worth</th></tr></thead>
        <tbody>${d.rows.map((x) => html`<tr><td>${x.label}</td><td class="r">${amt(x.assets)}</td><td class="r">${amt(x.liabilities)}</td><td class="r"><b>${amt(x.net_worth)}</b></td></tr>`)}</tbody></table></div>`);
    wireRange(body, r, rerender);
  }

  if (tab === "sheet") {
    S.view.sheetDate = S.view.sheetDate || today();
    const d = await get("/api/reports/balance-sheet" + qs({ date: S.view.sheetDate }));
    if (!ctx.alive()) return;
    const side = (cls) => d.groups.filter((g) => g.class === cls).map((g) => html`<div class="group-h"><span>${g.label}</span><span class="amt">${money(g.total)}</span></div>
      <ul class="list">${g.accounts.map((a) => html`<li><a class="item" href="#/account/${encodeURIComponent(a.id)}"><span class="main-t"><span class="t">${a.name}</span></span><span class="end">${amt(a.balance)}</span></a></li>`)}</ul>`);
    body.innerHTML = String(html`<div class="period"><span>As of</span><input type="date" data-sd value="${S.view.sheetDate}" style="width:auto"></div>
      <div class="grid two"><div class="card"><header><h2>Assets</h2><b class="amt pos">${money(d.assets)}</b></header>${side("asset")}</div>
        <div class="card"><header><h2>Liabilities</h2><b class="amt neg">${money(d.liabilities)}</b></header>${side("liability")}</div></div>
      <div class="card row between"><h2>Net worth</h2><span class="hero amt">${money(d.net_worth)}</span></div>`);
    body.querySelector("[data-sd]").addEventListener("change", (e) => { S.view.sheetDate = e.target.value || today(); rerender(); });
  }

  if (tab === "accounts") {
    const d = await get("/api/reports/accounts" + qs(periodQuery(p)));
    if (!ctx.alive()) return;
    body.innerHTML = String(html`${periodBar(p, d.range)}
      <div class="card scroll-x"><table class="tbl"><thead><tr><th>Account</th><th class="r">Opening</th><th class="r">In</th><th class="r">Out</th><th class="r">Closing</th></tr></thead>
        <tbody>${d.rows.map((x) => html`<tr><td><a href="#/account/${encodeURIComponent(x.account)}">${accName(x.account)}</a></td><td class="r">${amt(x.opening)}</td>
          <td class="r pos">${amt(x.inflow)}</td><td class="r neg">${amt(x.outflow)}</td><td class="r"><b>${amt(x.closing)}</b></td></tr>`)}</tbody></table></div>`);
    wirePeriod(body, p, d, rerender);
  }

  if (tab === "labels") {
    const d = await get("/api/reports/labels" + qs(periodQuery(p)));
    if (!ctx.alive()) return;
    body.innerHTML = String(html`${periodBar(p, d.range)}
      <div class="card scroll-x">${d.rows.length ? html`<table class="tbl"><thead><tr><th>Label</th><th class="r">Transactions</th><th class="r">Income</th><th class="r">Expense</th></tr></thead>
        <tbody>${d.rows.map((x) => html`<tr><td><a href="#/tx?label=${encodeURIComponent(x.label)}">#${labelName(x.label)}</a></td><td class="r">${x.count}</td><td class="r">${amt(x.income)}</td><td class="r">${amt(x.expense)}</td></tr>`)}</tbody></table>`
        : html`<div class="empty">No labelled transactions in this period</div>`}</div>`);
    wirePeriod(body, p, d, rerender);
  }
  wireTips(body);
}
