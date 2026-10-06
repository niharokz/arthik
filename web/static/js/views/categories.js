// Categories: the two-level tree with totals for any period (default from settings:
// weekly … yearly or a custom range), category detail with trend chart, and
// add/edit/delete-with-move.

import { html, raw, qs, money, amt, paise, fromPaise, calc } from "../util.js";
import { S, sheet, toast, confirmSheet, formData, ro, loading, periodState, periodQuery, periodBar, wirePeriod, PERIODS, kids, cat } from "../ui.js";
import { get, post, put, del } from "../api.js";
import { columns, wireTips } from "../charts.js";
import { grouped, wireRows } from "../txlist.js";
import { openTx } from "../txform.js";

const KINDS = [["expense", "Expense"], ["income", "Income"], ["both", "Both ways"]];

export async function render(root, ctx) {
  if (ctx.name === "category" && ctx.arg) return detail(root, ctx, ctx.arg);
  const st = S.state;
  const p = periodState("cats");
  S.view.catKind = S.view.catKind || "expense";
  const kind = S.view.catKind;
  ctx.actions(ro() ? "" : html`<button class="primary" data-new>Add</button>`);
  root.innerHTML = String(loading());
  const tot = await get("/api/categories/totals" + qs(periodQuery(p)));
  if (!ctx.alive()) return;
  const parents = st.categories.filter((c) => !c.parent && c.kind === kind);
  const hasBoth = st.categories.some((c) => c.kind === "both");
  const sum = parents.reduce((n, c) => n + paise(tot.totals[c.id]), 0);
  const line = (c, child) => {
    const v = tot.totals[c.id] || "0";
    const b = tot.budgets[c.id];
    const pct = b ? Math.round((paise(v) * 100) / paise(b)) : 0;
    return html`<li><a class="item ${child ? "child" : ""}" href="#/category/${encodeURIComponent(c.id)}">
      <span class="ic" style="${c.color ? `background:${c.color}22` : ""}">${c.icon || (child ? "·" : "🏷️")}</span>
      <span class="main-t"><span class="t">${c.name}${c.hidden ? html` <span class="badge">hidden</span>` : ""}</span>
        ${b ? html`<span class="s">${money(v)} of ${money(b)} budget</span><div class="bar ${pct > 100 ? "over" : pct > 85 ? "near" : ""}" style="margin-top:4px"><i style="width:${Math.min(100, pct)}%"></i></div>` : ""}</span>
      <span class="end">${amt(v, { cls: paise(v) === 0 ? "faint" : "" })}${!child && sum ? html`<span class="s">${Math.round((paise(v) * 1000) / sum) / 10}%</span>` : ""}</span></a></li>`;
  };
  root.innerHTML = String(html`
    ${periodBar(p, tot.range)}
    <div class="row wrap between" style="margin-bottom:14px">
      <div class="seg">${KINDS.filter(([k]) => k !== "both" || hasBoth).map(([k, l]) => html`<button data-kind="${k}" class="${k === kind ? "on" : ""}">${l}</button>`)}</div>
      <span>${kind === "expense" ? "Spent" : kind === "income" ? "Received" : "Net"} <b>${amt(fromPaise(sum))}</b></span>
    </div>
    <div class="card">${parents.length ? html`<ul class="list">${parents.map((c) => [line(c, false), kids(c.id).map((k) => line(k, true))])}</ul>`
      : html`<div class="empty">No ${kind} categories yet</div>`}</div>
    <p class="muted" style="font-size:.85rem">The default period is set in <a href="#/settings">Settings</a> (currently ${PERIODS.find(([k]) => k === st.settings.default_period)?.[1].toLowerCase()}); categories.md is kept in sync for that period.</p>`);
  wirePeriod(root, p, tot, () => render(root, ctx));
  root.querySelectorAll("[data-kind]").forEach((b) => b.addEventListener("click", () => { S.view.catKind = b.dataset.kind; render(root, ctx); }));
  ctx.actionsEl.querySelector("[data-new]")?.addEventListener("click", () => categoryForm(null, ctx, { kind }));
}

