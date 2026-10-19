// The transaction sheet: add / edit / duplicate an expense, income or transfer,
// with Bluecoins' calculator amount, remembered items, splits, labels, status and
// past/future dates. It also reviews inbox drafts: saving accepts the draft.

import { html, raw, calc, fromPaise, paise, money, todayStr, nowTime, newId, debounce, qs, fmtShort } from "./util.js";
import { S, sheet, toast, confirmSheet, catOptions, accOptions, ro, today, cat, accName, catName } from "./ui.js";
import { get, post, txWrite } from "./api.js";
import { haptic } from "./native.js";

const lastAcc = () => { try { return localStorage.getItem("arthik.lastAccount") || ""; } catch { return ""; } };
const setLastAcc = (id) => { try { localStorage.setItem("arthik.lastAccount", id); } catch { /* ignore */ } };

/**
 * Opens the form. tx = existing view.Tx to edit (or a prefill object for a new one).
 * opts.draft = an inbox draft to review (saving accepts it).
 * onSaved() is called after a successful save/delete.
 */
export function openTx(tx = {}, onSaved = () => {}, opts = {}) {
  if (ro()) { toast("The demo is read-only"); return; }
  const draft = opts.draft || null;
  if (draft) tx = { ...draft, _prefill: true };
  const editing = !!(tx.id && !tx._prefill);
  const accounts = (S.state.accounts || []).filter((a) => !a.archived);
  if (!accounts.length) { toast("Add an account first (Accounts → Add)", true); return; }
  const first = draft ? "" : lastAcc() && accounts.find((a) => a.id === lastAcc()) ? lastAcc() : accounts[0].id;
  // Offer to remember SMS digits when the account was not found from them.
  const askDigits = !!(draft?.digits && !(draft.account || draft.to_account));

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
    remember: askDigits,
  };
  if (st.type === "journal") {
    toast("This entry has custom postings — edit it in its transaction file", true);
    return;
  }

  let s;
  const choose = draft ? html`<option value="">Choose…</option>` : "";
  const body = () => html`
    ${draft?.raw ? html`<div class="draft-raw"><small>${draft.source === "sms" ? "SMS" : draft.source}</small>${draft.raw}${draft.pair_raw ? html`<hr>${draft.pair_raw}` : ""}</div>` : ""}
    <div class="seg full" role="tablist" style="margin-bottom:14px">
      ${[["expense", "Expense"], ["income", "Income"], ["transfer", "Transfer"]].map(([v, l]) =>
        html`<button type="button" data-type="${v}" class="${st.type === v ? "on" : ""}">${l}</button>`)}
    </div>
    <div class="f ac">
      <label for="tx-title" class="muted" style="font-size:.8rem;display:block;margin-bottom:4px">Title${editing ? "" : " — type a few letters to reuse a past entry"}</label>
      <input id="tx-title" name="title" autocomplete="off" value="${st.title}" placeholder="What was it?" ${editing || draft ? "" : raw("autofocus")}
        role="combobox" aria-autocomplete="list" aria-expanded="false" aria-controls="tx-ac">
      <div class="ac-list" id="tx-ac" role="listbox" hidden></div>
    </div>
    ${st.splits.length ? "" : html`
      <label class="f"><span>Amount (₹) — you can type 120+45</span>
        <input name="amount" class="amount-in" inputmode="decimal" autocomplete="off" value="${st.amount}" placeholder="0.00" ${editing ? raw("autofocus") : ""}></label>
      <div class="calc-hint" data-calc></div>`}
    ${st.type === "transfer" ? html`
      <div class="fields two">
        <label class="f"><span>From account</span><select name="account">${choose}${accOptions(st.account)}</select></label>
        <label class="f"><span>To account</span><select name="to"><option value="">Choose…</option>${accOptions(st.to)}</select></label>
      </div>` : html`
      <label class="f"><span>Account</span><select name="account">${choose}${accOptions(st.account)}</select></label>
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
    <label class="f"><span>Status</span><select name="status">
      ${[["uncleared", "Uncleared"], ["cleared", "Cleared"], ["reconciled", "Reconciled"]].map(([v, l]) => html`<option value="${v}" ${st.status === v ? raw("selected") : ""}>${l}</option>`)}
    </select></label>
    ${askDigits ? html`<label class="check"><input type="checkbox" name="remember" ${st.remember ? raw("checked") : ""}> Remember …${draft.digits} for this account</label>` : ""}
    <label class="f"><span>Notes</span><textarea name="notes">${st.notes}</textarea></label>
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
    <span class="grow"></span><button type="button" data-close>Cancel</button><button type="button" class="primary" data-save>${draft ? "Accept" : "Save"}</button>`;

  s = sheet(draft ? "Review draft" : editing ? "Edit transaction" : "Add transaction", body(), actions, (d, close) => {
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
      if (g("remember")) st.remember = g("remember").checked;
      st.notes = g("notes").value;
      d.querySelectorAll("[data-scat]").forEach((el) => (st.splits[+el.dataset.scat].category = el.value));
      d.querySelectorAll("[data-samt]").forEach((el) => (st.splits[+el.dataset.samt].amount = el.value));
    };
    const render = () => { // draws the form from st as it is
      d.querySelector(".sheet-b").innerHTML = String(body());
      wire();
    };
    const redraw = () => { pull(); render(); }; // keeps what was typed
    const hint = () => {
      const a = d.querySelector('[name="amount"]'), h = d.querySelector("[data-calc]");
      if (!a || !h) return;
      const v = calc(a.value);
      h.textContent = /[+\-*/()]/.test(a.value.replace(/^-/, "")) ? (Number.isNaN(v) ? "…" : "= " + money(v / 100)) : "";
    };
    // ---- remembered transactions (autofill) ----
    let acItems = [], acIndex = -1;
    const acList = () => d.querySelector("#tx-ac");
    const acHide = () => {
      const l = acList();
      if (l) { l.hidden = true; d.querySelector("#tx-title")?.setAttribute("aria-expanded", "false"); }
      acIndex = -1;
    };
    const acLine = (x) => {
      const where = x.type === "transfer" ? `${accName(x.account)} → ${accName(x.to_account)}`
        : `${x.splits?.length ? "Split · " + x.splits.length + " categories" : catName(x.category, true)} · ${accName(x.account)}`;
      return html`<button type="button" class="ac-item" role="option" data-ac="${acItems.indexOf(x)}">
        <span class="ac-main"><b>${x.title}</b><small>${where}</small></span>
        <span class="ac-end"><span class="amt ${x.type === "income" ? "pos" : x.type === "expense" ? "neg" : ""}">${money(x.amount)}</span>
          <small>${x.uses > 1 ? `${x.uses}×` : fmtShort(x.last)}</small></span></button>`;
    };
    const acShow = () => {
      const l = acList();
      if (!l) return;
      if (!acItems.length) return acHide();
      l.innerHTML = String(html`${acItems.map(acLine)}`);
      l.hidden = false;
      d.querySelector("#tx-title")?.setAttribute("aria-expanded", "true");
      l.querySelectorAll("[data-ac]").forEach((b) => b.addEventListener("pointerdown", (e) => { e.preventDefault(); acPick(acItems[+b.dataset.ac]); }));
    };
    const suggest = debounce(async (q) => {
      if (editing) return;
      try {
        acItems = await get("/api/suggest" + qs({ q, type: st.type, limit: 6 }));
        if (document.activeElement === d.querySelector("#tx-title")) acShow();
      } catch { acItems = []; acHide(); } // offline web app: no suggestions
    }, 150);
    // Fills the form from a remembered transaction — everything except the amount,
    // which stays what you typed (or what the SMS said). Date and time stay too.
    const acPick = (x) => {
      if (!x) return;
      pull();
      Object.assign(st, {
        type: x.type, title: x.title, notes: x.notes || "", labels: [...(x.labels || [])],
        account: x.account || st.account, to: x.to_account || "", category: x.category || "",
        splits: (x.splits || []).map((sp) => ({ category: sp.category, amount: "" })), // categories only
      });
      acHide();
      render(); // not redraw: that would read the old fields back over st
      haptic("tick");
      const a = d.querySelector('[name="amount"]'); // ready to type the amount
      if (a && !a.value) a.focus();
    };

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
      title.addEventListener("input", () => suggest(title.value));
      title.addEventListener("focus", () => suggest(title.value));
      title.addEventListener("blur", () => setTimeout(acHide, 120));
      title.addEventListener("keydown", (e) => { // arrows + Enter on a keyboard
        const l = acList();
        if (!l || l.hidden) return;
        if (e.key === "ArrowDown" || e.key === "ArrowUp") {
          e.preventDefault();
          acIndex = (acIndex + (e.key === "ArrowDown" ? 1 : -1) + acItems.length) % acItems.length;
          l.querySelectorAll("[data-ac]").forEach((b, i) => b.classList.toggle("on", i === acIndex));
        } else if (e.key === "Enter" && acIndex >= 0) {
          e.preventDefault();
          acPick(acItems[acIndex]);
        } else if (e.key === "Escape") { // close the list, not the sheet
          e.preventDefault();
          e.stopPropagation();
          acHide();
        }
      });
      d.querySelector('[name="date"]').addEventListener("change", redraw);
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
      if (!e.account) return toast(st.type === "transfer" ? "Choose the account it came from" : "Choose an account", true);
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
      const btn = d.querySelector("[data-save]");
      btn.disabled = true;
      try {
        let r;
        if (draft) r = await post("/api/inbox/" + encodeURIComponent(draft.id) + "/accept", { ...e, remember_digits: !!st.remember });
        else if (editing) r = await txWrite("PUT", "/api/transactions/" + encodeURIComponent(tx.id), e);
        else { e.id = newId(e.date); r = await txWrite("POST", "/api/transactions", e); }
        setLastAcc(st.account);
        haptic("confirm");
        close();
        toast(r.queued ? "Saved offline — it will sync when you are back online" : draft ? "Accepted" : editing ? "Saved" : e.date > today() ? "Scheduled for " + e.date : "Added");
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
      openTx({ ...tx, _prefill: true, id: undefined, date: todayStr(), time: nowTime(), status: "uncleared" }, onSaved);
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
