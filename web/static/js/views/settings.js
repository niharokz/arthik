// Settings (and the "More" menu on phones): default category period, week/month/year
// start, look-ahead, theme, hide amounts, inbox fallback categories and learned
// rules, CSV import/export, data health, sign out.

import { html, raw } from "../util.js";
import { S, toast, formData, ro, sheet, PERIODS, catOptions, catName } from "../ui.js";
import { get, put, del, postText } from "../api.js";
import { logout, navOrder, NAV_ITEMS, BOTTOM_COUNT } from "../main.js";
import { menuCard, wireMenuCard } from "./menu.js";
import { isApp } from "../native.js";
import { phoneCard, wirePhoneCard } from "./phone.js";

const WEEKDAYS = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"];
const MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

export async function render(root, ctx) {
  const st = S.state;
  const s = st.settings;
  ctx.title(ctx.name === "more" ? "More" : "Settings");
  const dis = ro() ? raw("disabled") : "";
  root.innerHTML = String(html`
    ${ctx.name === "more" ? html`<div class="card"><ul class="list">${navOrder().slice(BOTTOM_COUNT).map((k) => [k, NAV_ITEMS[k][0], NAV_ITEMS[k][2]]).map(([k, l, i]) => html`<li><a class="item" href="#/${k}"><span class="ic">${i}</span><span class="main-t"><span class="t">${l}</span></span><span class="end faint">›</span></a></li>`)}</ul></div>` : ""}
    ${isApp ? phoneCard() : ""}
    ${menuCard()}
    <div class="grid two">
      <form class="card" data-settings><header><h2>Preferences</h2></header>
        <label class="f"><span>Default period for categories, budgets and reports</span><select name="default_period" ${dis}>
          ${PERIODS.filter(([k]) => k !== "custom").map(([k, l]) => html`<option value="${k}" ${k === s.default_period ? raw("selected") : ""}>${l === "Half-year" ? "Half-yearly" : l + "ly"}</option>`)}</select></label>
        <div class="fields two">
          <label class="f"><span>Week starts on</span><select name="week_start" ${dis}>${WEEKDAYS.map((w) => html`<option value="${w}" ${w === s.week_start ? raw("selected") : ""}>${w[0].toUpperCase() + w.slice(1)}</option>`)}</select></label>
          <label class="f"><span>Month starts on day</span><input name="month_start_day" type="number" min="1" max="28" value="${s.month_start_day}" ${dis}></label>
        </div>
        <div class="fields two">
          <label class="f"><span>Year starts in</span><select name="year_start_month" ${dis}>${MONTHS.map((m, i) => html`<option value="${i + 1}" ${i + 1 === s.year_start_month ? raw("selected") : ""}>${m}${i === 3 ? " (Indian FY)" : ""}</option>`)}</select></label>
          <label class="f"><span>Look-ahead for upcoming (days)</span><input name="upcoming_days" type="number" min="1" max="365" value="${s.upcoming_days}" ${dis}></label>
        </div>
        <label class="f"><span>Theme</span><select name="theme" ${dis}>${[["auto", "Follow the device"], ["light", "Light"], ["dark", "Dark"]].map(([k, l]) => html`<option value="${k}" ${k === s.theme ? raw("selected") : ""}>${l}</option>`)}</select></label>
        <label class="check"><input type="checkbox" name="hide_amounts" ${s.hide_amounts ? raw("checked") : ""} ${dis}> Hide amounts (blurred — tap an amount to peek)</label>
        <div class="fields two">
          <label class="f"><span>Inbox: expense category when unsure</span><select name="inbox_expense_category" ${dis}><option value="">Leave empty (I'll choose)</option>${catOptions("expense", s.inbox_expense_category)}</select></label>
          <label class="f"><span>Inbox: income category when unsure</span><select name="inbox_income_category" ${dis}><option value="">Leave empty (I'll choose)</option>${catOptions("income", s.inbox_income_category)}</select></label>
        </div>
        ${ro() ? html`<p class="muted">The demo is read-only.</p>` : html`<button class="primary" type="submit">Save preferences</button>`}
      </form>
      <div class="stack">
        <div class="card" data-rules><header><h2>🧠 Learned rules</h2></header><p class="muted" style="margin:0">Loading…</p></div>
        <div class="card"><header><h2>Export & import</h2></header>
          <p class="muted" style="margin-top:0">Export every transaction as CSV (the transaction list exports what you filtered). The same format imports back.</p>
          <div class="row wrap"><a class="btn" href="/api/export.csv" download>Export all (CSV)</a>${ro() ? "" : html`<button data-import>Import CSV…</button>`}</div></div>
        <div class="card"><header><h2>Data health</h2></header>
          ${st.load_error ? html`<p class="neg">${st.load_error}</p>` : ""}
          ${(st.problems || []).length ? html`<ul class="list">${st.problems.map((p) => html`<li style="padding:8px 0">${p.level === "info" ? "ℹ️" : "⚠️"} <b>${p.file}</b>${p.id ? html` <code>${p.id}</code>` : ""}<br><span class="muted">${p.msg}</span></li>`)}</ul>` : ""}
          ${st.load_error || (st.problems || []).some((p) => p.level !== "info") ? "" : html`<p class="pos" style="margin:0">✓ Every file loads and the books balance.</p>`}
          <p class="muted" style="font-size:.85rem;margin-bottom:0">Data lives in YAML files (accounts.md, categories.md, transactions/transaction_YYYYMM.md …). Edits made in other apps are picked up within seconds and every computed value is rebuilt.</p></div>
        ${isApp ? "" : html`<div class="card"><header><h2>Account</h2></header>
          <p style="margin-top:0">Signed in as <b>${st.user}</b>${ro() ? " (read-only demo)" : ""}. ${ro() ? "" : html`<span class="muted">The password is set in the server's .env file.</span>`}</p>
          <button data-logout>Sign out</button>
          <p class="faint" style="font-size:.8rem;margin-bottom:0">Arthik ${st.version} · <a href="/">About</a></p></div>`}
      </div>
    </div>`);
  root.querySelector("[data-settings]").addEventListener("submit", async (e) => {
    e.preventDefault();
    const v = formData(e.target);
    try {
      await put("/api/settings", { ...v, month_start_day: +v.month_start_day, year_start_month: +v.year_start_month, upcoming_days: +v.upcoming_days });
      for (const k of Object.keys(S.view)) if (S.view[k] && S.view[k].period) S.view[k].period = v.default_period;
      toast("Preferences saved");
      ctx.refresh();
    } catch (err) { toast(err.message, true); }
  });
  root.querySelector("[data-logout]")?.addEventListener("click", logout);
  if (isApp) wirePhoneCard(root, ctx);
  wireMenuCard(root);
  root.querySelector("[data-import]")?.addEventListener("click", () => importSheet(ctx));
  rulesCard(root.querySelector("[data-rules]"));
}

