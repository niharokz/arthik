// Settings, one card per feature with an index to jump between them: Look, Daily
// alert, Periods, Home & menu, Accounts, Inbox (with learned rules), Phone (app
// only), Data (export, import, health) and this device. Each card saves only its
// own part of settings.md, so nothing appears twice.

import { html, raw, qs, debounce, reducedMotion } from "../util.js";
import { S, toast, ro, sheet, PERIODS, catName } from "../ui.js";
import { get, post, del, postText, deviceKey, setDeviceKey } from "../api.js";
import { logout, navOrder, NAV_ITEMS, BOTTOM_COUNT } from "../main.js";
import { menuCard, wireMenuCard } from "./menu.js";
import { isApp } from "../native.js";
import { phoneCard, wirePhoneCard } from "./phone.js";
import { inboxSettingsCard, wireInboxSettings } from "./inbox.js";
import { lookCard, wireLook, alertsCard, wireAlerts, saveSettings } from "./look.js";
import { pluginScreens } from "../plugins.js";

const WEEKDAYS = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"];
const MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

// Section id → index label (Phone only inside the app, Device only on the web).
const SECTIONS = [["look", "Look"], ["alerts", "Alert"], ["periods", "Periods"], ["home", "Home & menu"], ["accounts", "Accounts"],
  ["inbox", "Inbox"], ["phone", "Phone", () => isApp], ["data", "Data"], ["device", "Device", () => !isApp]];

