// Settings → Phone (only inside the Android app): data folder or demo, file access,
// main-device switch, app lock (PIN or fingerprint/face), the nightly bank-SMS copy
// and the engine log. Settings are stored by the app itself.

import { html, raw, fmtShort } from "../util.js";
import { sheet, toast, confirmSheet } from "../ui.js";
import { call, config, onNative } from "../native.js";
import { pickFolder } from "../appmode.js";

const LOCK_AFTER = [[0, "Immediately"], [60, "After 1 minute"], [300, "After 5 minutes"], [900, "After 15 minutes"], [1800, "After 30 minutes"], [3600, "After 1 hour"]];
const LOCKS = { pin: "Arthik PIN", device: "Fingerprint / face (phone screen lock as backup)", none: "Off" };

const pad = (n) => String(n).padStart(2, "0");
const stamp = (ms) => { if (!ms) return "never"; const d = new Date(ms); return `${fmtShort(d.toISOString().slice(0, 10))} ${pad(d.getHours())}:${pad(d.getMinutes())}`; };

// The nightly SMS copy: permission, switch, time, look-back, last result.
function smsBlock(c) {
  const m = c.sms || {};
  const folder = c.mode === "folder" && c.folder_ok;
  return html`<div class="psec"><b>📩 Bank SMS → Inbox</b>
    <div class="muted" style="font-size:.85rem">Every night new bank and card SMS are copied to <code>inbox/sms_${m.device || "phone"}.md</code> in your folder; Arthik on nimory turns them into drafts to review. Only messages after the last run are read — never older ones.</div></div>
    <div class="prow">
      <div class="grow"><b>SMS permission</b>${m.restricted_settings && !m.permission ? html`<div class="muted">If Android blocks it: App info → ⋮ → <b>Allow restricted settings</b>, then allow SMS.</div>` : ""}</div>
      ${m.permission ? html`<span class="badge">✓ Allowed</span>` : html`<button class="primary" data-smsperm>Allow</button>`}
    </div>
    <label class="prow">
      <div class="grow"><b>Copy new bank SMS every night</b>${!folder ? html`<div class="neg">Choose your finance folder first.</div>` : ""}</div>
      <input type="checkbox" class="switch" data-sms ${m.enabled ? raw("checked") : ""} ${!m.permission || !folder ? raw("disabled") : ""}>
    </label>
    ${m.enabled ? html`
      <div class="prow"><div class="grow"><b>Time</b>${!m.exact && m.exact_settings ? html`<div class="muted">Android may run it up to 15 minutes late. <a href="#" data-exact>Allow exact time</a></div>` : ""}</div>
        <input type="time" data-smstime value="${pad(m.hour)}:${pad(m.minute)}" style="width:auto"></div>
      <div class="prow"><div class="grow"><b>After a missed night, look back</b></div>
        <select data-lookback style="width:auto">${[1, 2, 3, 4, 5, 6, 7].map((n) => html`<option value="${n}" ${n === m.lookback ? raw("selected") : ""}>${n} day${n > 1 ? "s" : ""}</option>`)}</select></div>
      <div class="prow"><div class="grow"><b>Last run</b><div class="muted">${stamp(m.last_run)}${m.last_result ? " · " + m.last_result : ""}${m.next_run ? html`<br>Next: ${stamp(m.next_run)}` : ""}</div></div>
        <button data-smsnow>Run now</button></div>` : ""}`;
}

// Android 13+ refused without asking: explain "restricted settings".
function blockedSheet() {
  sheet("Allow SMS for Arthik", html`
    <p style="margin-top:0">Android blocks SMS access for apps installed outside the Play Store until you allow it once:</p>
    <ol><li>Tap <b>Open app info</b> below.</li><li>Tap <b>⋮</b> (top right) → <b>Allow restricted settings</b> and confirm.</li>
      <li>Open <b>Permissions → SMS → Allow</b>.</li><li>Come back here and switch the nightly copy on.</li></ol>`,
  html`<span class="grow"></span><button data-close>Later</button><button class="primary" data-info>Open app info</button>`, (d, close) => {
    d.querySelector("[data-info]").addEventListener("click", () => { close(); call("openAppSettings"); });
  });
}

