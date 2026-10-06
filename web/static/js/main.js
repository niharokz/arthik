// App shell: sign-in, navigation (bottom bar on phones, sidebar on desktop), hash
// router, the "+" button, banners (demo, data problems, offline queue) and the
// service worker.

import { html, raw } from "./util.js";
import { S, loadState, toast, ro } from "./ui.js";
import { get, post, flush, onQueue, queue } from "./api.js";
import { openTx } from "./txform.js";

import * as dashboard from "./views/dashboard.js";
import * as transactions from "./views/transactions.js";
import * as accounts from "./views/accounts.js";
import * as categories from "./views/categories.js";
import * as budgets from "./views/budgets.js";
import * as scheduled from "./views/scheduled.js";
import * as reports from "./views/reports.js";
import * as calendar from "./views/calendar.js";
import * as labels from "./views/labels.js";
import * as settings from "./views/settings.js";

// route → [module, nav key, title]
const ROUTES = {
  home: [dashboard, "home", "Overview"],
  tx: [transactions, "tx", "Transactions"],
  calendar: [calendar, "calendar", "Calendar"],
  accounts: [accounts, "accounts", "Accounts"],
  account: [accounts, "accounts", "Account"],
  categories: [categories, "categories", "Categories"],
  category: [categories, "categories", "Category"],
  budgets: [budgets, "budgets", "Budgets"],
  scheduled: [scheduled, "scheduled", "Scheduled"],
  reports: [reports, "reports", "Reports"],
  labels: [labels, "labels", "Labels"],
  settings: [settings, "settings", "Settings"],
  more: [settings, "more", "More"],
};

const NAV = [
  ["home", "Overview", "🏠"], ["tx", "Transactions", "🧾"], ["calendar", "Calendar", "📅"],
  ["accounts", "Accounts", "🏦"], ["categories", "Categories", "🗂️"], ["budgets", "Budgets", "🎯"],
  ["scheduled", "Scheduled", "⏰"], ["reports", "Reports", "📊"], ["labels", "Labels", "🏷️"], ["settings", "Settings", "⚙️"],
];
const BOTTOM = [["home", "Home", "🏠"], ["tx", "Transactions", "🧾"], ["accounts", "Accounts", "🏦"], ["reports", "Reports", "📊"], ["more", "More", "☰"]];

const root = document.getElementById("app");

