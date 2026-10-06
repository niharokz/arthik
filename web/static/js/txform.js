// The transaction sheet: add / edit / duplicate an expense, income or transfer,
// with Bluecoins' calculator amount, remembered items, splits, labels, status,
// past/future dates and an optional repeat (which creates a scheduled reminder).

import { html, raw, esc, calc, fromPaise, paise, money, todayStr, nowTime, newId, debounce, qs } from "./util.js";
import { S, sheet, toast, confirmSheet, catOptions, accOptions, ro, today, cat } from "./ui.js";
import { get, txWrite } from "./api.js";

const REPEATS = [["none", "Does not repeat"], ["daily", "Daily"], ["weekly", "Weekly"], ["biweekly", "Every 2 weeks"], ["monthly", "Monthly"],
  ["bimonthly", "Every 2 months"], ["quarterly", "Quarterly"], ["halfyearly", "Half-yearly"], ["yearly", "Yearly"]];

const lastAcc = () => { try { return localStorage.getItem("arthik.lastAccount") || ""; } catch { return ""; } };
const setLastAcc = (id) => { try { localStorage.setItem("arthik.lastAccount", id); } catch { /* ignore */ } };

/**
 * Opens the form. tx = existing view.Tx to edit (or a prefill object for a new one).
 * onSaved() is called after a successful save/delete.
 */
