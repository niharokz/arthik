// Loans plugin (personal): the lender's summary, the repayment schedule (lender rows,
// then estimated from the rate), this financial year's principal and interest, and
// "Split as EMI" in the transaction form: one bank debit becomes interest (category)
// + principal (paid into the loan account).

import { html, money, amt, paise, fromPaise, calc, fmtDate, fmtShort } from "../util.js";
import { ro, today, accName, catName, loading, toast } from "../ui.js";
import { get } from "../api.js";
import { openTx } from "../txform.js";
import { register } from "../plugins.js";

let cache = null; // GET /api/plugins/loans, kept for the form shortcut
const load = async () => (cache = await get("/api/plugins/loans"));

const pct = (part, whole) => (paise(whole) ? Math.min(100, Math.max(0, (paise(part) / paise(whole)) * 100)) : 0);

function scheduleTable(v) {
  return html`<details class="card"><summary><b>Repayment schedule</b> · ${v.schedule.length} EMIs
      <span class="muted">(${v.schedule.filter((r) => !r.estimated).length} from the lender, the rest estimated at ${v.rate}%)</span></summary>
    <div class="scroll-x"><table class="tbl">
      <thead><tr><th>Due</th><th class="r">Principal</th><th class="r">Interest</th><th class="r">Outstanding</th></tr></thead>
      <tbody>${v.schedule.map((r) => html`<tr class="${r.paid ? "faint" : ""} ${r.estimated ? "est" : ""}">
        <td>${fmtShort(r.date)}${r.estimated ? html` <span class="badge" title="Estimated from the rate">est.</span>` : ""}</td>
        <td class="r">${money(r.principal)}</td><td class="r">${money(r.interest)}</td><td class="r">${money(r.outstanding)}</td></tr>`)}</tbody>
    </table></div></details>`;
}

function loanCard(v) {
  const owed = -paise(v.balance);
  const repaid = fromPaise(Math.max(0, paise(v.disbursed) - owed));
  const s = v.statement || {};
  return html`<section class="card loan" data-loan="${v.id}">
      <header><h2>🏛️ ${v.name}</h2></header>
      <p class="muted" style="margin:-6px 0 12px">${v.lender}${v.loan_number ? " · " + v.loan_number : ""}${v.area_office ? " · " + v.area_office : ""}</p>
      ${v.problems.length ? html`<ul class="warns">${v.problems.map((p) => html`<li>${p}</li>`)}</ul>` : ""}
      <div class="grid four">
        <div class="stat"><div class="label">Owed (Arthik)</div><div class="value">${amt(fromPaise(owed))}</div>
          <div class="sub">${accName(v.account)}</div></div>
        <div class="stat"><div class="label">Next EMI</div><div class="value">${v.next ? amt(fromPaise(paise(v.next.principal) + paise(v.next.interest))) : "—"}</div>
          <div class="sub">${v.next ? html`${fmtDate(v.next.date)} · principal ${money(v.next.principal)} · interest ${money(v.next.interest)}` : "Paid off"}</div></div>
        <div class="stat"><div class="label">EMIs left</div><div class="value">${v.remaining}</div>
          <div class="sub">last ${v.payoff_date ? fmtShort(v.payoff_date) : "—"} · ${v.rate}% a year</div></div>
        <div class="stat"><div class="label">Interest still to pay</div><div class="value">${amt(v.interest_left)}</div><div class="sub">estimated</div></div>
      </div>
      <div style="margin:12px 0 4px" class="row between"><span class="muted">Repaid ${money(repaid)} of ${money(v.disbursed)} disbursed</span><b>${Math.round(pct(repaid, v.disbursed))}%</b></div>
      <div class="bar"><i style="width:${pct(repaid, v.disbursed)}%"></i></div>
      <h3 style="margin:16px 0 6px">This financial year · ${fmtShort(v.fy_from)} – ${fmtShort(v.fy_to)}</h3>
      <div class="grid two">
        <div class="stat"><div class="label">Principal paid</div><div class="value">${amt(v.fy_principal)}</div><div class="sub">counts towards 80C</div></div>
        <div class="stat"><div class="label">Interest paid</div><div class="value">${amt(v.fy_interest)}</div><div class="sub">counts towards 24(b)</div></div>
      </div>
      <p class="muted" style="font-size:.82rem;margin:8px 0 0">${s.as_of ? `Lender statement of ${fmtDate(s.as_of)} (principal outstanding ${money(s.principal_outstanding)}, ${s.unpaid_installments} unpaid EMIs) plus EMIs recorded in Arthik after it.` : "From EMIs recorded in Arthik."}
        EMIs are paid from ${accName(v.pay_from)}; interest goes to ${catName(v.interest_category, true)}.</p>
      ${ro() || !v.next ? "" : html`<div class="row" style="margin-top:12px"><button class="primary" data-emi="${v.id}">Record ${fmtShort(v.next.date)} EMI</button></div>`}
    </section>
    ${scheduleTable(v)}`;
}

/** The EMI as a split transaction: interest to the category, principal to the loan. */
const emiEntry = (v, r) => ({
  _prefill: true, type: "expense", title: `${v.name} EMI`, account: v.pay_from, date: r.date > today() ? today() : r.date,
  splits: [{ category: v.interest_category, amount: r.interest }, { account: v.account, amount: r.principal }],
});

async function render(root, ctx) {
  root.innerHTML = String(loading());
  const res = await load();
  if (!ctx.alive()) return;
  root.innerHTML = String(html`${res.loans.length ? res.loans.map(loanCard) : html`<div class="card empty">No loans in loans.md yet.</div>`}
    <p class="muted" style="font-size:.85rem">A personal plugin: it reads <code>loans.md</code> in your data folder. Each year, paste the lender's new schedule at the end of <code>schedule_text</code> exactly as the lender shows it — rows are merged by due date and checked.</p>`);
  root.querySelectorAll("[data-emi]").forEach((b) => b.addEventListener("click", () => {
    const v = res.loans.find((x) => x.id === b.dataset.emi);
    openTx(emiEntry(v, v.next), () => ctx.refresh());
  }));
}

// "Split as EMI" in the transaction form when the amount is an EMI.
const emiOf = (st) => {
  if (st.type !== "expense" || st.splits.length || !cache) return null;
  const p = calc(st.amount);
  return Number.isNaN(p) ? null : cache.loans.find((v) => v.emis.some((e) => paise(e) === p)) || null;
};

const formHook = {
  show: (st) => !!emiOf(st),
  label: (st) => `🏛️ Split as ${emiOf(st).name} EMI`,
  async apply(st) {
    const r = await get(`/api/plugins/loans/split?amount=${fromPaise(calc(st.amount))}&date=${st.date}`);
    Object.assign(st, {
      title: st.title || `${r.name} EMI`,
      splits: [{ category: r.category, account: "", amount: r.interest }, { category: "", account: r.account, amount: r.principal }],
    });
    toast(`Principal ${money(r.principal)} → ${accName(r.account)} · interest ${money(r.interest)}${r.estimated ? " (estimated)" : ""}`);
    return true;
  },
};

register("loans", { title: "Loans", icon: "🏛️", render, formHook });
load().catch(() => { /* offline: the form shortcut stays hidden */ });