// What merchants become in the inbox, learned when you accept drafts.
async function rulesCard(el) {
  let rules = [];
  try { rules = await get("/api/inbox/rules"); } catch (e) { el.innerHTML = String(html`<header><h2>🧠 Learned rules</h2></header><p class="neg">${e.message}</p>`); return; }
  const draw = () => {
    el.innerHTML = String(html`<header><h2>🧠 Learned rules</h2><small class="faint">${rules.length}</small></header>
      <p class="muted" style="margin-top:0;font-size:.85rem">When an inbox draft's merchant matches, its title and category fill in by themselves. Accept a draft with a different category to change a rule.</p>
      ${rules.length ? html`<ul class="list">${rules.map((r) => html`<li class="row" style="padding:6px 0;gap:8px">
        <span class="grow"><code>${r.match}</code> → <b>${r.title || "—"}</b> · ${catName(r.category, true)}${r.uses > 1 ? html` <small class="faint">${r.uses}×</small>` : ""}</span>
        ${ro() ? "" : html`<button class="icon ghost" data-forget="${r.id}" aria-label="Forget rule">✕</button>`}</li>`)}</ul>`
      : html`<p class="faint" style="margin:0">Nothing learned yet.</p>`}`);
    el.querySelectorAll("[data-forget]").forEach((b) => b.addEventListener("click", async () => {
      try {
        await del("/api/inbox/rules/" + encodeURIComponent(b.dataset.forget));
        rules = rules.filter((r) => r.id !== b.dataset.forget);
        draw();
        toast("Rule forgotten");
      } catch (e) { toast(e.message, true); }
    }));
  };
  draw();
}

function importSheet(ctx) {
  let text = "";
  sheet("Import CSV", html`
    <p class="muted" style="margin-top:0">Columns: <code>date, title, amount, type, account, to_account, category, labels, status, notes, splits</code> (only date and amount are required; without a type, negative amounts are expenses). Use “Parent &gt; Child” for sub-categories. Rows whose id already exists are skipped.</p>
    <input type="file" accept=".csv,text/csv" data-file>
    <label class="check"><input type="checkbox" data-create checked> Create missing accounts and categories</label>
    <div data-preview></div>`,
    html`<span class="grow"></span><button data-close>Cancel</button><button class="primary" data-go disabled>Import</button>`, (d, close) => {
      const preview = async () => {
        const create = d.querySelector("[data-create]").checked ? "1" : "";
        try {
          const r = await postText(`/api/import?dry_run=1&create_missing=${create}`, text);
          d.querySelector("[data-preview]").innerHTML = String(report(r, true));
          d.querySelector("[data-go]").disabled = !r.imported;
        } catch (e) { d.querySelector("[data-preview]").innerHTML = String(html`<p class="neg">${e.message}</p>`); }
      };
      d.querySelector("[data-file]").addEventListener("change", async (e) => { text = await e.target.files[0].text(); preview(); });
      d.querySelector("[data-create]").addEventListener("change", () => text && preview());
      d.querySelector("[data-go]").addEventListener("click", async () => {
        const create = d.querySelector("[data-create]").checked ? "1" : "";
        try {
          const r = await postText(`/api/import?create_missing=${create}`, text);
          close();
          toast(`Imported ${r.imported} transaction(s)`);
          ctx.refresh();
        } catch (e) { toast(e.message, true); }
      });
    });
}

function report(r, dry) {
  return html`<div class="card" style="margin-top:12px">
    <p style="margin-top:0"><b>${r.rows}</b> rows · <b class="pos">${r.imported}</b> ${dry ? "would be imported" : "imported"} · ${r.skipped} already present · <b class="${r.errors.length ? "neg" : ""}">${r.errors.length}</b> with errors</p>
    ${r.new_accounts?.length ? html`<p>New accounts: ${r.new_accounts.join(", ")}</p>` : ""}
    ${r.new_categories?.length ? html`<p>New categories: ${r.new_categories.join(", ")}</p>` : ""}
    ${r.new_labels?.length ? html`<p>New labels: ${r.new_labels.join(", ")}</p>` : ""}
    ${r.errors.length ? html`<ul style="max-height:200px;overflow:auto;padding-left:18px">${r.errors.slice(0, 50).map((e) => html`<li>line ${e.line}: ${e.msg}</li>`)}</ul>` : ""}</div>`;
}
