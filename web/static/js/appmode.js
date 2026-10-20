// Screens shown only inside the Android app before a session opens: the first-run
// welcome (demo, your own folder or restoring saved settings) and the app lock.
// The native side owns the secrets; this file only draws the screens and calls it.

import { html } from "./util.js";
import { sheet, toast, confirmSheet } from "./ui.js";
import { isApp, call, config, json, onNative, haptic, applyAccent, syncBars } from "./native.js";

/** Decides what to show before the app is entered. */
export function appGate(root) {
  if (!isApp) return notInApp(root);
  applyAccent();
  requestAnimationFrame(syncBars);
  const c = config();
  if (c.engine_error) return engineError(root, c);
  if (!c.mode) return welcome(root, c);
  if (c.locked) return lockScreen(root, c);
  call("enter");
}

// A local-mode server opened in a normal browser.
function notInApp(root) {
  root.innerHTML = String(html`<div class="center"><div class="card login appear">
    <div class="brand"><img src="/icon.svg" alt=""><div><h1>Arthik</h1><small>phone engine</small></div></div>
    <p>This is the engine of the Arthik Android app. Open it from the app.</p></div></div>`);
}

// The engine could not open the chosen folder.
function engineError(root, c) {
  root.innerHTML = String(html`<div class="center"><div class="card login appear">
    <div class="brand"><img src="/icon.svg" alt=""><div><h1>Arthik</h1><small>couldn't open your folder</small></div></div>
    <p class="neg">${c.engine_error}</p>
    <div class="stack"><button class="primary block" data-pick>Choose another folder</button>
      <button class="block" data-demo>Open the demo</button></div></div></div>`);
  root.querySelector("[data-pick]").addEventListener("click", () => pickFolder(root));
  root.querySelector("[data-demo]").addEventListener("click", () => call("useDemo"));
}

// First run: explain the two ways to start.
function welcome(root, c) {
  root.innerHTML = String(html`<div class="center"><div class="welcome appear">
    <div class="brand big"><img src="/icon.svg" alt=""><div><h1>Arthik</h1><small>double-entry personal finance</small></div></div>
    <p class="muted">Every rupee debited somewhere and credited somewhere else — on your phone, offline, in plain files you own.</p>
    <button class="choice card" data-demo>
      <span class="ic">👀</span><span><b>Explore the demo</b><br><small>Six months of made-up data, read-only. Switch to your own folder any time in Settings.</small></span></button>
    <button class="choice card" data-pick>
      <span class="ic">📁</span><span><b>Use my finance folder</b><br><small>Pick the folder Syncthing keeps in sync with your server. Arthik reads and writes it directly.</small></span></button>
    <button class="choice card" data-restore>
      <span class="ic">♻️</span><span><b>Restore my settings</b><br><small>Pick the folder that has <code>arthik-phone.yaml</code> from your old phone or install.</small></span></button>
    <p class="faint" style="text-align:center;font-size:.8rem">Arthik ${c.version || ""}</p>
  </div></div>`);
  root.querySelector("[data-demo]").addEventListener("click", () => call("useDemo"));
  root.querySelector("[data-pick]").addEventListener("click", () => pickFolder(root));
  root.querySelector("[data-restore]").addEventListener("click", () => chooseSettingsFolder(true));
}

/**
 * Picks the folder for the settings file (arthik-phone.yaml). A file already there
 * is loaded (restore = true, or after asking); otherwise this phone's settings are
 * saved there. done() runs when nothing needs a restart.
 */
export function chooseSettingsFolder(restore = false, done = () => {}) {
  const off = onNative((e) => { if (e.type === "config_dir") { off(); settingsChosen(e, restore, done); } });
  call("chooseConfigDir");
}

async function settingsChosen(r, restore, done) {
  if (!r.ok) return settingsHelp(r.error || "That folder can't be used", restore, done);
  if (!r.exists) {
    toast(restore ? `No arthik-phone.yaml in ${r.path} — this phone's settings are saved there from now on` : "Settings are saved to " + r.path, restore);
    return done();
  }
  if (restore || await confirmSheet("Load the saved settings?", `${r.path} already has arthik-phone.yaml. Load it (this phone's settings are replaced; the PIN is never in the file), or keep this phone's settings and overwrite the file.`, "Load file", false)) {
    const err = call("loadConfig");
    return err ? toast(err, true) : toast("Settings restored — reopening Arthik");
  }
  call("saveConfig");
  toast("File overwritten with this phone's settings");
  done();
}

