// Settings → Menu order: move screens up or down. The first four become the phone's
// bottom bar; the order is saved in settings.md, so the web app and the phone match.

import { html } from "../util.js";
import { S, toast, ro } from "../ui.js";
import { put } from "../api.js";
import { haptic } from "../native.js";
import { navOrder, NAV_ITEMS, BOTTOM_COUNT, reshell } from "../main.js";

let moved = ""; // last moved key, highlighted after the redraw

/** The card's HTML. */
export function menuCard() {
  const order = navOrder();
  const row = (k, i) => html`<li class="menu-row ${k === moved ? "moved" : ""}" data-key="${k}">
    <span class="menu-pos">${i + 1}</span>
    <span class="ic">${NAV_ITEMS[k][2]}</span>
    <span class="grow">${NAV_ITEMS[k][0]}</span>
    ${ro() ? "" : html`<button class="icon ghost" data-up="${k}" aria-label="Move ${NAV_ITEMS[k][0]} up" ${i === 0 ? "disabled" : ""}>▲</button>
      <button class="icon ghost" data-down="${k}" aria-label="Move ${NAV_ITEMS[k][0]} down" ${i === order.length - 1 ? "disabled" : ""}>▼</button>`}
  </li>`;
  return html`<section class="card" data-menu>
    <header><h2>Menu order</h2>${ro() ? "" : html`<button class="ghost" data-reset>Reset</button>`}</header>
    <p class="muted" style="margin-top:0;font-size:.85rem">The first ${BOTTOM_COUNT} are the phone's bottom bar; the rest are under <b>More</b>. Settings always stays last.${ro() ? " (Read-only in the demo.)" : ""}</p>
    <ol class="list menu-list">
      ${order.slice(0, BOTTOM_COUNT).map(row)}
      <li class="menu-split"><span>More ▾</span></li>
      ${order.slice(BOTTOM_COUNT).map((k, i) => row(k, i + BOTTOM_COUNT))}
    </ol>
  </section>`;
}

/** Wires the move buttons. */
export function wireMenuCard(root) {
  const card = root.querySelector("[data-menu]");
  if (!card) return;
  const save = async (order, key) => {
    try {
      await put("/api/settings", { ...S.state.settings, nav_order: order });
      moved = key;
      haptic("tick");
      await reshell(); // menus and this screen redraw in the new order
      setTimeout(() => { moved = ""; }, 600);
    } catch (e) { toast(e.message, true); }
  };
  const move = (key, by) => {
    const order = navOrder();
    const i = order.indexOf(key), j = i + by;
    if (i < 0 || j < 0 || j >= order.length) return;
    [order[i], order[j]] = [order[j], order[i]];
    save(order, key);
  };
  card.querySelectorAll("[data-up]").forEach((b) => b.addEventListener("click", () => move(b.dataset.up, -1)));
  card.querySelectorAll("[data-down]").forEach((b) => b.addEventListener("click", () => move(b.dataset.down, 1)));
  card.querySelector("[data-reset]")?.addEventListener("click", () => save(Object.keys(NAV_ITEMS), ""));
}
