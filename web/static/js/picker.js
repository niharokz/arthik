// The category picker: a sheet with search, your most used categories (last 90 days,
// on every device), recently used ones (this device) and a grid of groups that opens
// into their sub-categories, so a choice takes one or two taps.
// Split lines can also pick an account the money moves into (e.g. a loan).

import { html } from "./util.js";
import { S, sheet, kids, cat, acc, catName } from "./ui.js";
import { haptic } from "./native.js";
import { icon } from "./icons.js";

const RECENT_KEY = "arthik.recentCats";
const RECENT_MAX = 8;
const PINNED_MAX = 6;

const recent = () => { try { return JSON.parse(localStorage.getItem(RECENT_KEY) || "[]"); } catch { return []; } };
const remember = (id) => {
  try { localStorage.setItem(RECENT_KEY, JSON.stringify([id, ...recent().filter((x) => x !== id)].slice(0, RECENT_MAX))); } catch { /* ignore */ }
};

const fits = (c, kind) => c && (c.kind === kind || c.kind === "both");

// Most used categories of this kind, from the server's 90-day counts.
const pinned = (kind) => Object.entries(S.state.category_uses || {})
  .sort((a, b) => b[1] - a[1]).map(([id]) => cat(id)).filter((c) => fits(c, kind)).slice(0, PINNED_MAX);

/** Text for a chosen line: "🍽️ Food › Dining Out", "→ LIC HomeLoan" or a prompt. */
export function pickLabel(v, empty = "Choose category…") {
  if (v?.account) return "→ " + (acc(v.account)?.name || v.account);
  if (v?.category) {
    const c = cat(v.category);
    const icon = c?.icon || (c?.parent ? cat(c.parent)?.icon : "") || "🏷️";
    return `${icon} ${catName(v.category, true)}`;
  }
  return empty;
}

/**
 * Opens the picker. kind = expense | income. opts.current = selected category id,
 * opts.accounts = also offer accounts (split lines), opts.exclude = account id to hide.
 * Resolves to {category} | {account} | null (closed).
 */
export function pickCategory(kind, opts = {}) {
  const all = (S.state.categories || []).filter((c) => fits(c, kind));
  const parents = all.filter((c) => !c.parent);
  let open = cat(opts.current)?.parent || ""; // the group shown, "" = all groups
  let q = "";

  return new Promise((resolve) => {
    let done = false;
    const tile = (c, extra = "") => html`<button type="button" class="pick-tile ${c.id === opts.current ? "on" : ""} ${extra}" data-c="${c.id}">
      <span class="pi">${c.icon || cat(c.parent)?.icon || "🏷️"}</span><span class="pn">${c.name}</span>
      ${!c.parent && extra !== "whole" && kids(c.id).some((k) => fits(k, kind)) ? html`<span class="pk">${kids(c.id).filter((k) => fits(k, kind)).length} ›</span>` : ""}</button>`;

    const body = () => {
      if (q) {
        const hits = all.filter((c) => catName(c.id, true).toLowerCase().includes(q));
        return html`<div class="pick-grid">${hits.length ? hits.map((c) => tile({ ...c, name: catName(c.id, true) })) : html`<p class="muted">No category matches “${q}”.</p>`}</div>`;
      }
      if (open) {
        const p = cat(open);
        return html`<div class="row" style="margin-bottom:10px"><button type="button" class="ghost" data-back>‹ All groups</button>
          <b class="grow" style="text-align:right">${p.icon || ""} ${p.name}</b></div>
          <div class="pick-grid">${tile({ ...p, name: "All " + p.name }, "whole")}${kids(open).filter((k) => fits(k, kind)).map((k) => tile(k))}</div>`;
      }
      const pin = pinned(kind);
      const rec = recent().map(cat).filter((c) => fits(c, kind) && !pin.includes(c));
      const chip = (c) => html`<button type="button" class="chip" data-c="${c.id}">${c.icon || cat(c.parent)?.icon || ""} ${catName(c.id, true)}</button>`;
      const accounts = opts.accounts ? (S.state.accounts || []).filter((a) => !a.archived && a.id !== opts.exclude) : [];
      return html`${pin.length ? html`<h3 class="pick-h">${icon("pin", 13)} Most used</h3><div class="chips pick-recent pick-pinned">${pin.map(chip)}</div>` : ""}
        ${rec.length ? html`<h3 class="pick-h">Recent</h3><div class="chips pick-recent">${rec.map(chip)}</div>` : ""}
        <h3 class="pick-h">Categories</h3><div class="pick-grid">${parents.map((c) => tile(c))}</div>
        ${accounts.length ? html`<h3 class="pick-h">Or move it to an account</h3><div class="chips">${accounts.map((a) =>
          html`<button type="button" class="chip" data-a="${a.id}">→ ${a.name}</button>`)}</div>` : ""}`;
    };

    sheet(kind === "income" ? "Income category" : "Category",
      html`<input class="pick-q" type="search" placeholder="Search categories" aria-label="Search categories" autocomplete="off"><div data-pick></div>`,
      "", (d, close) => {
        const finish = (v) => { done = true; haptic("tick"); close(); resolve(v); };
        const draw = () => {
          d.querySelector("[data-pick]").innerHTML = String(body());
          d.querySelector("[data-back]")?.addEventListener("click", () => { open = ""; draw(); });
          d.querySelectorAll("[data-c]").forEach((b) => b.addEventListener("click", () => {
            const c = cat(b.dataset.c);
            const sub = kids(c.id).some((k) => fits(k, kind));
            if (!c.parent && sub && !q && !b.classList.contains("whole") && !b.closest(".pick-recent")) { open = c.id; draw(); return; }
            remember(c.id);
            finish({ category: c.id });
          }));
          d.querySelectorAll("[data-a]").forEach((b) => b.addEventListener("click", () => finish({ account: b.dataset.a })));
        };
        const search = d.querySelector(".pick-q");
        search.addEventListener("input", () => { q = search.value.trim().toLowerCase(); draw(); });
        d.addEventListener("close", () => { if (!done) resolve(null); });
        draw();
      });
  });
}
