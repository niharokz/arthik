package com.nihars.arthik

import android.text.Html

/** Built-in fallback page for when the engine itself can't run (styled like the app). */
object Pages {
    /** Error page with Retry / Demo buttons wired to the bridge. */
    fun error(message: String, log: String): String = """
        <!doctype html><html><head><meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <style>
          :root { color-scheme: light dark; --bg:#f6f8f7; --card:#fff; --text:#13201e; --muted:#5a6866; --teal:#3a807a; --line:#dbe3e1; }
          @media (prefers-color-scheme: dark) { :root { --bg:#0f1413; --card:#171d1c; --text:#e8efed; --muted:#a3b1ae; --teal:#5fb3aa; --line:#2b3533; } }
          body { margin:0; min-height:100vh; display:grid; place-items:center; padding:20px; box-sizing:border-box;
                 font:15px/1.45 system-ui, sans-serif; background:var(--bg); color:var(--text); }
          .card { background:var(--card); border:1px solid var(--line); border-radius:14px; padding:20px; max-width:420px; width:100%; }
          h1 { font-size:1.2rem; margin:0 0 8px; } p { color:var(--muted); }
          button { font:inherit; width:100%; padding:11px; border-radius:10px; margin-top:10px; border:1px solid var(--line); background:var(--card); color:var(--text); }
          button.primary { background:var(--teal); border-color:var(--teal); color:#fff; }
          pre { font:12px/1.4 monospace; white-space:pre-wrap; max-height:40vh; overflow:auto; background:var(--bg); padding:10px; border-radius:8px; }
        </style></head><body><div class="card">
          <h1>Arthik couldn't start</h1>
          <p>${esc(message)}</p>
          <button class="primary" onclick="ArthikNative.restartEngine()">Try again</button>
          <button onclick="ArthikNative.useDemo()">Open the demo</button>
          <details><summary>Engine log</summary><pre>${esc(log)}</pre></details>
        </div></body></html>
    """.trimIndent()

    private fun esc(s: String): String = Html.escapeHtml(s)
}
