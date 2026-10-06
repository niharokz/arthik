// arthik service worker — makes the app work offline.
//
//  - App shell (HTML, CSS, JS, icons): network first so a new release shows up
//    at once; the cached copy is used when offline, so the app still opens.
//  - GET /api/*: network first; the last good answer is cached and used when
//    offline, so every screen still shows your data.
//  - Writes are never cached here: the page queues offline transaction changes
//    itself (js/api.js) and replays them when back online.

const VERSION = "arthik-v1";
const SHELL = `${VERSION}-shell`;
const API = `${VERSION}-api`;
const FILES = [
  "/app", "/css/app.css", "/manifest.webmanifest", "/icon.svg", "/icon-192.png", "/icon-512.png",
  "/js/main.js", "/js/util.js", "/js/api.js", "/js/ui.js", "/js/charts.js", "/js/txform.js", "/js/txlist.js",
  "/js/views/dashboard.js", "/js/views/transactions.js", "/js/views/accounts.js", "/js/views/categories.js",
  "/js/views/budgets.js", "/js/views/scheduled.js", "/js/views/reports.js", "/js/views/calendar.js",
  "/js/views/labels.js", "/js/views/settings.js",
];

self.addEventListener("install", (e) => {
  e.waitUntil(caches.open(SHELL).then((c) => c.addAll(FILES)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (e) => {
  e.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => !k.startsWith(VERSION)).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (e) => {
  const req = e.request;
  const url = new URL(req.url);
  if (req.method !== "GET" || url.origin !== location.origin) return;
  if (url.pathname.startsWith("/api/")) {
    if (url.pathname === "/api/export.csv") return; // downloads always go to the network
    e.respondWith(networkFirst(req));
    return;
  }
  if (url.pathname === "/" || url.pathname === "/sw.js" || url.pathname === "/healthz") return;
  const shellReq = req.mode === "navigate" && url.pathname.startsWith("/app") ? new Request("/app") : req;
  e.respondWith(networkFirst(shellReq, SHELL));
});

async function networkFirst(req, name = API) {
  const cache = await caches.open(name);
  try {
    const res = await fetch(req);
    if (res.ok) cache.put(req, res.clone());
    return res;
  } catch (err) {
    const hit = await cache.match(req);
    if (hit) return hit;
    return new Response(JSON.stringify({ error: "You are offline and this page has not been opened before." }), {
      status: 503, headers: { "Content-Type": "application/json" },
    });
  }
}
