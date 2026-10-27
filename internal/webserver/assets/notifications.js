"use strict";
// Page-bound delivery only: no push subscription, service worker, or background
// service. The server durably claims each automatic attempt before returning it.
const browserNotifications = (() => {
  const preference = "bambu-tui.browser-notifications.v1";
  const element = (id) => document.querySelector("#" + id);
  let optedIn = false, busy = false, permissionPending = false;
  let token = null, cursor = null, lastPoll = 0, generation = 0, message = "";
  try { optedIn = localStorage.getItem(preference) === "on"; } catch {
    message = "Browser storage unavailable; enable is session-only.";
  }
  function capability() {
    if (typeof Notification === "undefined") return "Unsupported in this browser";
    if (!window.isSecureContext) return "Unavailable outside a secure localhost context";
    return "Permission: " + Notification.permission;
  }
  function permitted() {
    return typeof Notification !== "undefined" && window.isSecureContext && Notification.permission === "granted";
  }
  function enabled() { return optedIn && permitted(); }
  function render() {
    element("notification-status").textContent = `${capability()} · Automatic: ${enabled() ? "On" : "Off"}${message ? " · " + message : ""}`;
    element("enable-notifications").disabled = permissionPending || enabled() || typeof Notification === "undefined" || !window.isSecureContext;
    element("test-notifications").disabled = permissionPending || typeof Notification === "undefined" || !window.isSecureContext;
    element("disable-notifications").disabled = !optedIn;
  }
  function save(value) {
    optedIn = value;
    generation++;
    cursor = null;
    try { localStorage.setItem(preference, value ? "on" : "off"); } catch {
      message = "Browser storage unavailable; preference lasts only in this page.";
    }
  }
  async function permission() {
    if (typeof Notification === "undefined" || !window.isSecureContext) { render(); return false; }
    permissionPending = true;
    message = "Waiting for browser permission…";
    render();
    try {
      // This function is called only from an explicit Enable or Test click.
      if (Notification.permission === "default") await Notification.requestPermission();
      if (!permitted()) {
        message = "Not allowed. Change this site's notification permission in browser settings, then retry.";
        return false;
      }
      message = "";
      return true;
    } catch {
      message = "Permission unavailable. Check browser and OS notification settings.";
      return false;
    } finally { permissionPending = false; render(); }
  }
  function submit(event, synthetic = false) {
    const notification = new Notification(event.title, {
      body: event.body, icon: "/icon.png", tag: synthetic ? "bambu-tui-browser-test" : event.id,
      silent: !event.sound,
    });
    message = "Submitted to browser; banner and sound unconfirmed.";
    notification.onerror = () => {
      message = "Browser/OS submission failed or unavailable; automatic attempts are not retried.";
      render();
    };
    notification.onclick = () => {
      window.focus();
      if (!synthetic && typeof snapshot !== "undefined" && snapshot?.printers.some((p) => p.id === event.printer)) {
        // Return to the printer's reported conditions without changing reviews.
        if (element("about-dialog").open) element("about-dialog").close();
        inspect(event.printer);
      }
      notification.close();
    };
    // No onclose decision: closing a banner never hides a farm notice.
  }
  async function poll() {
    render();
    if (!enabled() || busy) return;
    busy = true;
    const started = generation;
    const startedAt = Date.now();
    if (startedAt - lastPoll > 10000) cursor = null;
    try {
      if (!token) {
        const response = await fetch("/api/session", {cache: "no-store", signal: AbortSignal.timeout(5000)});
        if (!response.ok) throw new Error();
        token = (await response.json()).token;
      }
      const response = await fetch("/api/notifications", {
        method: "POST", headers: {"Content-Type": "application/json", "X-CSRF-Token": token},
        body: JSON.stringify({baseline: cursor === null, after: cursor ?? 0}), signal: AbortSignal.timeout(5000),
      });
      if (!response.ok) throw new Error();
      const result = await response.json();
      if (started !== generation) return;
      // Timeout signals pause during document suspension. Claims returned after
      // a long pause are consumed, but must not be displayed on resume.
      if (Date.now() - startedAt > 10000) {
        cursor = null;
        message = "Notification polling paused; resuming establishes a new baseline without backlog.";
        return;
      }
      if (result.error) { message = result.error; cursor = null; return; }
      if (cursor === null) message = "Listening for new events. Existing events form a baseline; suspended or closed pages cannot deliver.";
      cursor = result.cursor;
      lastPoll = Date.now();
      for (const event of result.events) {
        // Disabling/revoking permission while a claim was in flight consumes
        // the attempt, but must not submit it after the local opt-out.
        if (!enabled()) break;
        try { submit(event); } catch {
          message = "Notification constructor unavailable or rejected. Check browser/OS settings; attempt will not be retried.";
        }
      }
    } catch {
      token = null; cursor = null;
      message = "Notification connection unavailable or outcome unknown; reconnect establishes a new baseline without backlog.";
    } finally { busy = false; render(); }
  }
  element("enable-notifications").onclick = async () => {
    const started = generation;
    if (!(await permission()) || started !== generation) return;
    message = "Enabled for new events while this page runs. Existing events form a baseline.";
    save(true); render();
    await poll();
  };
  element("disable-notifications").onclick = () => {
    message = "Disabled in this browser. OS permission is unchanged.";
    save(false); render();
  };
  element("test-notifications").onclick = async () => {
    if (!(await permission())) return;
    try { submit({title: "Bambu TUI browser test", body: "Synthetic notification. Banner and sound are unconfirmed; automatic delivery preferences are unchanged.", sound: false}, true); }
    catch { message = "Notification constructor unavailable or rejected. Check browser and OS settings."; }
    render();
  };
  window.addEventListener("storage", (event) => {
    if (event.key === preference || event.key === null) {
      optedIn = event.newValue === "on"; generation++; cursor = null;
      message = "Browser preference changed in another tab."; render();
    }
  });
  render();
  return {enabled, poll, disconnected() { cursor = null; }};
})();
