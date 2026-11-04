// Home: the blocks you chose, in your order (Settings are saved in settings.md as
// `home`): net worth with its trend, the period's income/expense/savings, inbox,
// spending, budgets, upcoming, credit cards, accounts, recent transactions, cash flow,
// and blocks of your own for any account, category or label.

import { html, raw, money, money0, amt, paise, qs, fmtShort, relDay, parseDate, ymd, addDays, replay, countUp } from "../util.js";
import { S, periodState, periodQuery, periodBar, wirePeriod, catName, catIcon, today, loading, ro, acc, accName, cat, label, sheet, toast, celebrate } from "../ui.js";
import { get, put } from "../api.js";
import { haptic } from "../native.js";
import { incomeExpense, ranked, spark, wireTips } from "../charts.js";
import { grouped, wireRows, pendingCreates } from "../txlist.js";
import { openTx } from "../txform.js";
import { draftRow } from "./inbox.js";

// Built-in blocks: type → [name, icon, full width on wide screens].
const BLOCKS = {
  networth: ["Net worth", "💰", true], period: ["Income, expense & savings", "📆", true], inbox: ["Inbox", "📥"],
  spending: ["Spending by category", "🗂️"], budgets: ["Budgets", "🎯"], upcoming: ["Upcoming", "⏳"], cards: ["Credit cards", "💳"],
  accounts: ["Accounts", "🏦"], recent: ["Recent transactions", "🧾"], cashflow: ["Cash flow · 6 months", "📊"],
};
// Blocks of your own: type → [name, icon].
const CUSTOM = { account: ["Account", "🏦"], category: ["Category", "🗂️"], label: ["Label", "🏷️"] };
const DEFAULT = Object.keys(BLOCKS).map((type) => ({ type }));

/** The saved layout, or the default one. */
const layout = () => (S.state.settings.home?.length ? S.state.settings.home : DEFAULT);

// A block whose account, category or label is gone is skipped.
const exists = (b) => BLOCKS[b.type] || (b.type === "account" && acc(b.ref)) || (b.type === "category" && cat(b.ref)) || (b.type === "label" && label(b.ref));

export async function render(root, ctx) {
  const st = S.state;
  const p = periodState("dash");
  const blocks = layout().filter(exists);
  const has = (t) => blocks.some((b) => b.type === t);
  ctx.actions(ro() ? "" : html`<button class="ghost icon" data-customize aria-label="Customize home" title="Customize home">✎</button>`);
  root.innerHTML = String(loading());
  const sixAgo = parseDate(today());
  sixAgo.setMonth(sixAgo.getMonth() - 5, 1);
  const ahead = addDays(today(), st.settings.upcoming_days);
  const none = Promise.resolve(null);
  const tot = await get("/api/categories/totals" + qs(periodQuery(p)));
  const range = tot.range;
  const custom = blocks.filter((b) => CUSTOM[b.type]);
  const [bud, up, recent, flow, box, ...own] = await Promise.all([
    has("budgets") ? get("/api/budgets" + qs(periodQuery(p))) : none,
    has("upcoming") ? get("/api/transactions" + qs({ when: "scheduled", to: ahead, limit: 50 })) : none,
    has("recent") ? get("/api/transactions" + qs({ limit: 8, when: "posted" })) : none,
    has("cashflow") ? get("/api/reports/cashflow" + qs({ from: ymd(sixAgo), to: today() })) : none,
    has("inbox") ? get("/api/inbox").catch(() => ({ items: [], pending: 0 })) : none,
    ...custom.map((b) => get("/api/transactions" + qs(b.type === "account" ? { account: b.ref, limit: 5, when: "posted" }
      : { [b.type]: b.ref, from: range.from, to: range.to, limit: 5 }))),
  ]);
  if (!ctx.alive()) return;
  const data = { st, p, tot, bud, up, recent, flow, box, own: new Map(custom.map((b, i) => [b, own[i]])) };
  root.innerHTML = String(html`<div class="home">${blocks.map((b) => block(b, data))}</div>
    ${ro() ? "" : html`<div class="home-foot"><button class="ghost" data-customize>✎ Customize home</button></div>`}`);
  wire(root, ctx, blocks, data);
}