export function openTx(tx = {}, onSaved = () => {}) {
  if (ro()) { toast("The demo is read-only"); return; }
  const editing = !!(tx.id && !tx._prefill);
  const accounts = (S.state.accounts || []).filter((a) => !a.archived);
  if (!accounts.length) { toast("Add an account first (Accounts → Add)", true); return; }
  const first = lastAcc() && accounts.find((a) => a.id === lastAcc()) ? lastAcc() : accounts[0].id;

  const st = {
    type: tx.type === "journal" ? "journal" : tx.type || "expense",
    amount: tx.amount ? fromPaise(Math.abs(paise(tx.amount))) : "",
    negative: tx.amount ? paise(tx.amount) < 0 : false,
    title: tx.title || "",
    account: tx.account || first,
    to: tx.to_account || "",
    category: tx.category || "",
    splits: (tx.splits || []).map((s) => ({ category: s.category, amount: s.amount })),
    date: tx.date || today(),
    time: tx.time || (editing ? "" : nowTime()),
    status: tx.status || "uncleared",
    labels: [...(tx.labels || [])],
    notes: tx.notes || "",
    repeat: "none", endDate: "", count: "",
  };
  if (st.type === "journal") {
    toast("This entry has custom postings — edit it in its transaction file", true);
    return;
  }

  let s;
  const body = () => html`
    <div class="seg full" role="tablist" style="margin-bottom:14px">
      ${[["expense", "Expense"], ["income", "Income"], ["transfer", "Transfer"]].map(([v, l]) =>
        html`<button type="button" data-type="${v}" class="${st.type === v ? "on" : ""}">${l}</button>`)}
    </div>
    ${st.splits.length ? "" : html`
      <label class="f"><span>Amount (₹) — you can type 120+45</span>
        <input name="amount" class="amount-in" inputmode="decimal" autocomplete="off" value="${st.amount}" placeholder="0.00" autofocus></label>
      <div class="calc-hint" data-calc></div>`}
    <label class="f"><span>Title</span>
      <input name="title" list="tx-suggest" autocomplete="off" value="${st.title}" placeholder="What was it?">
      <datalist id="tx-suggest"></datalist></label>
    ${st.type === "transfer" ? html`
      <div class="fields two">
        <label class="f"><span>From account</span><select name="account">${accOptions(st.account)}</select></label>
        <label class="f"><span>To account</span><select name="to"><option value="">Choose…</option>${accOptions(st.to)}</select></label>
      </div>` : html`
      <label class="f"><span>Account</span><select name="account">${accOptions(st.account)}</select></label>
      ${st.splits.length ? splitsBlock() : html`
        <label class="f"><span>Category</span><select name="category"><option value="">Choose…</option>${catOptions(st.type, st.category)}</select></label>`}
      <div class="row" style="margin:-4px 0 12px"><button type="button" class="ghost" data-split>${st.splits.length ? "Remove split" : "Split into categories"}</button>
        ${st.splits.length ? "" : html`<label class="check" style="margin:0 0 0 auto"><input type="checkbox" name="negative" ${st.negative ? raw("checked") : ""}> ${st.type === "expense" ? "Refund" : "Reversal"}</label>`}</div>`}
    <div class="fields two">
      <label class="f"><span>Date ${st.date > today() ? html`<span class="badge warn">future — posts on this date</span>` : ""}</span><input type="date" name="date" value="${st.date}"></label>
      <label class="f"><span>Time</span><input type="time" name="time" value="${st.time}"></label>
    </div>
    <label class="f"><span>Labels</span>
      <div class="chips" data-labels>${(S.state.labels || []).map((l) =>
        html`<button type="button" class="chip ${st.labels.includes(l.id) ? "on" : ""}" data-label="${l.id}">${l.name}</button>`)}
        ${st.labels.filter((l) => !(S.state.labels || []).some((x) => x.id === l)).map((l) => html`<button type="button" class="chip on" data-label="${l}">${l}</button>`)}
      </div>
      <input name="newlabel" placeholder="New label, press Enter" style="margin-top:8px"></label>
    <div class="fields two">
      <label class="f"><span>Status</span><select name="status">
        ${[["uncleared", "Uncleared"], ["cleared", "Cleared"], ["reconciled", "Reconciled"]].map(([v, l]) => html`<option value="${v}" ${st.status === v ? raw("selected") : ""}>${l}</option>`)}
      </select></label>
      ${editing ? "" : html`<label class="f"><span>Repeat</span><select name="repeat">
        ${REPEATS.map(([v, l]) => html`<option value="${v}" ${st.repeat === v ? raw("selected") : ""}>${l}</option>`)}</select></label>`}
    </div>
    ${!editing && st.repeat !== "none" ? html`<div class="fields two">
      <label class="f"><span>End date (optional)</span><input type="date" name="endDate" value="${st.endDate}"></label>
      <label class="f"><span>Or stop after N times</span><input name="count" inputmode="numeric" value="${st.count}" placeholder="unlimited"></label>
    </div><p class="muted" style="margin-top:-6px;font-size:.85rem">Saved as a scheduled reminder; every occurrence up to today posts now, later ones post on their dates.</p>` : ""}
    <label class="f"><span>Notes</span><textarea name="notes">${st.notes}</textarea></label>
    ${editing && tx.reminder ? html`<p class="faint" style="font-size:.8rem">Posted by a scheduled reminder.</p>` : ""}
    <p class="faint" style="font-size:.78rem;margin:0" data-entry></p>`;

  function splitsBlock() {
    const total = st.splits.reduce((a, x) => a + (calc(x.amount) || 0), 0);
    return html`<div class="f"><span class="muted" style="font-size:.8rem;display:block;margin-bottom:4px">Split lines</span>
      ${st.splits.map((x, i) => html`<div class="split-line">
        <select data-scat="${i}"><option value="">Category…</option>${catOptions(st.type, x.category)}</select>
        <input data-samt="${i}" inputmode="decimal" value="${x.amount}" placeholder="0.00" aria-label="Amount">
        <button type="button" class="icon ghost" data-sdel="${i}" aria-label="Remove line">✕</button></div>`)}
      <div class="row between"><button type="button" class="ghost" data-sadd>+ Add line</button><span>Total <b class="amt">${money(total / 100)}</b></span></div></div>`;
  }

  const actions = html`
    ${editing ? html`<button type="button" class="danger" data-del>Delete</button><button type="button" data-dup>Duplicate</button>` : ""}
    <span class="grow"></span><button type="button" data-close>Cancel</button><button type="button" class="primary" data-save>Save</button>`;

  s = sheet(editing ? "Edit transaction" : "Add transaction", body(), actions, (d, close) => {
    const pull = () => {
      const g = (n) => d.querySelector(`[name="${n}"]`);
      if (g("amount")) st.amount = g("amount").value;
      st.title = g("title").value;
      st.account = g("account")?.value || st.account;
      if (g("to")) st.to = g("to").value;
      if (g("category")) st.category = g("category").value;
      if (g("negative")) st.negative = g("negative").checked;
      st.date = g("date").value;
      st.time = g("time").value;
      st.status = g("status").value;
      if (g("repeat")) st.repeat = g("repeat").value;
      if (g("endDate")) st.endDate = g("endDate").value;
      if (g("count")) st.count = g("count").value;
      st.notes = g("notes").value;
      d.querySelectorAll("[data-scat]").forEach((el) => (st.splits[+el.dataset.scat].category = el.value));
      d.querySelectorAll("[data-samt]").forEach((el) => (st.splits[+el.dataset.samt].amount = el.value));
    };
    const redraw = () => {
      pull();
      d.querySelector(".sheet-b").innerHTML = String(body());
      wire();
    };
    const hint = () => {
      const a = d.querySelector('[name="amount"]'), h = d.querySelector("[data-calc]");
      if (!a || !h) return;
      const v = calc(a.value);
      h.textContent = /[+\-*/()]/.test(a.value.replace(/^-/, "")) ? (Number.isNaN(v) ? "…" : "= " + money(v / 100)) : "";
    };
    const suggest = debounce(async (q) => {
      try {
        const list = await get("/api/suggest" + qs({ q, type: st.type }));
        S._suggest = list;
        const dl = d.querySelector("#tx-suggest");
        if (dl) dl.innerHTML = list.map((x) => `<option value="${esc(x.title)}">${esc(money(x.amount))}${x.category ? " · " + esc(cat(x.category)?.name || "") : ""}</option>`).join("");
      } catch { /* offline: no suggestions */ }
    }, 200);
    function wire() {
      d.querySelectorAll("[data-type]").forEach((b) => b.addEventListener("click", () => {
        pull();
        st.type = b.dataset.type;
        if (st.type === "transfer") st.splits = [];
        if (st.category && cat(st.category) && cat(st.category).kind !== st.type && cat(st.category).kind !== "both") st.category = "";
        redraw();
      }));
      d.querySelector('[name="amount"]')?.addEventListener("input", hint);
      const title = d.querySelector('[name="title"]');
      title.addEventListener("input", () => {
        suggest(title.value);
        const hit = (S._suggest || []).find((x) => x.title.toLowerCase() === title.value.trim().toLowerCase());
        if (hit && !editing) { // remembered item: fill what is still empty
          pull();
          if (!st.amount) st.amount = hit.amount;
          if (hit.type === st.type) {
            if (!st.category && hit.category) st.category = hit.category;
            if (hit.account) st.account = hit.account;
            if (hit.to_account) st.to = hit.to_account;
          }
          redraw();
          const t = d.querySelector('[name="title"]');
          t.focus();
          t.setSelectionRange(t.value.length, t.value.length);
        }
      });
      d.querySelector('[name="date"]').addEventListener("change", redraw);
      d.querySelector('[name="repeat"]')?.addEventListener("change", redraw);
      d.querySelector("[data-split]")?.addEventListener("click", () => {
        pull();
        if (st.splits.length) {
          st.amount = fromPaise(st.splits.reduce((a, x) => a + (calc(x.amount) || 0), 0));
          st.category = st.splits[0]?.category || "";
          st.splits = [];
        } else {
          st.splits = [{ category: st.category, amount: st.amount }, { category: "", amount: "" }];
        }
        redraw();
      });
      d.querySelector("[data-sadd]")?.addEventListener("click", () => { pull(); st.splits.push({ category: "", amount: "" }); redraw(); });
      d.querySelectorAll("[data-sdel]").forEach((b) => b.addEventListener("click", () => { pull(); st.splits.splice(+b.dataset.sdel, 1); redraw(); }));
      d.querySelectorAll("[data-samt]").forEach((el) => el.addEventListener("change", redraw));
      d.querySelectorAll("[data-label]").forEach((b) => b.addEventListener("click", () => {
        const id = b.dataset.label;
        st.labels = st.labels.includes(id) ? st.labels.filter((x) => x !== id) : [...st.labels, id];
        b.classList.toggle("on");
      }));
      const nl = d.querySelector('[name="newlabel"]');
      nl.addEventListener("keydown", (e) => {
        if (e.key !== "Enter") return;
        e.preventDefault();
        const v = nl.value.trim();
        if (v && !st.labels.includes(v)) { pull(); st.labels.push(v); redraw(); }
      });
      hint();
    }
    wire();

    const save = async () => {
      pull();
      const e = { type: st.type, title: st.title.trim(), account: st.account, date: st.date, time: st.time,
        status: st.status, labels: st.labels, notes: st.notes.trim() };
      if (!e.title) return toast("Enter a title", true);
      if (!e.date) return toast("Choose a date", true);
      if (st.type === "transfer") {
        const v = calc(st.amount);
        if (!(v > 0)) return toast("Enter a positive amount", true);
        if (!st.to) return toast("Choose the account to transfer to", true);
        e.amount = fromPaise(v);
        e.to_account = st.to;
      } else if (st.splits.length) {
        const lines = st.splits.filter((x) => x.category || x.amount);
        for (const x of lines) {
          if (!x.category || Number.isNaN(calc(x.amount)) || !calc(x.amount)) return toast("Every split line needs a category and an amount", true);
        }
        if (lines.length < 2) return toast("A split needs at least two lines", true);
        e.splits = lines.map((x) => ({ category: x.category, amount: fromPaise(calc(x.amount)) }));
      } else {
        const v = calc(st.amount);
        if (Number.isNaN(v) || !v) return toast("Enter an amount", true);
        if (!st.category) return toast("Choose a category", true);
        e.amount = fromPaise(st.negative ? -Math.abs(v) : v);
        e.category = st.category;
      }
      if (!editing && st.repeat !== "none") {
        e.repeat = st.repeat;
        if (st.endDate) e.end_date = st.endDate;
        if (st.count) e.count = parseInt(st.count, 10) || 0;
      }
      const btn = d.querySelector("[data-save]");
      btn.disabled = true;
      try {
        let r;
        if (editing) r = await txWrite("PUT", "/api/transactions/" + encodeURIComponent(tx.id), e);
        else { e.id = newId(e.date); r = await txWrite("POST", "/api/transactions", e); }
        setLastAcc(st.account);
        close();
        toast(r.queued ? "Saved offline — it will sync when you are back online" : editing ? "Saved" : e.date > today() ? "Scheduled for " + e.date : "Added");
        onSaved(r);
      } catch (err) {
        btn.disabled = false;
        toast(err.message, true);
      }
    };
    d.querySelector("[data-save]").addEventListener("click", save);
    d.querySelector(".sheet-b").addEventListener("keydown", (ev) => {
      if (ev.key === "Enter" && (ev.ctrlKey || ev.metaKey)) save();
    });
    d.querySelector("[data-dup]")?.addEventListener("click", () => {
      pull();
      close();
      openTx({ ...tx, _prefill: true, id: undefined, date: todayStr(), time: nowTime(), status: "uncleared", reminder: undefined }, onSaved);
    });
    d.querySelector("[data-del]")?.addEventListener("click", async () => {
      if (!(await confirmSheet("Delete transaction?", `“${tx.title}” will be removed from every balance and total.`))) return;
      try {
        const r = await txWrite("DELETE", "/api/transactions/" + encodeURIComponent(tx.id));
        close();
        toast(r.queued ? "Deleted offline — will sync" : "Deleted");
        onSaved(r);
      } catch (err) { toast(err.message, true); }
    });
  });
  return s;
}