export async function render(root, ctx) {
  const st = S.state;
  const s = st.settings;
  ctx.title("Settings");
  const more = navOrder().slice(BOTTOM_COUNT).map((k) => [k, NAV_ITEMS[k][0], NAV_ITEMS[k][2]]);
  const plugins = pluginScreens();
  const dis = ro() ? raw("disabled") : "";
  const sel = (v, cur) => (v === cur ? raw("selected") : "");
  const saveBtn = (t) => (ro() ? "" : html`<div class="row set-act"><span class="grow"></span><button class="primary" type="submit">${t}</button></div>`);
  root.innerHTML = String(html`
    ${more.length ? html`<nav class="card screens only-phone" aria-label="More screens"><ul class="list">${more.map(([k, l, i]) => html`<li><a class="item" href="#/${k}"><span class="ic">${i}</span><span class="main-t"><span class="t">${l}</span></span><span class="end faint">›</span></a></li>`)}</ul></nav>` : ""}
    ${plugins.length ? html`<nav class="card screens" aria-label="Plugins"><header><h2>Plugins</h2><span class="muted">personal add-ons</span></header><ul class="list">${plugins.map((p) => html`<li><a class="item" href="#/p/${p.id}"><span class="ic">${p.icon}</span><span class="main-t"><span class="t">${p.title}</span></span><span class="end faint">›</span></a></li>`)}</ul></nav>` : ""}
    <nav class="set-index" aria-label="Settings sections">${SECTIONS.filter(([, , on]) => !on || on()).map(([id, l]) => html`<button class="chip-btn" data-jump="${id}">${l}</button>`)}</nav>
    <div class="set-grid">
      ${lookCard()}
      ${alertsCard()}
      <form class="card set-sec" id="set-periods" data-periods><header><h2>Periods</h2></header>
        <label class="f"><span>Default period for categories, budgets and reports</span><select name="default_period" ${dis}>
          ${PERIODS.filter(([k]) => k !== "custom").map(([k, l]) => html`<option value="${k}" ${sel(k, s.default_period)}>${l === "Half-year" ? "Half-yearly" : l + "ly"}</option>`)}</select></label>
        <div class="fields two">
          <label class="f"><span>Week starts on</span><select name="week_start" ${dis}>${WEEKDAYS.map((w) => html`<option value="${w}" ${sel(w, s.week_start)}>${w[0].toUpperCase() + w.slice(1)}</option>`)}</select></label>
          <label class="f"><span>Month starts on day</span><input name="month_start_day" type="number" min="1" max="28" value="${s.month_start_day}" ${dis}></label>
        </div>
        <label class="f"><span>Year starts in</span><select name="year_start_month" ${dis}>${MONTHS.map((m, i) => html`<option value="${i + 1}" ${sel(i + 1, s.year_start_month)}>${m}${i === 3 ? " (Indian FY)" : ""}</option>`)}</select></label>
        ${saveBtn("Save periods")}
      </form>
      <div class="set-sec stack" id="set-home">
        <form class="card" data-homeprefs><header><h2>Home</h2><a href="#/home">Customize blocks on Home ✎</a></header>
          <label class="f"><span>Look-ahead for upcoming (days)</span><input name="upcoming_days" type="number" min="1" max="365" value="${s.upcoming_days}" ${dis}></label>
          <label class="check"><input type="checkbox" name="hide_amounts" ${s.hide_amounts ? raw("checked") : ""} ${dis}> Hide amounts (blurred — tap an amount to peek; the eye in the top bar switches it too)</label>
          <label class="check"><input type="checkbox" name="home_locked" ${s.home_locked ? raw("checked") : ""} ${dis}> Lock the home layout (hides ✎ Customize home)</label>
          ${saveBtn("Save home")}
        </form>
        ${menuCard()}
      </div>
      <form class="card set-sec" id="set-accounts" data-accprefs><header><h2>Accounts</h2><a href="#/accounts">Open accounts</a></header>
        <label class="f"><span>Re-check bank balances after (days)</span><input name="balance_check_days" type="number" min="1" max="365" value="${s.balance_check_days}" ${dis}></label>
        <p class="muted set-note">An account checked longer ago shows ○; one that doesn't match shows ❌ (and can raise the daily alert).</p>
        ${saveBtn("Save accounts")}
      </form>
      <div class="set-sec stack" id="set-inbox-wrap">
        ${inboxSettingsCard()}
        <div class="card" data-rules><header><h2>🧠 Learned rules</h2></header><p class="muted" style="margin:0">Loading…</p></div>
      </div>
      ${isApp ? html`<div class="set-sec wide" id="set-phone">${phoneCard()}</div>` : ""}
      <div class="set-sec stack" id="set-data">
        <div class="card"><header><h2>Export & import</h2></header>
          <p class="muted" style="margin-top:0">Export every transaction as CSV (the transaction list exports what you filtered). The same format imports back.</p>
          <div class="row wrap"><a class="btn" href="/api/export.csv" download>Export all (CSV)</a>${ro() ? "" : html`<button data-import>Import CSV…</button>`}</div></div>
        <div class="card"><header><h2>Data health</h2></header>
          ${st.load_error ? html`<p class="neg">${st.load_error}</p>` : ""}
          ${(st.problems || []).length ? html`<ul class="list">${st.problems.map((p) => html`<li style="padding:8px 0">${p.level === "info" ? "ℹ️" : "⚠️"} <b>${p.file}</b>${p.id ? html` <code>${p.id}</code>` : ""}<br><span class="muted">${p.msg}</span></li>`)}</ul>` : ""}
          ${st.load_error || (st.problems || []).some((p) => p.level !== "info") ? "" : html`<p class="pos" style="margin:0">✓ Every file loads and the books balance.</p>`}
          <p class="muted" style="font-size:.85rem;margin-bottom:0">Data lives in YAML files (accounts.md, categories.md, transactions/transaction_YYYYMM.md …). Edits made in other apps are picked up within seconds and every computed value is rebuilt.</p></div>
      </div>
      ${isApp ? "" : html`<div class="set-sec stack" id="set-device">
        <div class="card"><header><h2>Account</h2></header>
          <p style="margin-top:0">Signed in as <b>${st.user}</b>${ro() ? " (read-only demo)" : ""}. ${ro() ? "" : html`<span class="muted">The password is set in the server's .env file.</span>`}</p>
          <button data-logout>Sign out</button>
          <p class="faint" style="font-size:.8rem;margin-bottom:0">Arthik ${st.version} · <a href="/">About</a></p></div>
        ${ro() ? "" : html`<div class="card" data-devkey></div>`}</div>`}
    </div>`);

  // Each form saves its own fields; the rest of settings.md stays as it is.
  const form = (sel, pick, msg) => root.querySelector(sel)?.addEventListener("submit", async (e) => {
    e.preventDefault();
    try { await saveSettings(pick(e.target), msg); ctx.refresh(); } catch (err) { toast(err.message, true); }
  });
  form("[data-periods]", (f) => {
    for (const k of Object.keys(S.view)) if (S.view[k] && S.view[k].period) S.view[k].period = f.default_period.value;
    return { default_period: f.default_period.value, week_start: f.week_start.value, month_start_day: +f.month_start_day.value, year_start_month: +f.year_start_month.value };
  }, "Periods saved");
  form("[data-homeprefs]", (f) => {
    document.body.classList.toggle("hide-amounts", f.hide_amounts.checked);
    return { upcoming_days: +f.upcoming_days.value, hide_amounts: f.hide_amounts.checked, home_locked: f.home_locked.checked };
  }, "Home saved");
  form("[data-accprefs]", (f) => ({ balance_check_days: +f.balance_check_days.value }), "Accounts saved");
  wireInboxSettings(root, async (patch, msg) => { await saveSettings(patch, msg); ctx.refresh(); });
  wireLook(root, ctx);
  wireAlerts(root);
  root.querySelector("[data-logout]")?.addEventListener("click", logout);
  deviceKeyCard(root.querySelector("[data-devkey]"));
  if (isApp) wirePhoneCard(root, ctx);
  wireMenuCard(root);
  root.querySelector("[data-import]")?.addEventListener("click", () => importSheet(ctx));
  rulesCard(root.querySelector("[data-rules]"));
  const jump = (id) => {
    const el = root.querySelector(`#set-${id}`) || root.querySelector(`#set-${id}-wrap`);
    el?.scrollIntoView({ block: "start", behavior: reducedMotion() ? "auto" : "smooth" });
    el?.classList.add("flash-sec");
    setTimeout(() => el?.classList.remove("flash-sec"), 1200);
  };
  root.querySelector(".set-index").addEventListener("click", (e) => { const b = e.target.closest("[data-jump]"); if (b) jump(b.dataset.jump); });
  if (ctx.params.get("s")) requestAnimationFrame(() => jump(ctx.params.get("s")));
}

