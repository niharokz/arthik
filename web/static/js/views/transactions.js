// Transactions: search, period, filters (type, accounts, categories, labels, status,
// amount, posted/scheduled), totals of the filtered list, bulk actions and CSV export.

import { html, raw, qs, money, amt, paise, debounce } from "../util.js";
import { S, PERIODS, sheet, toast, confirmSheet, accName, catName, labelName, ro, today, loading, formData } from "../ui.js";
import { get, post } from "../api.js";
import { grouped, wireRows, pendingCreates } from "../txlist.js";
import { openTx } from "../txform.js";

function filters(ctx) {
  if (!S.view.tx) S.view.tx = { period: "all", date: today(), from: "", to: "", q: "", type: [], account: [], category: [], label: [], status: [], when: "", min: "", max: "", upcoming: false };
  const f = S.view.tx;
  const p = ctx.params;
  if ([...p.keys()].length) { // a link like #/tx?label=goa-trip resets the filters to it
    Object.assign(f, { period: "all", q: "", type: [], account: [], category: [], label: [], status: [], when: "", min: "", max: "" });
    for (const k of ["account", "category", "label", "status", "type"]) if (p.get(k)) f[k] = p.get(k).split(",");
    if (p.get("q")) f.q = p.get("q");
    if (p.get("from") && p.get("to")) Object.assign(f, { period: "custom", from: p.get("from"), to: p.get("to") });
    history.replaceState(null, "", "#/tx");
  }
  return f;
}

const activeCount = (f) => ["type", "account", "category", "label", "status"].reduce((n, k) => n + (f[k].length ? 1 : 0), 0) + (f.when ? 1 : 0) + (f.min || f.max ? 1 : 0);

let selecting = false;
const selected = new Set();