async function detail(root, ctx, id) {
  const c = cat(id);
  if (!c) { root.innerHTML = String(html`<div class="empty">Category not found. <a href="#/categories">Back</a></div>`); return; }
  ctx.title(c.name);
  ctx.actions(ro() ? "" : html`<button data-edit>Edit</button>`);
  const p = periodState("cats");
  root.innerHTML = String(loading());
  const pr = await get("/api/period" + qs(periodQuery(p)));
  const [trend, tx, tot] = await Promise.all([
    get("/api/reports/trend" + qs({ category: id, bucket: "monthly" })),
    get("/api/transactions" + qs({ category: id, from: pr.range.from, to: pr.range.to, limit: 500 })),
    get("/api/categories/totals" + qs(periodQuery(p))),
  ]);
  if (!ctx.alive()) return;
  const parent = c.parent ? cat(c.parent) : null;
  const b = tot.budgets[id];
  root.innerHTML = String(html`
    ${periodBar(p, tot.range)}
    <div class="grid three">
      <div class="card stat"><div class="label">${c.kind === "expense" ? "Spent" : c.kind === "income" ? "Received" : "Net"} · ${tot.range.label}</div><div class="value">${amt(tot.totals[id] || "0")}</div>
        <div class="sub">${parent ? html`in <a href="#/category/${encodeURIComponent(parent.id)}">${parent.name}</a>` : kids(id).length ? `${kids(id).length} sub-categories included` : ""}</div></div>
      <div class="card stat"><div class="label">Budget</div><div class="value">${b ? amt(b) : "—"}</div>
        <div class="sub">${b ? `${money(fromPaise(paise(b) - paise(tot.totals[id] || "0")))} left` : c.budget ? "" : "no budget set"}</div></div>
      <div class="card stat"><div class="label">Monthly average · 12 months</div><div class="value">${amt(trend.average)}</div></div>
    </div>
    <div class="card" style="margin-top:14px"><header><h2>Last 12 months</h2></header>${columns(trend.rows.map((r) => ({ label: r.label, value: r.total })), { name: c.name })}</div>
    <div class="card"><header><h2>Transactions · ${tot.range.label}</h2><a href="#/tx?category=${encodeURIComponent(id)}">All</a></header><div data-list>${grouped(tx.items)}</div></div>`);
  wirePeriod(root, p, tot, () => detail(root, ctx, id));
  wireTips(root);
  wireRows(root.querySelector("[data-list]"), tx.items, (t) => openTx(t, () => ctx.refresh()));
  ctx.actionsEl.querySelector("[data-edit]")?.addEventListener("click", () => categoryForm(c, ctx));
}