// What merchants become in the inbox, learned when you accept drafts. There can be
// thousands, so they come a page at a time, most used first, with a search box.
const RULES_PAGE = 25;
function rulesCard(el) {
  let q = "", offset = 0, page = null, busy = false;
  const head = (n) => html`<header><h2>🧠 Learned rules</h2>${n != null ? html`<small class="faint">${n.toLocaleString("en-IN")}</small>` : ""}</header>`;
  const load = async () => {
    if (busy) return;
    busy = true;
    try {
      page = await get("/api/inbox/rules" + qs({ q, offset, limit: RULES_PAGE }));
      draw();
    } catch (e) {
      el.innerHTML = String(html`${head()}<p class="neg">${e.message}</p>`);
    } finally { busy = false; }
  };
  const draw = () => {
    const { items, total } = page;
    const from = total ? offset + 1 : 0, to = offset + items.length;
    el.querySelector("[data-rules-body]").innerHTML = String(html`
      ${items.length ? html`<ul class="list rules">${items.map((r) => html`<li class="row rule">
        <span class="grow"><code>${r.match}</code> → <b>${r.title || "—"}</b> · ${catName(r.category, true)}${r.uses > 1 ? html` <small class="faint">${r.uses}×</small>` : ""}</span>
        ${ro() ? "" : html`<button class="icon ghost" data-forget="${r.id}" aria-label="Forget rule">✕</button>`}</li>`)}</ul>`
      : html`<p class="faint" style="margin:0">${q ? "No rule matches." : "Nothing learned yet."}</p>`}
      ${total > RULES_PAGE ? html`<div class="pager">
        <button class="ghost" data-page="-1" ${offset === 0 ? raw("disabled") : ""} aria-label="Previous page">‹ Prev</button>
        <span class="faint">${from.toLocaleString("en-IN")}–${to.toLocaleString("en-IN")} of ${total.toLocaleString("en-IN")}</span>
        <button class="ghost" data-page="1" ${to >= total ? raw("disabled") : ""} aria-label="Next page">Next ›</button></div>` : ""}`);
    el.querySelector("header small")?.remove();
    if (!q) el.querySelector("header").insertAdjacentHTML("beforeend", String(html`<small class="faint">${total.toLocaleString("en-IN")}</small>`));
  };
  el.innerHTML = String(html`${head()}
    <p class="muted" style="margin-top:0;font-size:.85rem">When an inbox draft's merchant matches, its title and category fill in by themselves. Accept a draft with a different category to change a rule.</p>
    <input type="search" data-rules-q placeholder="Search merchant, title or category" aria-label="Search rules" style="margin-bottom:8px">
    <div data-rules-body><p class="muted" style="margin:0">Loading…</p></div>`);
  el.querySelector("[data-rules-q]").addEventListener("input", debounce((e) => { q = e.target.value.trim(); offset = 0; load(); }, 250));
  el.addEventListener("click", async (e) => {
    const pg = e.target.closest("[data-page]");
    if (pg) {
      offset = Math.max(0, offset + Number(pg.dataset.page) * RULES_PAGE);
      await load();
      el.scrollIntoView({ block: "nearest", behavior: reducedMotion() ? "auto" : "smooth" });
      return;
    }
    const b = e.target.closest("[data-forget]");
    if (!b) return;
    try {
      await del("/api/inbox/rules/" + encodeURIComponent(b.dataset.forget));
      if (page.items.length === 1 && offset > 0) offset -= RULES_PAGE; // the page emptied: step back
      await load();
      toast("Rule forgotten");
    } catch (err) { toast(err.message, true); }
  });
  load();
}

