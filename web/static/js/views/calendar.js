// Calendar: a month grid with each day's income and expense (future-dated and
// reminder items marked), and the selected day's transactions underneath.

import { html, qs, money, amt, short, paise, fmtDay } from "../util.js";
import { S, today, loading } from "../ui.js";
import { get } from "../api.js";
import { grouped, wireRows } from "../txlist.js";
import { openTx } from "../txform.js";

const DOW = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

export async function render(root, ctx) {
  S.view.calMonth = S.view.calMonth || today().slice(0, 7);
  S.view.calDay = S.view.calDay || today();
  root.innerHTML = String(loading());
  const m = await get("/api/reports/calendar" + qs({ month: S.view.calMonth }));
  const dayTx = await get("/api/transactions" + qs({ from: S.view.calDay, to: S.view.calDay, upcoming: "1", limit: 200 }));
  if (!ctx.alive()) return;
  const ws = ["sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"].indexOf(S.state.settings.week_start || "monday");
  const first = new Date(m.from + "T00:00:00");
  const lead = (first.getDay() - ws + 7) % 7;
  const daysIn = Number(m.to.slice(8, 10));
  const byDay = Object.fromEntries(m.days.map((d) => [d.date, d]));
  let inc = 0, exp = 0;
  m.days.forEach((d) => { if (d.date <= today()) { inc += paise(d.income); exp += paise(d.expense); } });
  const cells = [];
  for (let i = 0; i < lead; i++) cells.push(html`<button class="out" tabindex="-1" aria-hidden="true"></button>`);
  for (let d = 1; d <= daysIn; d++) {
    const ds = `${m.month}-${String(d).padStart(2, "0")}`;
    const x = byDay[ds];
    cells.push(html`<button data-day="${ds}" class="${ds === today() ? "today" : ""} ${ds === S.view.calDay ? "sel" : ""}">
      <span class="d">${d}</span>
      ${x && paise(x.income) ? html`<span class="pos amt">+${short(paise(x.income))}</span>` : ""}
      ${x && paise(x.expense) ? html`<span class="neg amt">−${short(paise(x.expense))}</span>` : ""}
      ${x && x.upcoming ? html`<span class="up">● ${x.upcoming}</span>` : ""}</button>`);
  }
  const label = first.toLocaleDateString("en-IN", { month: "long", year: "numeric" });
  root.innerHTML = String(html`
    <div class="period"><button class="icon" data-m="${m.prev}" aria-label="Previous month">‹</button><span class="label">${label}</span>
      <button class="icon" data-m="${m.next}" aria-label="Next month">›</button>${S.view.calMonth !== today().slice(0, 7) ? html`<button class="ghost" data-m="${today().slice(0, 7)}">Today</button>` : ""}</div>
    <div class="grid two">
      <div class="card">
        <div class="cal">${[...Array(7)].map((_, i) => html`<div class="dow">${DOW[(i + ws) % 7]}</div>`)}${cells}</div>
        <div class="row between" style="margin-top:12px;font-size:.9rem"><span>Income <b class="pos">${money(inc / 100)}</b></span><span>Expense <b class="neg">${money(exp / 100)}</b></span>
          <span class="muted"><span class="up" style="color:var(--warn)">●</span> scheduled</span></div>
      </div>
      <div class="card"><header><h2>${fmtDay(S.view.calDay)}</h2></header><div data-list>${grouped(dayTx.items, { empty: "Nothing on this day" })}</div></div>
    </div>`);
  root.querySelectorAll("[data-m]").forEach((b) => b.addEventListener("click", () => {
    S.view.calMonth = b.dataset.m;
    S.view.calDay = b.dataset.m === today().slice(0, 7) ? today() : b.dataset.m + "-01";
    render(root, ctx);
  }));
  root.querySelectorAll("[data-day]").forEach((b) => b.addEventListener("click", () => { S.view.calDay = b.dataset.day; render(root, ctx); }));
  wireRows(root.querySelector("[data-list]"), dayTx.items, (t) => (t.upcoming ? ctx.go("#/scheduled") : openTx(t, () => ctx.refresh())));
}

export { amt };