export function categoryForm(c, ctx, defaults = {}) {
  const st = S.state;
  const isNew = !c;
  c = c || { kind: defaults.kind || "expense", parent: defaults.parent || "" };
  let kind = c.kind;
  const parentsFor = (k) => st.categories.filter((x) => !x.parent && x.kind === k && x.id !== c.id);
  const body = () => html`
    <label class="f"><span>Name</span><input name="name" value="${c.name || ""}" autofocus></label>
    <div class="fields two">
      <label class="f"><span>Kind</span><select name="kind" ${c.parent ? raw("disabled") : ""}>${KINDS.map(([k, l]) => html`<option value="${k}" ${k === kind ? raw("selected") : ""}>${l}</option>`)}</select></label>
      <label class="f"><span>Parent</span><select name="parent" ${!isNew && kids(c.id).length ? raw("disabled") : ""}><option value="">— top level —</option>
        ${parentsFor(kind).map((x) => html`<option value="${x.id}" ${x.id === c.parent ? raw("selected") : ""}>${x.icon || ""} ${x.name}</option>`)}</select></label>
    </div>
    ${kind === "both" ? html`<p class="muted" style="margin-top:-6px;font-size:.85rem">“Both ways” is for money that can go in or out, e.g. <b>Market</b> gains and losses: record a gain as Income and a loss as Expense with this category.</p>` : ""}
    <div class="fields two">
      <label class="f"><span>Icon (emoji)</span><input name="icon" value="${c.icon || ""}" maxlength="4"></label>
      <label class="f"><span>Colour</span><input type="color" name="color" value="${c.color || "#3a807a"}" style="height:40px;padding:4px"></label>
    </div>
    <div class="fields two">
      <label class="f"><span>Budget (optional)</span><input name="budget" inputmode="decimal" value="${paise(c.budget) ? c.budget : ""}" placeholder="none"></label>
      <label class="f"><span>Budget period</span><select name="budget_period">${PERIODS.filter(([k]) => k !== "custom").map(([k, l]) => html`<option value="${k}" ${(c.budget_period || "monthly") === k ? raw("selected") : ""}>${l}</option>`)}</select></label>
    </div>
    <label class="check"><input type="checkbox" name="hidden" ${c.hidden ? raw("checked") : ""}> Hide from pickers</label>`;
  sheet(isNew ? "Add category" : "Edit category", body(),
    html`${isNew ? "" : html`<button class="danger" data-del>Delete</button>`}<span class="grow"></span><button data-close>Cancel</button><button class="primary" data-save>Save</button>`,
    (d, close) => {
      const onKind = (e) => {
        Object.assign(c, formData(d));
        kind = e.target.value;
        c.parent = "";
        d.querySelector(".sheet-b").innerHTML = String(body());
        d.querySelector('[name="kind"]').addEventListener("change", onKind);
      };
      d.querySelector('[name="kind"]').addEventListener("change", onKind);
      d.querySelector("[data-save]").addEventListener("click", async () => {
        const v = formData(d);
        const budget = v.budget.trim() ? calc(v.budget) : 0;
        if (Number.isNaN(budget)) return toast("Budget is not a number", true);
        const parent = d.querySelector('[name="parent"]').disabled ? c.parent || "" : v.parent;
        const body = { name: v.name, kind: parent ? cat(parent).kind : kind, parent, icon: v.icon, color: v.color,
          budget: fromPaise(budget), budget_period: budget ? v.budget_period : "", hidden: v.hidden, order: c.order || 0 };
        try {
          if (isNew) await post("/api/categories", body); else await put("/api/categories/" + encodeURIComponent(c.id), body);
          close();
          toast("Saved");
          ctx.refresh();
        } catch (e) { toast(e.message, true); }
      });
      d.querySelector("[data-del]")?.addEventListener("click", () => { close(); deleteCategory(c, ctx); });
    });
}

function deleteCategory(c, ctx) {
  const others = S.state.categories.filter((x) => x.id !== c.id && x.parent !== c.id);
  sheet("Delete " + c.name, html`
    <p>If transactions or reminders use this category, they are moved to the category you choose.</p>
    <label class="f"><span>Move them to</span><select name="to"><option value="">(nothing uses it)</option>
      ${others.map((x) => html`<option value="${x.id}">${x.parent ? "  › " : ""}${x.icon || ""} ${x.name}</option>`)}</select></label>`,
    html`<span class="grow"></span><button data-close>Cancel</button><button class="danger" data-ok>Delete</button>`, (d, close) => {
      d.querySelector("[data-ok]").addEventListener("click", async () => {
        const to = d.querySelector('[name="to"]').value;
        try {
          await del("/api/categories/" + encodeURIComponent(c.id) + qs({ move_to: to }));
          close();
          toast("Deleted");
          await ctx.refresh();
          ctx.go("#/categories");
        } catch (e) { toast(e.message, true); }
      });
    });
}

export { confirmSheet };