export async function render(root, ctx) {
  const f = filters(ctx);
  let range = null, rangeReply = null;
  if (f.period !== "all") {
    rangeReply = await get("/api/period" + qs(f.period === "custom" ? { period: "custom", from: f.from, to: f.to } : { period: f.period, date: f.date }));
    range = rangeReply.range;
  }
  const query = { from: range?.from, to: range?.to, q: f.q, type: f.type, account: f.account, category: f.category, label: f.label, status: f.status, when: f.when, min: f.min, max: f.max };
  if (!root.querySelector("[data-txlist]")) root.innerHTML = String(loading());
  const res = await get("/api/transactions" + qs({ ...query, limit: 1000, upcoming: f.upcoming ? "1" : "" }));
  if (!ctx.alive()) return;
  const pending = f.period === "all" && !activeCount(f) && !f.q ? pendingCreates() : [];
  const items = [...pending, ...res.items];
  const s = res.sums;
  ctx.actions(ro() ? "" : html`<button class="ghost" data-select>${selecting ? "Done" : "Select"}</button>`);

  root.innerHTML = String(html`
    <div class="row wrap" style="margin-bottom:10px">
      <input type="search" class="grow" data-q placeholder="Search title, notes, amount, category…" value="${f.q}" style="min-width:200px">
      <button data-filters>Filters${activeCount(f) ? html` <span class="badge">${activeCount(f)}</span>` : ""}</button>
      <a class="btn" href="/api/export.csv${qs(query)}" download>Export CSV</a>
    </div>
    <div class="period">
      <select data-pk aria-label="Period"><option value="all" ${f.period === "all" ? raw("selected") : ""}>All dates</option>
        ${PERIODS.map(([v, l]) => html`<option value="${v}" ${f.period === v ? raw("selected") : ""}>${l}</option>`)}</select>
      ${f.period === "custom" ? html`<input type="date" data-from value="${f.from || range?.from}"> – <input type="date" data-to value="${f.to || range?.to}">`
        : f.period !== "all" ? html`<button class="icon" data-step="-1" aria-label="Previous">‹</button><span class="label">${range.label}</span><button class="icon" data-step="1" aria-label="Next">›</button>` : ""}
    </div>
    ${chips(f)}
    <div class="grid four" style="margin-bottom:14px">
      <div class="card stat"><div class="label">Transactions</div><div class="value">${s.count}</div>${s.upcoming ? html`<div class="sub">+ ${s.upcoming} upcoming</div>` : ""}</div>
      <div class="card stat"><div class="label">Income</div><div class="value pos">${amt(s.income)}</div></div>
      <div class="card stat"><div class="label">Expense</div><div class="value neg">${amt(s.expense)}</div></div>
      ${f.account.length ? html`<div class="card stat"><div class="label">In / out of ${f.account.length === 1 ? accName(f.account[0]) : "accounts"}</div><div class="value" style="font-size:1rem">${amt(s.inflow)} / ${amt(s.outflow)}</div></div>`
        : html`<div class="card stat"><div class="label">Net</div><div class="value">${amt(s.net, { signed: true })}</div></div>`}
    </div>
    ${selecting ? html`<div class="card row wrap" data-bulk><b>${selected.size} selected</b><span class="grow"></span>
      <button data-b="status:cleared">Mark cleared</button><button data-b="status:reconciled">Mark reconciled</button><button data-b="status:uncleared">Uncleared</button>
      <button data-b="add_label">Add label</button><button class="danger" data-b="delete">Delete</button></div>` : ""}
    <div class="card" data-txlist>${grouped(items, { account: f.account.length === 1 ? f.account[0] : undefined, empty: f.q || activeCount(f) ? "Nothing matches these filters" : "No transactions yet — tap ＋ to add one" })}
      ${res.total > res.items.filter((x) => !x.upcoming).length ? html`<p class="muted" style="text-align:center">Showing the newest 1000 of ${res.total}. Narrow the period to see older ones.</p>` : ""}</div>`);

  const rerender = () => render(root, ctx);
  const q = root.querySelector("[data-q]");
  q.addEventListener("input", debounce(() => { f.q = q.value; rerender(); }, 300));
  root.querySelector("[data-pk]").addEventListener("change", (e) => { f.period = e.target.value; f.date = today(); rerender(); });
  root.querySelectorAll("[data-step]").forEach((b) => b.addEventListener("click", () => {
    f.date = (b.dataset.step === "-1" ? rangeReply.prev : rangeReply.next).from; rerender();
  }));
  const fr = root.querySelector("[data-from]"), to = root.querySelector("[data-to]");
  [fr, to].forEach((el) => el?.addEventListener("change", () => { if (fr.value && to.value) { f.from = fr.value; f.to = to.value; rerender(); } }));
  root.querySelector("[data-filters]").addEventListener("click", () => filterSheet(f, rerender));
  root.querySelectorAll("[data-unchip]").forEach((b) => b.addEventListener("click", () => {
    const [k, v] = b.dataset.unchip.split("|");
    if (Array.isArray(f[k])) f[k] = f[k].filter((x) => x !== v); else f[k] = "";
    rerender();
  }));
  ctx.actionsEl.querySelector("[data-select]")?.addEventListener("click", () => { selecting = !selecting; selected.clear(); rerender(); });

  const list = root.querySelector("[data-txlist]");
  if (selecting) {
    list.querySelectorAll("[data-tx]").forEach((b) => {
      const id = b.dataset.tx;
      const box = document.createElement("input");
      box.type = "checkbox";
      box.checked = selected.has(id);
      box.setAttribute("aria-label", "Select");
      b.prepend(box);
      b.addEventListener("click", (e) => {
        e.stopImmediatePropagation();
        if (e.target !== box) box.checked = !box.checked;
        box.checked ? selected.add(id) : selected.delete(id);
        root.querySelector("[data-bulk] b").textContent = `${selected.size} selected`;
      }, true);
    });
    root.querySelectorAll("[data-b]").forEach((b) => b.addEventListener("click", () => bulk(b.dataset.b, ctx)));
  } else {
    wireRows(list, items, (t) => (t._pending ? toast("Waiting to sync — you can edit it once it is synced") : t.upcoming ? ctx.go("#/scheduled") : openTx(t, () => ctx.refresh())));
  }
}

function chips(f) {
  const out = [];
  const add = (k, v, text) => out.push(html`<button class="chip on" data-unchip="${k}|${v}" title="Remove filter">${text} ✕</button>`);
  f.type.forEach((v) => add("type", v, v));
  f.account.forEach((v) => add("account", v, accName(v)));
  f.category.forEach((v) => add("category", v, catName(v, true)));
  f.label.forEach((v) => add("label", v, "#" + labelName(v)));
  f.status.forEach((v) => add("status", v, v));
  if (f.when) add("when", "", f.when === "posted" ? "posted only" : "scheduled only");
  if (f.min || f.max) { out.push(html`<span class="chip on">${f.min ? money(f.min) : "₹0"} – ${f.max ? money(f.max) : "∞"}</span>`); }
  return out.length ? html`<div class="chips" style="margin-bottom:12px">${out}</div>` : "";
}