function block(b, d) {
  const out = (BLOCKS[b.type] ? BUILT[b.type] : OWN[b.type])(b, d);
  if (!out) return "";
  const wide = BLOCKS[b.type]?.[2];
  return html`<div class="home-b ${wide ? "wide" : ""}" data-block="${b.type}${b.ref ? "/" + b.ref : ""}">${out}</div>`;
}

const head = (b, title, link, linkText = "All") => html`<header><h2>${b.title || title}</h2>${link ? html`<a href="${link}">${linkText}</a>` : ""}</header>`;

const BUILT = {
  networth: (b, { st }) => html`<section class="card hero" data-hero>
      <div class="hero-head">
        <div class="hero-main"><div class="label">${b.title || "Net worth"}</div>
          <div class="hero-value">${amt(st.summary.net_worth)}</div>
          <div class="delta" data-delta></div></div>
        <div class="seg mini" role="group" aria-label="Net worth timeline">${Object.entries(SPANS).map(([k, v]) =>
          html`<button data-span="${k}" class="${k === span() ? "on" : ""}" aria-pressed="${k === span()}">${v.label}</button>`)}</div>
      </div>
      <div class="hero-spark" data-spark><div class="sk spark-sk"></div></div>
      <div class="hero-foot"><span>Assets ${amt(st.summary.assets)}</span><span>Owed ${amt(String(-paise(st.summary.liabilities) / 100))}</span>
        ${st.summary.excluded_ids?.length ? html`<span title="${st.summary.excluded_ids.map(accName).join(", ")}">Not counted ${amt(st.summary.excluded, { signed: true })}</span>` : ""}</div>
      ${st.summary.checks_off || st.summary.unchecked ? html`<a class="hero-check" href="#/accounts">${st.summary.checks_off ? `⚠️ ${st.summary.checks_off} don't match the bank` : ""}${st.summary.checks_off && st.summary.unchecked ? " · " : ""}${st.summary.unchecked ? `${st.summary.unchecked} not checked in ${st.settings.balance_check_days} days` : ""}</a>` : ""}
    </section>`,
  period: (b, { p, tot }) => html`<div data-period>${periodBar(p, tot.range)}
    <div class="stat-chips">
      <div class="stat-chip"><span>Income</span>${amt(tot.income, { cls: "pos" })}</div>
      <div class="stat-chip"><span>Expense</span>${amt(tot.expense, { cls: "neg" })}</div>
      <div class="stat-chip"><span>${paise(tot.income) > 0 ? `Saved ${Math.max(0, Math.round((paise(tot.net) * 100) / paise(tot.income)))}%` : "Net"}</span>${amt(tot.net, { signed: true })}</div>
    </div></div>`,
  inbox: (b, { box }) => html`<section class="card">${head(b, html`📥 Inbox${box.pending ? html` <span class="badge warn">${box.pending}</span>` : ""}`, "#/inbox", "Review")}
      ${box.items.length ? html`<ul class="list" data-inbox>${box.items.slice(0, 4).map((x) => draftRow(x, false))}</ul>`
      : html`<div class="empty">Nothing to review</div>`}</section>`,
  spending: (b, { st, tot }) => {
    const top = st.categories.filter((c) => !c.parent && c.kind === "expense" && paise(tot.totals[c.id]) > 0)
      .map((c) => ({ key: c.id, name: c.name, icon: c.icon, value: tot.totals[c.id] }))
      .sort((x, y) => paise(y.value) - paise(x.value)).slice(0, 6);
    return html`<section class="card">${head(b, `Spending · ${tot.range.label}`, "#/categories")}
      ${top.length ? html`<div data-cats>${ranked(top, tot.expense)}</div>` : html`<div class="empty">Nothing spent in this period</div>`}</section>`;
  },
  budgets: (b, { bud }) => html`<section class="card">${head(b, "Budgets", "#/budgets")}
    ${paise(bud.total.budget) > 0 ? html`
      <div class="row between"><span>${amt(bud.total.spent)} of ${amt(bud.total.budget)}</span><span class="${paise(bud.total.left) < 0 ? "neg" : "muted"}">${paise(bud.total.left) < 0 ? "over by " : ""}${amt(String(Math.abs(paise(bud.total.left)) / 100))}${paise(bud.total.left) >= 0 ? " left" : ""}</span></div>
      <div class="bar ${bud.total.percent > 100 ? "over" : bud.total.percent > 85 ? "near" : ""}" style="margin:8px 0 12px"><i style="width:${Math.min(100, bud.total.percent)}%"></i></div>
      ${bud.lines.filter((l) => !l.parent).slice(0, 4).map((l) => html`<a class="bline" href="#/category/${encodeURIComponent(l.category)}">
        <div class="row between"><span>${catIcon(l.category)} ${catName(l.category)}</span><small>${amt(l.spent)} / ${amt(l.budget)}</small></div>
        <div class="bar ${l.percent > 100 ? "over" : l.percent > 85 ? "near" : ""}"><i style="width:${Math.min(100, l.percent)}%"></i></div></a>`)}`
    : html`<div class="empty">No budgets yet — set one on a category</div>`}</section>`,
  upcoming: (b, { up }) => {
    const soon = up.items.sort((x, y) => (x.date < y.date ? -1 : 1)).slice(0, 6);
    return html`<section class="card">${head(b, "Upcoming", "#/tx")}
      ${soon.length ? html`<ul class="list" data-soon>${soon.map((t) => html`<li><a class="item" href="#" data-id="${t.id}">
        <span class="ic">${t.type === "transfer" ? "⇄" : catIcon(t.category)}</span>
        <span class="main-t"><span class="t">${t.title}</span><span class="s">${fmtShort(t.date)} · ${relDay(t.date, today())}</span></span>
        <span class="end"><span class="amt ${t.type === "income" ? "pos" : t.type === "expense" ? "neg" : ""}">${money(t.amount)}</span></span></a></li>`)}</ul>`
      : html`<div class="empty">Nothing scheduled</div>`}</section>`;
  },
  cards: (b, { st }) => {
    const cards = st.accounts.filter((a) => a.card && !a.archived);
    if (!cards.length) return "";
    return html`<section class="card">${head(b, "Credit cards")}
      ${cards.map((a) => html`<a class="item" href="#/account/${encodeURIComponent(a.id)}"><span class="ic">${a.icon}</span>
        <span class="main-t"><span class="t">${a.name}</span><span class="s">${a.card.due_date ? `Due ${fmtShort(a.card.due_date)} · ` : ""}${a.credit_limit ? `${a.card.utilised}% used` : ""}</span></span>
        <span class="end"><span class="amt neg">${money(String(-paise(a.balance) / 100))}</span><span class="s">${a.credit_limit ? html`avail ${money(a.card.available)}` : ""}</span></span></a>`)}</section>`;
  },
  accounts: (b, { st }) => {
    const visible = st.accounts.filter((a) => !a.archived && !a.hidden);
    return html`<section class="card">${head(b, "Accounts", "#/accounts")}
      ${visible.length ? html`<ul class="list">${visible.slice(0, 8).map((a) => html`<li><a class="item" href="#/account/${encodeURIComponent(a.id)}">
        <span class="ic">${a.icon}</span><span class="main-t"><span class="t">${a.name}</span><span class="s">${a.type_label}</span></span>
        <span class="end">${amt(a.balance, { cls: paise(a.balance) < 0 ? "neg" : "" })}</span></a></li>`)}</ul>`
      : html`<div class="empty">${ro() ? "No accounts" : html`No accounts yet. <a href="#/accounts">Add your first account</a>`}</div>`}</section>`;
  },
  recent: (b, d) => {
    d.recentItems = [...pendingCreates(), ...d.recent.items];
    return html`<section class="card">${head(b, "Recent", "#/tx")}<div data-recent>${grouped(d.recentItems)}</div></section>`;
  },
  cashflow: (b, { flow }) => html`<section class="card">${head(b, "Cash flow · 6 months", "#/reports", "Reports")}${incomeExpense(flow.rows)}</section>`,
};