// Picker failed: type the folder instead.
function settingsHelp(error, restore, done) {
  sheet("Folder for the settings file", html`
    <p class="neg">${error}</p>
    <p class="muted">Type the full path, for example <code>/storage/emulated/0/Notes/.arthik</code>.</p>
    <label class="f"><span>Folder path</span><input name="p" autocapitalize="off" autocomplete="off" spellcheck="false"></label>`,
  html`<span class="grow"></span><button data-close>Cancel</button><button class="primary" data-ok>Use folder</button>`, (d, close) => {
    d.querySelector("[data-ok]").addEventListener("click", () => {
      const r = json("setConfigDir", d.querySelector("[name=p]").value.trim());
      if (!r.ok) return toast(r.error || "That folder can't be used", true);
      close();
      settingsChosen(r, restore, done);
    });
  });
}

/** Runs the native folder picker; the app restarts the engine when it succeeds. */
export function pickFolder() {
  const off = onNative((e) => {
    if (e.type !== "folder") return;
    off();
    if (!e.ok && e.error) folderHelp(e.error);
  });
  call("chooseFolder");
}

// Picker failed (or the user prefers typing): offer a manual path.
function folderHelp(error) {
  sheet("Choose your finance folder", html`
    <p class="neg">${error}</p>
    <p class="muted">You can also type the full path, for example <code>/storage/emulated/0/Notes/data/finance</code>.</p>
    <label class="f"><span>Folder path</span><input name="p" autocapitalize="off" autocomplete="off" spellcheck="false"></label>`,
  html`<button data-access>File access…</button><span class="grow"></span><button data-close>Cancel</button><button class="primary" data-ok>Use folder</button>`, (d, close) => {
    d.querySelector("[data-access]").addEventListener("click", () => call("requestFilesAccess"));
    d.querySelector("[data-ok]").addEventListener("click", () => {
      const r = json("setFolderPath", d.querySelector("[name=p]").value.trim());
      if (r.ok) close(); else toast(r.error || "That folder can't be used", true);
    });
  });
}

// Device lock: the system prompt (fingerprint, face or the phone's screen lock).
function deviceLockScreen(root) {
  root.innerHTML = String(html`<div class="center"><div class="lock appear">
    <img class="lock-logo" src="/icon.svg" alt="">
    <h1>Unlock Arthik</h1>
    <p class="muted">Fingerprint, face or your phone's screen lock</p>
    <button class="primary" data-unlock>Unlock</button>
  </div></div>`);
  root.querySelector("[data-unlock]").addEventListener("click", () => call("unlockDevice"));
  setTimeout(() => call("unlockDevice"), 250);
}

// PIN pad + fingerprint button; wrong PIN shakes.
function lockScreen(root, c) {
  if (c.lock === "device") return deviceLockScreen(root);
  let pin = "";
  const len = c.pin_length || 4;
  root.innerHTML = String(html`<div class="center"><div class="lock appear">
    <img class="lock-logo" src="/icon.svg" alt="">
    <h1>Unlock Arthik</h1>
    <div class="dots" data-dots>${Array.from({ length: len }, () => html`<i></i>`)}</div>
    <div class="keypad">${["1", "2", "3", "4", "5", "6", "7", "8", "9", c.biometric_on ? "bio" : "", "0", "del"].map((k) =>
      k === "" ? html`<span></span>` : html`<button class="key" data-k="${k}" aria-label="${k === "bio" ? "Use fingerprint" : k === "del" ? "Delete" : k}">${k === "bio" ? "☝" : k === "del" ? "⌫" : k}</button>`)}</div>
  </div></div>`);
  const dots = root.querySelector("[data-dots]");
  const paint = () => dots.querySelectorAll("i").forEach((d, i) => d.classList.toggle("on", i < pin.length));
  const tryPin = () => {
    if (pin.length < len) return;
    if (call("unlock", pin)) { haptic("confirm"); return; } // native opens the app
    haptic("error");
    dots.classList.remove("shake"); void dots.offsetWidth; dots.classList.add("shake");
    pin = "";
    setTimeout(paint, 250);
  };
  const press = (k) => {
    if (k === "bio") return call("unlockBiometric");
    if (k === "del") pin = pin.slice(0, -1);
    else if (pin.length < len) pin += k;
    haptic("tick");
    paint();
    tryPin();
  };
  root.querySelectorAll("[data-k]").forEach((b) => b.addEventListener("click", () => press(b.dataset.k)));
  document.onkeydown = (e) => { if (/^\d$/.test(e.key)) press(e.key); else if (e.key === "Backspace") press("del"); };
  if (c.biometric_on) setTimeout(() => call("unlockBiometric"), 250); // offer fingerprint straight away
}
