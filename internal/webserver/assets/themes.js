"use strict";
// Theme choices affect only this browser. The host publishes resolved TUI RGBs,
// never paths, raw configuration, CSS, or user-file metadata.
(() => {
  const select = document.querySelector("#theme-select");
  const status = document.querySelector("#theme-status");
  // Legacy lookup is only for same-origin upgrade migration.
  const key = "com.suitepotato.bambustand.browser-theme.v1";
  const legacyKey = "bambu-tui.browser-theme.v1";
  const roles = {
    background: "bg", text: "text", secondary_text: "secondary", muted_text: "muted",
    info: "info", accent: "green", success: "success", warning: "amber", error: "red",
    focus: "focus", border: "border", selected_text: "selected-text",
    selected_background: "selected-background", hotkey: "hotkey",
    footer_background: "footer-background", footer_text: "footer-text",
    footer_muted: "footer-muted", footer_info: "footer-info", footer_success: "footer-success",
    footer_warning: "footer-warning", footer_error: "footer-error",
    footer_hotkey: "footer-hotkey", footer_separator: "footer-separator",
  };
  let themes = [], signature = "", loading = false, storageFailed = false;
  let choice = "host";
  try {
    let saved = localStorage.getItem(key);
    if (saved === null) {
      saved = localStorage.getItem(legacyKey);
      if (saved !== null) { localStorage.setItem(key, saved); localStorage.removeItem(legacyKey); }
    }
    choice = saved || "host";
  } catch { storageFailed = true; }
  function apply() {
    const theme = themes.find(t => t.id === choice) || themes.find(t => t.id === "host");
    if (!theme) return;
    choice = theme.id;
    const colors = theme.colors;
    for (const [role, property] of Object.entries(roles)) {
      document.documentElement.style.setProperty(`--${property}`,
        colors[role] || "#101312"); // Browser canvas for inherited terminal background.
    }
    const background = colors.background || "#101312";
    const [r, g, b] = background.slice(1).match(/../g).map(v => parseInt(v, 16));
    document.documentElement.style.colorScheme = (r * 299 + g * 587 + b * 114) / 1000 > 128 ? "light" : "dark";
    select.value = choice;
    status.textContent = `${theme.id === "host" ? "Following the host theme." : `${theme.label} · This browser only.`}${storageFailed ? " Theme preference cannot be saved in this browser." : ""}`;
  }
  function valid(theme) {
    return theme && typeof theme.id === "string" && /^[a-z0-9-]{1,64}$/.test(theme.id) &&
      typeof theme.label === "string" && theme.label.length > 0 && theme.label.length <= 100 &&
      theme.colors && Object.keys(roles).every(role =>
        role === "background" && !theme.colors[role] || /^#[a-f\d]{6}$/i.test(theme.colors[role]));
  }
  async function refresh() {
    if (loading) return;
    loading = true;
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 5000);
    try {
      const response = await fetch("/api/themes", {cache: "no-store", signal: controller.signal});
      if (!response.ok) throw new Error("themes unavailable");
      const catalog = await response.json();
      if (!Array.isArray(catalog.themes) || !catalog.themes.length || catalog.themes.length > 100 ||
          !catalog.themes.every(valid) || !catalog.themes.some(t => t.id === "host") ||
          new Set(catalog.themes.map(t => t.id)).size !== catalog.themes.length)
        throw new Error("invalid themes");
      const next = JSON.stringify(catalog.themes);
      if (signature !== next) {
        themes = catalog.themes;
        const options = themes.map(theme => {
          const option = document.createElement("option");
          option.value = theme.id;
          option.textContent = theme.label;
          return option;
        });
        select.replaceChildren(...options);
        signature = next;
      }
      select.disabled = false;
      apply();
    } catch {
      status.textContent = "Themes unavailable. Keeping the current appearance; reopen Settings to retry.";
    } finally {
      clearTimeout(timeout);
      loading = false;
    }
  }
  select.addEventListener("change", () => {
    choice = select.value;
    apply();
    try {
      localStorage.setItem(key, choice);
      storageFailed = false;
    } catch { storageFailed = true; }
    apply();
  });
  window.addEventListener("storage", event => {
    if (event.key === key || event.key === null) {
      choice = event.newValue || "host";
      apply();
    }
  });
  document.querySelector("#settings").addEventListener("click", refresh);
  document.addEventListener("visibilitychange", () => { if (!document.hidden) refresh(); });
  setInterval(() => { if (!document.hidden) refresh(); }, 30000);
  refresh();
})();
