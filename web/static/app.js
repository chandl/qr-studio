// QR Studio — demo UI. No dependencies; motion lives in CSS where possible.
(function () {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const root = document.documentElement;
  const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  const finePointer = window.matchMedia("(hover: hover) and (pointer: fine)");

  // ======================================================================
  // Theme
  // ======================================================================
  const themeToggle = $("theme-toggle");

  function applyTheme(theme, persist) {
    root.dataset.theme = theme;
    document.querySelector('meta[name="theme-color"]').content = theme === "light" ? "#fafaf7" : "#09090b";
    themeToggle.setAttribute("aria-label", theme === "light" ? "Switch to dark theme" : "Switch to light theme");
    if (persist) {
      try { localStorage.setItem("theme", theme); } catch (e) { /* storage unavailable */ }
    }
    document.dispatchEvent(new CustomEvent("themechange"));
  }
  function toggleTheme() {
    applyTheme(root.dataset.theme === "light" ? "dark" : "light", true);
  }
  themeToggle.addEventListener("click", toggleTheme);
  applyTheme(root.dataset.theme || "dark", false);

  // Follow the OS only while the user hasn't picked a theme themselves.
  window.matchMedia("(prefers-color-scheme: light)").addEventListener("change", (e) => {
    let stored = null;
    try { stored = localStorage.getItem("theme"); } catch (err) { /* ignore */ }
    if (!stored) applyTheme(e.matches ? "light" : "dark", false);
  });

  // ======================================================================
  // Clipboard
  // ======================================================================
  // navigator.clipboard only exists in secure contexts; self-hosted over
  // plain http on a LAN falls back to execCommand.
  function writeClipboard(text) {
    if (navigator.clipboard && window.isSecureContext) {
      return navigator.clipboard.writeText(text);
    }
    return new Promise((resolve, reject) => {
      const ta = document.createElement("textarea");
      ta.value = text;
      ta.setAttribute("readonly", "");
      ta.style.cssText = "position:fixed;top:0;left:0;opacity:0;";
      document.body.appendChild(ta);
      ta.select();
      const ok = document.execCommand("copy");
      ta.remove();
      ok ? resolve() : reject(new Error("copy failed"));
    });
  }

  // ======================================================================
  // Generator (same behavior as before; IDs are unchanged)
  // ======================================================================
  const state = {
    data: "https://example.com",
    format: "png",
    ecl: "M",
    size: 400,
    margin: 2,
    color: "#000000",
    bgcolor: "#ffffff",
    shape: "square",
  };

  const eclHints = {
    L: "Recovers from ~7% damage. Smallest code for a given size.",
    M: "Recovers from ~15% damage. Good default.",
    Q: "Recovers from ~25% damage.",
    H: "Recovers from ~30% damage. Best for damaged or printed codes.",
  };
  const DATA_HINT = "Up to 2,000 characters over GET. Longer payloads switch this preview to POST automatically.";

  // ---- segmented / shape controls ----
  function wireSegmented(containerId, onChange) {
    const el = $(containerId);
    el.addEventListener("click", (e) => {
      const btn = e.target.closest("button[data-value]");
      if (!btn) return;
      [...el.children].forEach((c) => c.setAttribute("aria-pressed", "false"));
      btn.setAttribute("aria-pressed", "true");
      onChange(btn.dataset.value);
    });
  }
  wireSegmented("format-seg", (v) => { state.format = v; onFormatChange(); scheduleRefresh(); });
  wireSegmented("ecl-seg", (v) => { state.ecl = v; $("ecl-hint").textContent = eclHints[v]; scheduleRefresh(); });
  wireSegmented("shape-seg", (v) => { state.shape = v; scheduleRefresh(); });

  function onFormatChange() {
    $("download").download = "qrcode." + state.format;
  }

  // ---- text / textarea ----
  $("data").addEventListener("input", (e) => {
    state.data = e.target.value;
    $("data-hint").textContent = state.data.length > 2000
      ? `${state.data.length.toLocaleString()} characters — switched to POST automatically.`
      : DATA_HINT;
    scheduleRefresh();
  });

  // ---- sliders ----
  function paintRange(input) {
    const min = Number(input.min), max = Number(input.max);
    input.style.setProperty("--pct", ((Number(input.value) - min) / (max - min)) * 100 + "%");
  }
  $("size").addEventListener("input", (e) => {
    state.size = parseInt(e.target.value, 10);
    $("size-value").textContent = state.size + " px";
    paintRange(e.target);
    scheduleRefresh();
  });
  $("margin").addEventListener("input", (e) => {
    state.margin = parseInt(e.target.value, 10);
    $("margin-value").textContent = state.margin + (state.margin === 1 ? " module" : " modules");
    paintRange(e.target);
    scheduleRefresh();
  });
  paintRange($("size"));
  paintRange($("margin"));

  // ---- color sync (picker <-> hex text) ----
  function wireColor(pickerId, hexId, key) {
    const picker = $(pickerId), hex = $(hexId);
    picker.addEventListener("input", () => {
      hex.value = picker.value;
      state[key] = picker.value;
      scheduleRefresh();
    });
    hex.addEventListener("input", () => {
      let v = hex.value.trim();
      if (!v.startsWith("#")) v = "#" + v;
      if (/^#[0-9a-fA-F]{6}$/.test(v)) {
        picker.value = v;
        state[key] = v;
        scheduleRefresh();
      }
    });
  }
  wireColor("color", "color-hex", "color");
  wireColor("bgcolor", "bgcolor-hex", "bgcolor");

  // ---- request building ----
  function buildParams() {
    return {
      data: state.data,
      format: state.format,
      ecl: state.ecl,
      size: state.size,
      margin: state.margin,
      color: state.color,
      bgcolor: state.bgcolor,
      shape: state.shape,
    };
  }

  function buildGetURL() {
    const p = buildParams();
    const usp = new URLSearchParams({ ...p, size: String(p.size), margin: String(p.margin) });
    return "/qr?" + usp.toString();
  }

  function usesPost() {
    return state.data.length > 2000;
  }

  // ---- banners ----
  function showError(msg) {
    $("error-text").textContent = msg;
    $("error").classList.add("show");
    $("warning").classList.remove("show");
  }
  function clearBanners() {
    $("error").classList.remove("show");
    $("warning").classList.remove("show");
  }

  // ---- snippets ----
  const TABS = ["url", "html", "curl"];
  let activeTab = "url";
  function updateSnippets(finalGetUrl) {
    const origin = window.location.origin;
    if (usesPost()) {
      const body = buildParams();
      $("snippet-url").textContent = "Not available: this request uses POST (data exceeds 2,000 characters), so it has no shareable GET URL.";
      $("snippet-html").textContent = "Not available for POST-only requests. Shorten the data to get an embeddable <img> URL.";
      $("snippet-curl").textContent =
`curl -X POST ${origin}/qr \\
  -H "Content-Type: application/json" \\
  -d '${JSON.stringify(body, null, 2).replace(/'/g, "'\\''")}' \\
  -o qrcode.${state.format}`;
    } else {
      const full = origin + finalGetUrl;
      $("snippet-url").textContent = full;
      $("snippet-html").textContent = `<img src="${full}" alt="QR code" width="${state.size}" height="${state.size}">`;
      $("snippet-curl").textContent = `curl "${full}" -o qrcode.${state.format}`;
    }
  }

  const tabList = $("snippet-tabs");
  function selectTab(tab, focus) {
    activeTab = tab;
    [...tabList.children].forEach((c) => {
      const on = c.dataset.tab === tab;
      c.setAttribute("aria-selected", on ? "true" : "false");
      c.tabIndex = on ? 0 : -1;
      if (on && focus) c.focus();
    });
    TABS.forEach((t) => $("snippet-" + t).classList.toggle("show", t === tab));
    $("snippet-cmd").textContent = tab;
  }
  tabList.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-tab]");
    if (btn) selectTab(btn.dataset.tab, false);
  });
  tabList.addEventListener("keydown", (e) => {
    const i = TABS.indexOf(activeTab);
    const next = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: TABS.length - 1 }[e.key];
    if (next === undefined) return;
    e.preventDefault();
    selectTab(TABS[(next + TABS.length) % TABS.length], true);
  });

  function flashCopied(btnEl) {
    const label = btnEl.querySelector(".btn-label") || btnEl;
    if (!btnEl.dataset.label) btnEl.dataset.label = label.textContent;
    label.textContent = "Copied";
    btnEl.classList.add("copied");
    clearTimeout(btnEl._copiedTimer);
    btnEl._copiedTimer = setTimeout(() => {
      label.textContent = btnEl.dataset.label;
      btnEl.classList.remove("copied");
    }, 1200);
  }

  function copyToClipboard(text, btnEl) {
    if (!text) return;
    writeClipboard(text).then(() => flashCopied(btnEl)).catch(() => {});
  }

  $("copy-snippet-btn").addEventListener("click", (e) => {
    copyToClipboard($("snippet-" + activeTab).textContent, e.currentTarget);
  });

  $("copy-link-btn").addEventListener("click", (e) => {
    const url = usesPost() ? "" : window.location.origin + buildGetURL();
    if (!url) { showError("This request needs POST (long data) — no shareable link. See the cURL tab instead."); return; }
    copyToClipboard(url, e.currentTarget);
  });

  // ---- fetch + render ----
  let requestSeq = 0;
  let lastObjectUrl = null;
  const preview = $("preview");
  const spinner = $("spinner");

  async function refresh() {
    if (!state.data) {
      showError("Enter some data to encode.");
      return;
    }
    const seq = ++requestSeq;
    spinner.classList.add("show");
    preview.classList.add("loading");

    let url, fetchOpts;
    if (usesPost()) {
      url = "/qr";
      fetchOpts = {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(buildParams()),
      };
    } else {
      url = buildGetURL();
      fetchOpts = { cache: "no-store" };
    }

    try {
      const res = await fetch(url, fetchOpts);
      if (seq !== requestSeq) return;

      if (!res.ok) {
        const body = await res.json().catch(() => ({ error: "request failed" }));
        showError(body.error || "Request failed.");
        spinner.classList.remove("show");
        preview.classList.remove("loading");
        return;
      }

      clearBanners();
      const warn = res.headers.get("X-QR-Contrast-Warning");
      if (warn) {
        $("warning-text").textContent = warn;
        $("warning").classList.add("show");
      }

      const blob = await res.blob();
      if (seq !== requestSeq) return;
      const objectUrl = URL.createObjectURL(blob);
      preview.onload = () => {
        if (lastObjectUrl) URL.revokeObjectURL(lastObjectUrl);
        lastObjectUrl = objectUrl;
      };
      preview.src = objectUrl;

      $("download").href = usesPost() ? objectUrl : url;
      updateSnippets(url);
    } catch (e) {
      if (seq !== requestSeq) return;
      showError("Could not reach the server: " + e.message);
    } finally {
      if (seq === requestSeq) {
        spinner.classList.remove("show");
        preview.classList.remove("loading");
      }
    }
  }

  let debounceTimer = null;
  function scheduleRefresh() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(refresh, 180);
  }

  refresh();

  // ======================================================================
  // Status pill (/healthz)
  // ======================================================================
  (async function pingHealth() {
    const pill = $("status-pill");
    let live = false;
    try {
      const res = await fetch("/healthz", { cache: "no-store" });
      live = res.ok;
    } catch (e) { /* offline */ }
    pill.dataset.state = live ? "live" : "down";
    pill.querySelector(".status-text").textContent = live ? "live" : "down";
    pill.setAttribute("aria-label", "Service status: " + (live ? "live" : "down"));
  })();

  // ======================================================================
  // Nav: frosted once scrolled, scroll-spy
  // ======================================================================
  const nav = $("nav");
  new IntersectionObserver(([entry]) => {
    nav.classList.toggle("scrolled", !entry.isIntersecting);
  }).observe($("nav-sentinel"));

  const spyLinks = [...document.querySelectorAll("[data-spy]")];
  const spySections = spyLinks.map((a) => $(a.dataset.spy));
  let spyQueued = false;
  function updateSpy() {
    spyQueued = false;
    const line = window.innerHeight * 0.4;
    let current = null;
    for (const s of spySections) {
      if (s.getBoundingClientRect().top <= line) current = s.id;
    }
    // At the very bottom, the last section wins even if it's short.
    if (window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2) {
      current = spySections[spySections.length - 1].id;
    }
    spyLinks.forEach((a) => {
      if (a.dataset.spy === current) a.setAttribute("aria-current", "true");
      else a.removeAttribute("aria-current");
    });
  }
  window.addEventListener("scroll", () => {
    if (!spyQueued) { spyQueued = true; requestAnimationFrame(updateSpy); }
  }, { passive: true });
  window.addEventListener("resize", updateSpy, { passive: true });
  updateSpy();

  // ======================================================================
  // Cards: cursor spotlight, border glow, subtle tilt (fine pointers only)
  // ======================================================================
  (function cardEffects() {
    const MAX_TILT = 6; // degrees
    let active = null;
    let pending = null;

    function reset(card) {
      card.style.setProperty("--rx", "0deg");
      card.style.setProperty("--ry", "0deg");
    }
    function apply() {
      const e = pending;
      pending = null;
      const card = e.target instanceof Element ? e.target.closest(".card") : null;
      if (active && active !== card) reset(active);
      active = card;
      if (!card) return;
      const r = card.getBoundingClientRect();
      const x = e.clientX - r.left, y = e.clientY - r.top;
      card.style.setProperty("--mx", x + "px");
      card.style.setProperty("--my", y + "px");
      if (card.classList.contains("tilt") && !reducedMotion.matches) {
        card.style.setProperty("--rx", ((0.5 - y / r.height) * 2 * MAX_TILT).toFixed(2) + "deg");
        card.style.setProperty("--ry", ((x / r.width - 0.5) * 2 * MAX_TILT).toFixed(2) + "deg");
      }
    }
    document.addEventListener("pointermove", (e) => {
      if (e.pointerType !== "mouse" || !finePointer.matches) return;
      if (!pending) requestAnimationFrame(apply);
      pending = e;
    }, { passive: true });
    document.documentElement.addEventListener("pointerleave", () => {
      if (active) reset(active);
      active = null;
    });
  })();
})();