function parseHash() {
  const h = location.hash.replace(/^#\/?/, "");
  const [path, q = ""] = h.split("?");
  const [name, ...rest] = path.split("/");
  return { name: ROUTES[name] ? name : "home", arg: decodeURIComponent(rest.join("/")), params: new URLSearchParams(q) };
}

export const go = (hash) => { if (location.hash !== hash) location.hash = hash; else route(); };

function shell() {
  const st = S.state;
  root.innerHTML = String(html`<div class="app">
    <nav class="side" aria-label="Main">
      <a class="brand" href="#/home"><img src="/icon.svg" alt="">arthik</a>
      ${ro() ? "" : html`<button class="primary add" data-add>＋ Add transaction</button>`}
      ${NAV.map(([k, l, i]) => html`<a class="nav" href="#/${k}" data-nav="${k}"><span>${i}</span>${l}</a>`)}
      <div class="foot">${st.user}${ro() ? " · read-only demo" : ""}<br>arthik ${st.version}</div>
    </nav>
    <div class="main">
      <header class="top"><button class="ghost icon" data-back hidden aria-label="Back">‹</button><h1 data-title>arthik</h1><span data-qbadge></span><span data-actions class="row"></span></header>
      <div data-banners></div>
      <div class="content" data-view></div>
    </div>
    <nav class="bottom" aria-label="Main">${BOTTOM.map(([k, l, i]) => html`<a href="#/${k}" data-nav="${k}"><span class="ic">${i}</span>${l}</a>`)}</nav>
    ${ro() ? "" : html`<button class="fab" data-add aria-label="Add transaction">＋</button>`}
  </div>`);
  root.querySelectorAll("[data-add]").forEach((b) => b.addEventListener("click", () => openTx({}, () => refresh())));
  root.querySelector("[data-back]").addEventListener("click", () => history.back());
  banners();
  queueBadge(queue().length);
}

function banners() {
  const st = S.state;
  const el = root.querySelector("[data-banners]");
  if (!el) return;
  const parts = [];
  if (st.read_only) parts.push(html`<div class="banner demo">👀 You are viewing the <b>read-only demo</b> with made-up data. <a href="/">About arthik</a></div>`);
  if (st.load_error) parts.push(html`<div class="banner warn">⚠️ A data file could not be read, so changes are paused until it is fixed: <code>${st.load_error}</code></div>`);
  else if ((st.problems || []).length) parts.push(html`<div class="banner warn">⚠️ ${st.problems.length} data problem(s) — those entries are left out of totals. <a href="#/settings">See details</a></div>`);
  el.innerHTML = String(html`${parts}`);
}

function queueBadge(n) {
  const el = root.querySelector("[data-qbadge]");
  if (el) el.innerHTML = n ? String(html`<span class="badge warn" title="Changes waiting to sync">⟳ ${n} pending</span>`) : "";
}

let current = null;
async function route() {
  if (!S.state) return;
  const r = parseHash();
  const [mod, nav, title] = ROUTES[r.name];
  root.querySelectorAll("[data-nav]").forEach((a) => a.classList.toggle("on", a.dataset.nav === nav || (nav === "more" && a.dataset.nav === "more")));
  const view = root.querySelector("[data-view]");
  const titleEl = root.querySelector("[data-title]");
  const actions = root.querySelector("[data-actions]");
  titleEl.textContent = title;
  actions.innerHTML = "";
  root.querySelector("[data-back]").hidden = !r.arg;
  const token = {};
  current = token;
  const ctx = {
    name: r.name, arg: r.arg, params: r.params,
    title: (t) => (titleEl.textContent = t),
    actions: (h) => (actions.innerHTML = String(h)),
    actionsEl: actions,
    alive: () => current === token,
    refresh,
    go,
  };
  if (r.params.get("add") === "1" && !ro()) { // PWA shortcut "Add transaction"
    history.replaceState(null, "", "#/" + r.name);
    openTx({}, () => refresh());
  }
  try {
    await mod.render(view, ctx);
  } catch (e) {
    if (current === token) view.innerHTML = String(html`<div class="card"><p class="neg">${e.message}</p></div>`);
  }
  window.scrollTo({ top: 0 });
}

/** Reloads /api/state and re-renders the current view. */
export async function refresh() {
  try {
    await loadState();
    banners();
  } catch (e) { toast(e.message, true); }
  await route();
}

// ---- sign-in ----

async function loginScreen(me) {
  const demo = me.demo;
  root.innerHTML = String(html`<div class="center"><form class="card login" data-login>
    <div class="brand"><img src="/icon.svg" alt=""><div><h1>arthik</h1><small>double-entry personal finance</small></div></div>
    <label class="f"><span>User</span><input name="user" autocomplete="username" required autofocus></label>
    <label class="f"><span>Password</span><input name="password" type="password" autocomplete="current-password" required></label>
    <button class="primary block" type="submit">Sign in</button>
    <p class="neg" data-err hidden></p>
    ${demo ? html`<div class="demo-box"><b>Try the demo</b> (read-only, made-up data)<br>
      user <code>${demo.user}</code> · password <code>${demo.password}</code>
      <div style="margin-top:8px"><button type="button" data-demo>Open the demo</button></div></div>` : ""}
    <p style="margin:14px 0 0;font-size:.85rem"><a href="/">About arthik</a></p>
  </form></div>`);
  const f = root.querySelector("[data-login]");
  const submit = async (user, password) => {
    try {
      await post("/api/login", { user, password });
      await boot();
    } catch (e) {
      const el = f.querySelector("[data-err]");
      el.hidden = false;
      el.textContent = e.message;
    }
  };
  f.addEventListener("submit", (e) => { e.preventDefault(); submit(f.user.value, f.password.value); });
  f.querySelector("[data-demo]")?.addEventListener("click", () => submit(demo.user, demo.password));
}

export async function logout() {
  try { await post("/api/logout", {}); } catch { /* ignore */ }
  try { const keys = await caches.keys(); await Promise.all(keys.filter((k) => k.includes("api")).map((k) => caches.delete(k))); } catch { /* ignore */ }
  S.state = null;
  location.hash = "";
  boot();
}

async function boot() {
  let me;
  try {
    me = await get("/api/me");
  } catch (e) {
    root.innerHTML = String(html`<div class="center"><div class="card login"><h1>arthik</h1><p>Can't reach the server${navigator.onLine ? "" : " (you are offline)"}.</p><button data-retry>Try again</button></div></div>`);
    root.querySelector("[data-retry]").addEventListener("click", boot);
    return;
  }
  S.me = me;
  if (!me.signed_in) return loginScreen(me);
  try {
    await loadState();
  } catch (e) {
    root.innerHTML = String(html`<div class="center"><div class="card login"><h1>arthik</h1><p class="neg">${e.message}</p><button data-retry>Try again</button></div></div>`);
    root.querySelector("[data-retry]").addEventListener("click", boot);
    return;
  }
  shell();
  await route();
  syncNow();
}

// ---- offline sync ----

async function syncNow() {
  if (!queue().length || !navigator.onLine) return;
  const r = await flush();
  if (r.sent) toast(`Synced ${r.sent} offline change(s)`);
  for (const f of r.failed) toast(`Could not sync “${f.item.body?.title || f.item.url}”: ${f.error}`, true);
  if (r.sent || r.failed.length) refresh();
}

onQueue(queueBadge);
document.addEventListener("click", (e) => { // "hide amounts": tap one to peek
  const a = document.body.classList.contains("hide-amounts") && e.target.closest(".amt");
  if (a) { a.classList.add("peek"); setTimeout(() => a.classList.remove("peek"), 3000); }
}, true);
window.addEventListener("online", syncNow);
setInterval(syncNow, 30000);
window.addEventListener("hashchange", route);
window.addEventListener("arthik:signedout", () => { if (S.state) { S.state = null; boot(); } });

if ("serviceWorker" in navigator) {
  navigator.serviceWorker.register("/sw.js", { scope: "/" }).catch(() => {});
}

boot();
export { raw };