function filterSheet(f, rerender) {
  const st = S.state;
  const box = (name, value, text, on) => html`<label class="chip ${on ? "on" : ""}" style="cursor:pointer"><input type="checkbox" data-f="${name}" value="${value}" ${on ? raw("checked") : ""} hidden>${text}</label>`;
  const body = html`
    <h3>Type</h3><div class="chips" style="margin:6px 0 14px">${["expense", "income", "transfer"].map((t) => box("type", t, t, f.type.includes(t)))}</div>
    <h3>Accounts</h3><div class="chips" style="margin:6px 0 14px">${st.accounts.map((a) => box("account", a.id, a.name, f.account.includes(a.id)))}</div>
    <h3>Categories</h3><div class="chips" style="margin:6px 0 14px">${st.categories.filter((c) => !c.parent).map((c) => [box("category", c.id, (c.icon || "") + " " + c.name, f.category.includes(c.id)),
      st.categories.filter((k) => k.parent === c.id).map((k) => box("category", k.id, "› " + k.name, f.category.includes(k.id)))])}</div>
    ${st.labels.length ? html`<h3>Labels</h3><div class="chips" style="margin:6px 0 14px">${st.labels.map((l) => box("label", l.id, "#" + l.name, f.label.includes(l.id)))}</div>` : ""}
    <h3>Status</h3><div class="chips" style="margin:6px 0 14px">${["uncleared", "cleared", "reconciled"].map((s) => box("status", s, s, f.status.includes(s)))}</div>
    <div class="fields two">
      <label class="f"><span>Min amount</span><input name="min" inputmode="decimal" value="${f.min}"></label>
      <label class="f"><span>Max amount</span><input name="max" inputmode="decimal" value="${f.max}"></label>
    </div>
    <label class="f"><span>Dates</span><select name="when">
      <option value="">Posted and future-dated</option><option value="posted" ${f.when === "posted" ? raw("selected") : ""}>Posted only (today and earlier)</option>
      <option value="scheduled" ${f.when === "scheduled" ? raw("selected") : ""}>Future-dated only</option></select></label>
    <label class="check"><input type="checkbox" name="upcoming" ${f.upcoming ? raw("checked") : ""}> Also show upcoming reminder occurrences</label>`;
  sheet("Filters", body, html`<button data-clear>Clear all</button><span class="grow"></span><button class="primary" data-apply>Apply</button>`, (d, close) => {
    d.querySelectorAll("label.chip").forEach((l) => l.addEventListener("change", () => l.classList.toggle("on", l.querySelector("input").checked)));
    d.querySelector("[data-apply]").addEventListener("click", () => {
      for (const k of ["type", "account", "category", "label", "status"]) f[k] = [...d.querySelectorAll(`[data-f="${k}"]:checked`)].map((x) => x.value);
      const v = formData(d);
      Object.assign(f, { min: v.min.trim(), max: v.max.trim(), when: v.when, upcoming: v.upcoming });
      close();
      rerender();
    });
    d.querySelector("[data-clear]").addEventListener("click", () => {
      Object.assign(f, { type: [], account: [], category: [], label: [], status: [], when: "", min: "", max: "", upcoming: false });
      close();
      rerender();
    });
  });
}

async function bulk(action, ctx) {
  if (!selected.size) return toast("Select some transactions first");
  const ids = [...selected];
  let body;
  if (action === "delete") {
    if (!(await confirmSheet(`Delete ${ids.length} transaction(s)?`, "They will be removed from every balance and total."))) return;
    body = { ids, action: "delete" };
  } else if (action === "add_label") {
    const name = await new Promise((resolve) => {
      sheet("Add label", html`<label class="f"><span>Label</span><input name="l" list="lbls" autofocus><datalist id="lbls">${S.state.labels.map((l) => html`<option value="${l.name}">`)}</datalist></label>`,
        html`<span class="grow"></span><button data-close>Cancel</button><button class="primary" data-ok>Add</button>`,
        (d, close) => d.querySelector("[data-ok]").addEventListener("click", () => { const v = d.querySelector("[name=l]").value.trim(); close(); resolve(v); }));
    });
    if (!name) return;
    body = { ids, action: "add_label", value: name };
  } else {
    body = { ids, action: "status", value: action.split(":")[1] };
  }
  try {
    const r = await post("/api/transactions/bulk", body);
    toast(`${r.updated} updated`);
    selected.clear();
    selecting = false;
    ctx.refresh();
  } catch (e) { toast(e.message, true); }
}

export { paise };
