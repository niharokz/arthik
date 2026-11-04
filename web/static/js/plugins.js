// Personal plugins: optional screens and shortcuts that live outside the core app.
// The server lists the enabled ones in state.plugins; each has a module in
// js/plugins/<id>.js that registers itself here when loaded.

import { S } from "./ui.js";

const registry = new Map(); // id → { title, icon, route, render, formHook }

/** Called by a plugin module to add itself. */
export function register(id, plugin) { registry.set(id, plugin); }

/** Loads the modules of the plugins the server enabled (missing ones are skipped). */
export async function loadPlugins() {
  for (const p of S.state?.plugins || []) {
    if (registry.has(p.id)) continue;
    try { await import(`./plugins/${p.id}.js`); } catch (e) { console.warn("plugin", p.id, e); }
  }
}

/** Enabled plugins that have a screen. */
export const pluginScreens = () => (S.state?.plugins || []).map((p) => ({ ...p, ...registry.get(p.id) })).filter((p) => p.render);

/** The screen for a route id, if a plugin provides it. */
export const pluginScreen = (id) => pluginScreens().find((p) => p.id === id);

/** Shortcuts plugins add to the transaction form: { show(st), label(st), apply(st) }. */
export const formHooks = () => [...registry.values()].map((p) => p.formHook).filter(Boolean);