function importSheet(ctx) {
  let text = "";
  sheet("Import CSV", html`
    <p class="muted" style="margin-top:0">Columns: <code>date, title, amount, type, account, to_account, category, notes, splits</code> (only date and amount are required; without a type, negative amounts are expenses). Use “Parent &gt; Child” for sub-categories. Rows whose id already exists are skipped.</p>
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
    ${r.errors.length ? html`<ul style="max-height:200px;overflow:auto;padding-left:18px">${r.errors.slice(0, 50).map((e) => html`<li>line ${e.line}: ${e.msg}</li>`)}</ul>` : ""}</div>`;
}

// ---- this device: the device key keeps it signed in until sign-out ----

async function deviceKeyCard(card) {
  if (!card) return;
  let me;
  try { me = await get("/api/me"); } catch { return; }
  const until = me.expires ? new Date(me.expires) : null;
  const status = me.remembered
    ? html`<p class="pos" style="margin-top:0">✓ This device stays signed in until you sign out.</p>`
    : html`<p class="muted" style="margin-top:0">Signed in until ${until ? until.toLocaleString([], { dateStyle: "medium", timeStyle: "short" }) : "the session ends"}. Enter the device key to stay signed in on this device.</p>`;
  card.innerHTML = String(html`<header><h2>🔐 This device</h2></header>${status}
    ${me.device_key ? html`<form class="row wrap" data-keyform style="gap:8px">
        <input name="key" type="password" autocomplete="off" placeholder="Device key" value="${deviceKey()}" style="flex:1 1 220px" aria-label="Device key">
        <button class="primary" type="submit">Save</button>
        ${deviceKey() || me.remembered ? html`<button type="button" data-forget>Forget on this device</button>` : ""}
      </form>
      <p class="faint" style="font-size:.8rem;margin-bottom:0">The key lives in the server's .env. Changing it there signs every remembered device out — enter the new one here again. ${me.remembered ? "" : "A wrong key counts as a failed sign-in."}</p>`
    : html`<p class="faint" style="font-size:.85rem;margin:0">No device key is set on the server. Add ARTHIK_DEVICE_KEY to .env to keep chosen devices signed in.</p>`}`);
  card.querySelector("[data-keyform]")?.addEventListener("submit", async (e) => {
    e.preventDefault();
    const key = e.target.key.value.trim();
    if (!key) return;
    try {
      await post("/api/device-key", { key });
      setDeviceKey(key);
      toast("This device stays signed in until you sign out");
      deviceKeyCard(card);
    } catch (err) { toast(err.message, true); }
  });
  card.querySelector("[data-forget]")?.addEventListener("click", async () => {
    setDeviceKey("");
    try { await del("/api/device-key"); } catch { /* signed out already */ }
    toast("Device key removed — this sign-in now ends like any other");
    deviceKeyCard(card);
  });
}
