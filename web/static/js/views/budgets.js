// Budgets: budget vs actual for any period. Budgets are set per category (with their
// own period) and scaled to the period you view; they reset every period.

import { html, raw, qs, money, amt, paise, fromPaise } from "../util.js";
import { S, sheet, ro, loading, periodState, periodQuery, periodBar, wirePeriod, catName, catIcon, cat } from "../ui.js";
import { get } from "../api.js";
import { categoryForm } from "./categories.js";

export async function render(root, ctx) {
  const p = periodState("budgets");
  ctx.actions(ro() ? "" : html`<button class="primary" data-set>Set budget</button>`);
  root.innerHTML = String(loading());
  const b = await get("/api/budgets" + qs(periodQuery(p)));
  if (!ctx.alive()) return;
  const t = b.total;
  const cls = (pct) => (pct > 100 ? "over" : pct > 85 ? "near" : "");
  const income = b.lines.filter((l) => l.kind !== "expense");
  const expense = b.lines.filter((l) => l.kind === "expense");
  const lineH = (l) => html`<li><a class="item ${l.parent ? "child" : ""}" href="#/category/${encodeURIComponent(l.category)}" style="display:block">
      <div class="row between"><span class="ellipsis">${catIcon(l.category)} ${catName(l.category)}${l.derived ? html` <small class="faint">(sum of sub-budgets)</small>` : ""}</span>
        <span class="amt">${money(l.spent)} <small class="faint">/ ${money(l.budget)}</small></span></div>
      <div class="bar ${l.kind === "expense" ? cls(l.percent) : ""}" style="margin:6px 0 4px"><i style="width:${Math.min(100, l.percent)}%"></i></div>
      <div class="row between"><small class="muted">${l.percent}%</small><small class="${paise(l.left) < 0 && l.kind === "expense" ? "neg" : "muted"}">${
        l.kind === "expense" ? (paise(l.left) < 0 ? `over by ${money(fromPaise(-paise(l.left)))}` : `${money(l.left)} left`) : (paise(l.left) > 0 ? `${money(l.left)} to go` : "target reached")}</small></div></a></li>`;
  root.innerHTML = String(html`
    ${periodBar(p, b.range)}
    ${paise(t.budget) > 0 ? html`<div class="card">
      <div class="row between"><div class="stat"><div class="label">Spent of budget · ${b.range.label}</div><div class="value">${amt(t.spent)} <small class="muted">/ ${money(t.budget)}</small></div></div>
        <div class="stat" style="text-align:right"><div class="label">${paise(t.left) < 0 ? "Over" : "Left"}</div><div class="value ${paise(t.left) < 0 ? "neg" : "pos"}">${amt(fromPaise(Math.abs(paise(t.left))))}</div></div></div>
      <div class="bar ${cls(t.percent)}" style="margin-top:10px;height:12px"><i style="width:${Math.min(100, t.percent)}%"></i></div>
      ${progressNote(b.range)}</div>` : ""}
    ${expense.length ? html`<div class="card"><header><h2>Expense budgets</h2></header><ul class="list">${expense.map(lineH)}</ul></div>` : ""}
    ${income.length ? html`<div class="card"><header><h2>Income targets</h2></header><ul class="list">${income.map(lineH)}</ul></div>` : ""}
    ${!b.lines.length ? html`<div class="card empty">No budgets yet.${ro() ? "" : " Use “Set budget” to give a category a weekly, monthly, quarterly, half-yearly or yearly budget."}</div>` : ""}
    <p class="muted" style="font-size:.85rem">Budgets reset at the start of every period. A monthly budget shown for a quarter counts three times.</p>`);
  wirePeriod(root, p, b, () => render(root, ctx));
  ctx.actionsEl.querySelector("[data-set]")?.addEventListener("click", () => {
    sheet("Set a budget", html`<label class="f"><span>Category</span><select name="c" autofocus>
      ${S.state.categories.filter((c) => !c.parent).map((c) => [html`<option value="${c.id}">${c.icon || ""} ${c.name}</option>`,
        S.state.categories.filter((k) => k.parent === c.id).map((k) => html`<option value="${k.id}">  › ${k.name}</option>`)])}</select></label>`,
      html`<span class="grow"></span><button data-close>Cancel</button><button class="primary" data-ok>Next</button>`, (d, close) => {
        d.querySelector("[data-ok]").addEventListener("click", () => { const id = d.querySelector("select").value; close(); categoryForm(cat(id), ctx); });
      });
  });
}

/** "Day 6 of 31 — 19% of the period gone" helps judge whether spending is on pace. */
function progressNote(r) {
  const t = S.state.today;
  if (!(r.from <= t && t <= r.to)) return "";
  const day = Math.round((Date.parse(t) - Date.parse(r.from)) / 864e5) + 1;
  const len = Math.round((Date.parse(r.to) - Date.parse(r.from)) / 864e5) + 1;
  return html`<p class="muted" style="margin:8px 0 0;font-size:.85rem">Day ${day} of ${len} — ${Math.round((day * 100) / len)}% of the period gone.</p>`;
}

export { raw };