/** The card's HTML (reads the app's current settings). */
export function phoneCard() {
  const c = config();
  const demo = c.mode === "demo";
  return html`<section class="card phone-card" data-phone>
    <header><h2>📱 Phone</h2><small class="faint">${c.device || ""}</small></header>
    <div class="prow">
      <div class="grow"><b>${demo ? "Demo data (read-only)" : "My finance folder"}</b>
        <div class="muted ellipsis" title="${c.folder || ""}">${c.folder ? c.folder : "No folder chosen yet"}</div>
        ${!demo && c.folder && !c.folder_ok ? html`<div class="neg">This folder can't be read or written — check file access.</div>` : ""}</div>
      <div class="row wrap">
        <button data-pick>${c.folder ? "Change folder" : "Choose folder"}</button>
        ${demo ? (c.folder ? html`<button class="primary" data-mine>Use my folder</button>` : "") : html`<button data-demo>View demo</button>`}
      </div>
    </div>
    <div class="prow">
      <div class="grow"><b>File access</b><div class="muted">Needed to read and write the Syncthing folder.</div></div>
      ${c.files_access ? html`<span class="badge">✓ Allowed</span>` : html`<button class="primary" data-access>Allow</button>`}
    </div>
    <label class="prow">
      <div class="grow"><b>This phone is the main device</b><div class="muted">Leave off when nimory runs Arthik: nimory then writes balances, merges sync conflicts and imports the inbox (SMS and Kronos drafts). Turn on only if the phone is used without the server.</div></div>
      <input type="checkbox" class="switch" data-primary ${c.primary ? raw("checked") : ""}>
    </label>
    <div class="prow">
      <div class="grow"><b>App lock</b><div class="muted">${c.lock === "pin" ? `PIN (${c.pin_length} digits)${c.biometric_on ? " + fingerprint" : ""}` : LOCKS[c.lock] || "Off"}</div></div>
      <div class="row wrap">
        ${c.lock === "pin" ? html`<button data-pin>Change PIN</button>` : html`<button class="${c.lock === "none" ? "primary" : ""}" data-pin>${c.lock === "device" ? "Use a PIN instead" : "Set a PIN"}</button>`}
        ${c.lock !== "device" && c.device_lock_available ? html`<button data-devlock>Fingerprint / face</button>` : ""}
        ${c.lock !== "none" ? html`<button data-nolock>Turn off</button>` : ""}
      </div>
    </div>
    ${c.lock === "pin" && c.biometric_available ? html`<label class="prow"><div class="grow"><b>Also unlock with fingerprint</b></div><input type="checkbox" class="switch" data-bio ${c.biometric_on ? raw("checked") : ""}></label>` : ""}
    ${c.lock !== "none" ? html`
      <div class="prow"><div class="grow"><b>Lock again when away</b><div class="muted">Switching to another app keeps Arthik open for this long.</div></div>
        <select data-after style="width:auto">${LOCK_AFTER.map(([v, l]) => html`<option value="${v}" ${+c.lock_after === v ? raw("selected") : ""}>${l}</option>`)}</select></div>
      <div class="prow"><div class="grow"></div><button data-locknow>Lock now</button></div>` : ""}
    ${smsBlock(c)}
    <div class="prow">
      <div class="grow"><b>Engine</b><div class="muted">Arthik ${c.version || ""} runs on this phone; nothing leaves it except through Syncthing.</div></div>
      <div class="row wrap"><button data-log>View log</button><button data-restart>Restart</button></div>
    </div>
  </section>`;
}

/** Wires the card's buttons (re-renders the settings view after changes). */
export function wirePhoneCard(root, ctx) {
  const card = root.querySelector("[data-phone]");
  if (!card) return;
  const $ = (sel) => card.querySelector(sel);
  const redraw = () => ctx.refresh();
  const off = onNative((e) => {
    if (e.type === "access" || e.type === "lock") redraw();
    if (e.type === "sms_permission") { if (e.blocked) blockedSheet(); redraw(); }
    if (e.type === "sms") { toast(e.result); redraw(); }
  });
  window.addEventListener("hashchange", off, { once: true });

  $("[data-pick]")?.addEventListener("click", () => pickFolder());
  $("[data-demo]")?.addEventListener("click", () => call("useDemo"));
  $("[data-mine]")?.addEventListener("click", () => call("useFolder"));
  $("[data-access]")?.addEventListener("click", () => call("requestFilesAccess"));
  $("[data-primary]")?.addEventListener("change", async (e) => {
    const on = e.target.checked;
    const ok = await confirmSheet(on ? "Make this phone the main device?" : "Stop being the main device?",
      on ? "Only one device should be main. If nimory also runs Arthik on this folder, keep this off." : "nimory (or another main device) will write balances and import the inbox.",
      "Restart engine", false);
    if (!ok) { e.target.checked = !on; return; }
    call("setPrimary", on);
  });
  $("[data-pin]")?.addEventListener("click", () => pinSheet(redraw));
  $("[data-nolock]")?.addEventListener("click", async () => {
    if (await confirmSheet("Turn off the app lock?", "Anyone holding your unlocked phone can open Arthik.", "Turn off")) { call("clearLock"); redraw(); }
  });
  $("[data-bio]")?.addEventListener("change", (e) => {
    if (!call("setBiometric", e.target.checked)) { e.target.checked = false; toast("Fingerprint unlock isn't available", true); }
  });
  $("[data-after]")?.addEventListener("change", (e) => call("setLockAfter", +e.target.value));
  $("[data-devlock]")?.addEventListener("click", () => {
    if (call("setDeviceLock")) { toast("Arthik now opens with your fingerprint, face or screen lock"); redraw(); }
    else toast("Set a screen lock on the phone first", true);
  });
  $("[data-smsperm]")?.addEventListener("click", () => call("requestSms"));
  $("[data-sms]")?.addEventListener("change", (e) => {
    if (!call("setSms", e.target.checked)) { e.target.checked = false; toast("Allow SMS first", true); return; }
    toast(e.target.checked ? "New bank SMS will be copied every night" : "SMS copy off");
    redraw();
  });
  $("[data-smstime]")?.addEventListener("change", (e) => { const [h, m] = e.target.value.split(":").map(Number); call("setSmsTime", h, m); redraw(); });
  $("[data-lookback]")?.addEventListener("change", (e) => call("setSmsLookback", +e.target.value));
  $("[data-smsnow]")?.addEventListener("click", (e) => { e.target.disabled = true; call("smsRunNow"); });
  $("[data-exact]")?.addEventListener("click", (e) => { e.preventDefault(); call("openExactAlarms"); });
  $("[data-locknow]")?.addEventListener("click", () => call("lockNow"));
  $("[data-restart]")?.addEventListener("click", () => call("restartEngine"));
  $("[data-log]")?.addEventListener("click", () => {
    sheet("Engine log", html`<pre class="log">${call("log") || "(empty)"}</pre>`, html`<span class="grow"></span><button data-close>Close</button>`,
      (d) => { const pre = d.querySelector(".log"); pre.scrollTop = pre.scrollHeight; });
  });
}

// Asks for a new PIN twice, then stores it natively (hashed).
function pinSheet(done) {
  sheet("Set a PIN", html`
    <p class="muted" style="margin-top:0">4 to 8 digits. You'll enter it when you open Arthik.</p>
    <label class="f"><span>New PIN</span><input name="a" type="password" inputmode="numeric" pattern="[0-9]*" maxlength="8" autocomplete="off" autofocus></label>
    <label class="f"><span>Repeat PIN</span><input name="b" type="password" inputmode="numeric" pattern="[0-9]*" maxlength="8" autocomplete="off"></label>`,
  html`<span class="grow"></span><button data-close>Cancel</button><button class="primary" data-ok>Save PIN</button>`, (d, close) => {
    d.querySelector("[data-ok]").addEventListener("click", () => {
      const a = d.querySelector("[name=a]").value, b = d.querySelector("[name=b]").value;
      if (!/^\d{4,8}$/.test(a)) return toast("Use 4 to 8 digits", true);
      if (a !== b) return toast("The PINs don't match", true);
      if (!call("setPin", a)) return toast("Couldn't save the PIN", true);
      close();
      toast("PIN saved");
      done();
    });
  });
}
