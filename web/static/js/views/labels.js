// Labels: free tags on transactions (trip, reimbursable, home …): list with usage,
// add / rename / recolour / delete (removed from every transaction).

import { html, raw } from "../util.js";
import { S, sheet, toast, confirmSheet, formData, ro } from "../ui.js";
import { post, put, del } from "../api.js";

export async function render(root, ctx) {
  const labels = [...S.state.labels].sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));
  ctx.actions(ro() ? "" : html`<button class="primary" data-new>Add</button>`);
  root.innerHTML = String(html`<div class="card">${labels.length ? html`<ul class="list">${labels.map((l) => html`<li><div class="item" style="cursor:default">
      <span class="ic" style="${l.color ? `background:${l.color}33` : ""}">#</span>
      <a class="main-t" href="#/tx?label=${encodeURIComponent(l.id)}"><span class="t">${l.name}</span><span class="s">${l.count} transaction${l.count === 1 ? "" : "s"}</span></a>
      ${ro() ? "" : html`<button class="ghost" data-edit="${l.id}">Edit</button>`}</div></li>`)}</ul>`
    : html`<div class="empty">No labels yet. Add one here or type a new label on any transaction.</div>`}</div>
    <p class="muted" style="font-size:.85rem">Tap a label to see its transactions; the <a href="#/reports/labels">label report</a> totals them by period.</p>`);
  ctx.actionsEl.querySelector("[data-new]")?.addEventListener("click", () => form(null, ctx));
  root.querySelectorAll("[data-edit]").forEach((b) => b.addEventListener("click", () => form(S.state.labels.find((l) => l.id === b.dataset.edit), ctx)));
}

function form(l, ctx) {
  const isNew = !l;
  l = l || {};
  sheet(isNew ? "Add label" : "Edit label", html`
    <label class="f"><span>Name</span><input name="name" value="${l.name || ""}" autofocus></label>
    <label class="f"><span>Colour</span><input type="color" name="color" value="${l.color || "#3a807a"}" style="height:40px;padding:4px"></label>`,
    html`${isNew ? "" : html`<button class="danger" data-del>Delete</button>`}<span class="grow"></span><button data-close>Cancel</button><button class="primary" data-save>Save</button>`,
    (d, close) => {
      d.querySelector("[data-save]").addEventListener("click", async () => {
        const v = formData(d);
        try {
          if (isNew) await post("/api/labels", v); else await put("/api/labels/" + encodeURIComponent(l.id), v);
          close(); toast("Saved"); ctx.refresh();
        } catch (e) { toast(e.message, true); }
      });
      d.querySelector("[data-del]")?.addEventListener("click", async () => {
        if (!(await confirmSheet("Delete label?", `“${l.name}” will be removed from ${l.count} transaction(s). The transactions stay.`))) return;
        try { await del("/api/labels/" + encodeURIComponent(l.id)); close(); toast("Deleted"); ctx.refresh(); }
        catch (e) { toast(e.message, true); }
      });
    });
}

export { raw };