const OWN = {
  account: (b, { own }) => {
    const a = acc(b.ref);
    return html`<section class="card own">${head(b, html`${a.icon} ${a.name}`, `#/account/${encodeURIComponent(a.id)}`)}
      <div class="own-big">${amt(a.balance, { cls: paise(a.balance) < 0 ? "neg" : "" })}
        <small>${a.card ? (a.credit_limit ? `available ${money(a.card.available)}` : "") + (a.card.due_date ? ` · due ${fmtShort(a.card.due_date)}` : "") : a.type_label}</small></div>
      <div data-own="${b.type}/${b.ref}">${grouped(own.get(b).items, { account: a.id, empty: "No transactions yet" })}</div></section>`;
  },
  category: (b, { own, tot }) => {
    const c = cat(b.ref);
    const v = tot.totals[c.id] || "0", bu = tot.budgets[c.id];
    const pct = bu ? Math.round((paise(v) * 100) / paise(bu)) : 0;
    return html`<section class="card own">${head(b, html`${catIcon(c.id)} ${c.name}`, `#/category/${encodeURIComponent(c.id)}`)}
      <div class="own-big">${amt(v)}<small>${tot.range.label}${bu ? ` · ${pct}% of ${money(bu)}` : ""}</small></div>
      ${bu ? html`<div class="bar ${pct > 100 ? "over" : pct > 85 ? "near" : ""}" style="margin-bottom:8px"><i style="width:${Math.min(100, pct)}%"></i></div>` : ""}
      <div data-own="${b.type}/${b.ref}">${grouped(own.get(b).items, { empty: "Nothing in this period" })}</div></section>`;
  },
  label: (b, { own, tot }) => {
    const l = label(b.ref), r = own.get(b);
    return html`<section class="card own">${head(b, html`# ${l.name}`, `#/tx?label=${encodeURIComponent(l.id)}`)}
      <div class="own-big">${amt(r.sums.expense, { cls: "neg" })}<small>spent · ${tot.range.label}${paise(r.sums.income) ? html` · received ${money(r.sums.income)}` : ""}</small></div>
      <div data-own="${b.type}/${b.ref}">${grouped(r.items, { empty: "Nothing in this period" })}</div></section>`;
  },
};

function wire(root, ctx, blocks, d) {
  const again = () => render(root, ctx);
  root.querySelectorAll("[data-customize]").forEach((x) => x.addEventListener("click", () => customize(ctx)));
  ctx.actionsEl.querySelector("[data-customize]")?.addEventListener("click", () => customize(ctx));
  const per = root.querySelector("[data-period]");
  if (per) wirePeriod(per, d.p, d.tot, () => again().then(() => countUp(root, ".stat-chip .amt")));
  wireHero(root.querySelector("[data-hero]"), ctx);
  wireTips(root);
  root.querySelectorAll("[data-cats] [data-key]").forEach((el) => el.addEventListener("click", () => ctx.go("#/category/" + encodeURIComponent(el.dataset.key))));
  const recent = root.querySelector("[data-recent]");
  if (recent) wireRows(recent, d.recentItems, (t) => (t._pending ? null : openTx(t, () => ctx.refresh())));
  root.querySelectorAll("[data-soon] [data-id]").forEach((a) => a.addEventListener("click", (e) => {
    e.preventDefault();
    openTx(d.up.items.find((t) => t.id === a.dataset.id), () => ctx.refresh());
  }));
  root.querySelectorAll("[data-inbox] [data-draft]").forEach((a) => a.addEventListener("click", () => ctx.go("#/inbox")));
  for (const [b, r] of d.own) {
    const el = root.querySelector(`[data-own="${CSS.escape(b.type + "/" + b.ref)}"]`);
    if (el) wireRows(el, r.items, (t) => openTx(t, () => ctx.refresh()));
  }
}

// ---- customize: show, hide, order and create blocks ----

const nameOf = (b) => b.title || (BLOCKS[b.type] ? BLOCKS[b.type][0]
  : b.type === "account" ? acc(b.ref)?.name : b.type === "category" ? catName(b.ref) : "# " + (label(b.ref)?.name || b.ref));
const iconOf = (b) => (BLOCKS[b.type] ? BLOCKS[b.type][1] : b.type === "account" ? acc(b.ref)?.icon || "🏦" : b.type === "category" ? catIcon(b.ref) : "🏷️");

function customize(ctx) {
  const st = S.state;
  let list = layout().filter(exists).map((b) => ({ ...b }));
  let kind = "account", moved = -1;
  const options = (k) => k === "account" ? st.accounts.filter((a) => !a.archived).map((a) => [a.id, `${a.icon} ${a.name}`])
    : k === "category" ? st.categories.map((c) => [c.id, `${catIcon(c.id)} ${catName(c.id, true)}`])
      : st.labels.map((l) => [l.id, "# " + l.name]);
  const body = () => html`
    <p class="muted" style="margin-top:0">Top to bottom, as on the home screen. Saved in settings.md, so the phone and the web match.</p>
    <ol class="list home-edit">${list.map((b, i) => html`<li class="menu-row ${i === moved ? "moved" : ""}">
      <span class="ic">${iconOf(b)}</span><span class="grow">${nameOf(b)}${CUSTOM[b.type] ? html` <small class="faint">${CUSTOM[b.type][0].toLowerCase()}</small>` : ""}</span>
      <button class="icon ghost" data-up="${i}" aria-label="Move up" ${i === 0 ? raw("disabled") : ""}>▲</button>
      <button class="icon ghost" data-down="${i}" aria-label="Move down" ${i === list.length - 1 ? raw("disabled") : ""}>▼</button>
      <button class="icon ghost danger-t" data-rm="${i}" aria-label="Remove">✕</button></li>`)}</ol>
    ${Object.keys(BLOCKS).some((t) => !list.some((b) => b.type === t)) ? html`<h3 style="margin:16px 0 8px">Add</h3>
      <div class="row wrap">${Object.entries(BLOCKS).filter(([t]) => !list.some((b) => b.type === t)).map(([t, [n, i]]) =>
        html`<button class="chip-btn" data-add="${t}">${i} ${n}</button>`)}</div>` : ""}
    <h3 style="margin:16px 0 8px">Create a block</h3>
    <div class="fields two">
      <label class="f"><span>Show</span><select data-kind>${Object.entries(CUSTOM).map(([k, [n, i]]) => html`<option value="${k}" ${k === kind ? raw("selected") : ""}>${i} ${n}</option>`)}</select></label>
      <label class="f"><span>Which</span><select data-ref>${options(kind).map(([v, n]) => html`<option value="${v}">${n}</option>`)}</select></label>
    </div>
    <div class="row"><input data-title placeholder="Title (optional)" maxlength="60" class="grow"><button data-create ${options(kind).length ? "" : raw("disabled")}>Add block</button></div>`;
  sheet("Customize home", body(), html`<button class="ghost" data-reset>Default</button><span class="grow"></span><button data-close>Cancel</button><button class="primary" data-save>Save</button>`, (d, close) => {
    const redraw = () => { d.querySelector(".sheet-b").innerHTML = String(body()); bind(); };
    const bind = () => {
      const $$ = (sel, fn) => d.querySelectorAll(sel).forEach((x) => x.addEventListener("click", () => fn(x)));
      $$("[data-up]", (x) => { const i = +x.dataset.up; [list[i - 1], list[i]] = [list[i], list[i - 1]]; moved = i - 1; haptic("tick"); redraw(); });
      $$("[data-down]", (x) => { const i = +x.dataset.down; [list[i + 1], list[i]] = [list[i], list[i + 1]]; moved = i + 1; haptic("tick"); redraw(); });
      $$("[data-rm]", (x) => { list.splice(+x.dataset.rm, 1); moved = -1; redraw(); });
      $$("[data-add]", (x) => { list.push({ type: x.dataset.add }); moved = list.length - 1; redraw(); });
      d.querySelector("[data-kind]").addEventListener("change", (e) => { kind = e.target.value; redraw(); });
      d.querySelector("[data-create]").addEventListener("click", () => {
        const ref = d.querySelector("[data-ref]").value, title = d.querySelector("[data-title]").value.trim();
        if (!ref) return;
        if (list.some((b) => b.type === kind && b.ref === ref)) return toast("That block is already on your home screen", true);
        list.push({ type: kind, ref, ...(title ? { title } : {}) });
        moved = list.length - 1;
        redraw();
      });
    };
    bind();
    d.querySelector("[data-reset]").addEventListener("click", () => { list = DEFAULT.map((b) => ({ ...b })); moved = -1; redraw(); });
    d.querySelector("[data-save]").addEventListener("click", async () => {
      if (!list.length) return toast("Keep at least one block", true);
      try {
        await put("/api/settings", { ...S.state.settings, home: list });
        close();
        celebrate("ok");
        toast("Home updated");
        ctx.refresh();
      } catch (e) { toast(e.message, true); }
    });
  });
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
