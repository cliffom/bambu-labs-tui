"use strict";
const $ = (selector) => document.querySelector(selector);
const escapeHTML = (value) =>
  String(value ?? "").replace(
    /[&<>"']/g,
    (char) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        char
      ],
  );
const unknown = (value) =>
  value === null || value === undefined ? "Unknown" : escapeHTML(value);
const labels = {
  RUNNING: "Printing",
  PAUSE: "Paused",
  FAILED: "Failed",
  FINISH: "Finished",
  IDLE: "Idle",
  UNKNOWN: "Unknown",
  PREPARE: "Preparing",
  SLICING: "Slicing",
};
const stateLabel = (state) => labels[state] || state || "Unknown";
let commandPending = null, commandMessage = "";
let showHiddenMessages = false;
let csrfToken = null;
let snapshot = null,
  filter = "all",
  query = "",
  sort = "name",
  selected = null,
  feedError = false,
  timer,
  controller;
const feedDelayed = () =>
  snapshot && Date.now() - Date.parse(snapshot.at) > 10000;
const feedUnavailable = () => feedError || feedDelayed();
const current = (p) => p.fresh && !feedUnavailable();
const idleFinished = (p) => p.idle_finished === true && p.online === true && current(p) && snapshot?.connected === true;
const prefix = (p) => (current(p) ? "" : "Last-known · ");
const status = (p) =>
  feedUnavailable()
    ? "Monitor unavailable"
    : p.connection === "Live"
      ? stateLabel(p.state)
      : p.connection;
const temperature = (t) =>
  t?.current == null
    ? "Unknown"
    : `${escapeHTML(t.current)}${t.target == null ? "" : " / " + escapeHTML(t.target)}°C`;
const duration = (n) =>
  n == null
    ? "Unknown"
    : n >= 60
      ? `${Math.floor(n / 60)}h ${n % 60}m`
      : `${n}m`;
const localTime = (value) =>
  value ? new Date(value).toLocaleString() : "Unavailable";
function remaining(p) {
  if (p.state !== "RUNNING") return "—";
  if (p.remaining == null) return "Unknown";
  return (
    (current(p) && p.eta_current ? "" : "Last-known: ") + duration(p.remaining)
  );
}
function swatch(color) {
  const match = /^(?:#)?([a-f\d]{6})(?:[a-f\d]{2})?$/i.exec(color || "");
  return match
    ? `<span class="swatch" style="background:#${match[1]}" title="#${match[1]}"></span>`
    : '<span class="swatch" title="Unknown color" style="border:1px solid var(--muted)"></span>';
}
function allSlots(p) {
  return [...p.ams.flatMap((unit) => unit.slots), ...p.external];
}
// Unknown states remain outside idle/done for ordering only. Neither freshness
// nor the cloud connection label establishes device connectivity or activity.
function printerGroup(p) {
  if (p.online === false) return 2;
  return p.state === "IDLE" || p.state === "FINISH" ? 1 : 0;
}
const compareText = (a, b) => (a < b ? -1 : a > b ? 1 : 0);
function matchesFilter(p, choice) {
  return choice === "all" ||
    (choice === "printing" && p.state === "RUNNING") ||
    (choice === "idle" && idleFinished(p)) ||
    (choice === "attention" && p.attention) ||
    (choice === "unavailable" && !p.fresh);
}
function filterCount(choice) {
  return (snapshot?.printers || []).filter(p => matchesFilter(p, choice)).length;
}
function visible() {
  return (snapshot?.printers || [])
    .filter(
      (p) =>
        matchesFilter(p, filter) &&
        p.name.toLowerCase().includes(query.toLowerCase()),
    )
    .sort((a, b) => {
      const group = printerGroup(a) - printerGroup(b);
      if (group) return group;
      if (sort === "attention" && a.attention !== b.attention)
        return a.attention ? -1 : 1;
      const state = sort === "state"
        ? compareText(stateLabel(a.state).toLowerCase(), stateLabel(b.state).toLowerCase())
        : 0;
      return state || compareText(a.name.toLowerCase(), b.name.toLowerCase()) || compareText(a.id, b.id);
    });
}
function machine(p) {
  if (p.image)
    return `<img class="printer-sprite" src="${escapeHTML(p.image)}" alt="" loading="lazy" decoding="async">`;

  const color = p.attention ? "#d5b17d" : "#b2d9b9";
  return `<svg viewBox="0 0 160 150" fill="none" aria-hidden="true"><path d="M25 132L82 144L142 127L84 116Z" fill="#0d110e"/><path d="M29 29L88 16L134 29V124L80 137L29 123Z" fill="#424e46" stroke="#68796c"/><path d="M35 34L81 44V128L35 118Z" fill="#222c25" stroke="#657569"/><path d="M81 44L127 34V118L81 128Z" fill="#101c14" stroke="#657569"/><path d="M29 29L81 41L134 29L88 16Z" fill="#657269"/><path d="M40 39L75 47V117L40 110Z" fill="#19261d"/><path d="M87 48L121 40V111L87 119Z" fill="#1a2920"/><path d="M45 98L84 106L119 97L80 90Z" fill="#66796b"/><path d="M45 98V103L84 112L119 102V97L84 106Z" fill="#3c5042"/><path d="M84 54V96M113 46V95M45 50V95" stroke="#536559" stroke-width="2"/><path d="M45 59L116 69M45 65L116 75" stroke="#66786b" stroke-width="2"/><path d="M75 63L88 66V81L75 78Z" fill="#b3c5b6"/><path d="M88 66L96 63V78L88 81Z" fill="#6d8574"/><path d="M69 89L85 85L99 89V99L84 103L69 99Z" fill="${color}"/><path d="M39 28L62 22L78 26L55 32Z" fill="#9aaba0"/></svg>`;
}
function card(p) {
  const offline = p.online === false;
  const label = offline ? "Offline" : status(p);
  const heading = `<div class="card-head"><div><h3>${escapeHTML(p.name)}</h3><span class="model">${escapeHTML(p.model || "Unknown model")}</span></div><span class="pill ${p.attention || !current(p) ? "warn" : p.state === "IDLE" ? "neutral" : ""}">${escapeHTML(label)}</span></div><div class="printer-view">${machine(p)}</div>`;
  const attention = p.attention ? '<p class="card-note">◇ Attention · unreviewed conditions</p>' : "";
  const opening = `<button class="card ${p.attention ? "warning" : ""}${offline ? " offline" : ""}" data-id="${escapeHTML(p.id)}" aria-label="Inspect ${escapeHTML(p.name)}, ${escapeHTML(label)}">`;
  if (offline) return `${opening}${heading}${attention}</button>`;
  const slots = allSlots(p).filter((s) => s.present !== false),
    nozzle = p.nozzles.find((n) => n.active) || p.nozzles[0];
  const progress = Number.isFinite(p.progress)
    ? Math.max(0, Math.min(100, p.progress))
    : null;
  return `${opening}${heading}${!current(p) ? '<p class="card-note telemetry-note">Current state unconfirmed · readings are last-known; missing values remain unknown.</p>' : ""}<div class="job">${escapeHTML(p.job || "No reported job")}</div><div class="progress-label"><span>Reported progress</span><strong>${progress === null ? "Unknown" : progress + "%"}</strong></div><div class="progress-track"><span style="width:${progress || 0}%"></span></div><div class="metrics"><div>Finish estimate<strong>${escapeHTML(remaining(p))}</strong></div><div>Nozzle<strong>${temperature(nozzle?.temperature)}</strong></div><div>Bed<strong>${temperature(p.bed)}</strong></div></div><div class="card-bottom"><div class="swatches">${slots
    .slice(0, 4)
    .map((s) => swatch(s.color))
    .join(
      "",
    )}<span>${escapeHTML(slots[0]?.material || "Unknown material")}</span></div><span>${current(p) ? '<i class="green"></i>Fresh' : "◷ Last-known"}</span></div>${attention}</button>`;
}
// Keep connected card/image elements across polls. Replacing #cards.innerHTML
// recreates lazy, async-decoded images even when their immutable URL is unchanged.
function patchCardNode(current, next) {
  if (current.nodeType !== next.nodeType || current.nodeName !== next.nodeName) {
    current.replaceWith(next);
    return;
  }
  if (current.nodeType !== 1) {
    if (current.nodeValue !== next.nodeValue) current.nodeValue = next.nodeValue;
    return;
  }
  for (const {name} of Array.from(current.attributes))
    if (!next.hasAttribute(name)) current.removeAttribute(name);
  for (const {name, value} of next.attributes)
    if (current.getAttribute(name) !== value) current.setAttribute(name, value);
  const previous = Array.from(current.childNodes);
  const children = Array.from(next.childNodes);
  children.forEach((child, index) => {
    if (previous[index]) patchCardNode(previous[index], child);
    else current.appendChild(child);
  });
  for (const child of previous.slice(children.length)) child.remove();
}
function renderCards(list) {
  const container = $("#cards");
  if (!list.length) {
    container.innerHTML = `<div class="empty">${snapshot?.printers.length ? "No printers match this view." : "No discovered printers yet."}<button id="reset">Reset to all printers</button></div>`;
    return;
  }
  const ids = new Set(list.map((printer) => printer.id));
  const existing = new Map();
  // Remove filtered/disappeared cards first so remaining cards need no moves
  // when their relative order has not changed.
  for (const child of Array.from(container.children)) {
    if (ids.has(child.dataset.id)) existing.set(child.dataset.id, child);
    else child.remove();
  }
  list.forEach((printer, index) => {
    const template = document.createElement("template");
    template.innerHTML = card(printer);
    const next = template.content.firstElementChild;
    const current = existing.get(printer.id) || next;
    if (current !== next) patchCardNode(current, next);
    // Avoid moving already ordered cards: moving a connected image can itself
    // interrupt its presentation, and moving a button can disturb keyboard focus.
    if (container.children[index] !== current)
      container.insertBefore(current, container.children[index] || null);
  });
}
function render() {
  const list = visible(),
    focused = document.activeElement?.dataset.id;
  renderCards(list);
  if (focused) {
    const target = document.querySelector(`[data-id="${CSS.escape(focused)}"]`)
      || document.querySelector(`.filters [data-filter="${filter}"]`);
    target?.focus({ preventScroll: true });
  }

  const total = snapshot?.summary.total || 0;
  const noun = total === 1 ? "printer" : "printers";
  $("#result-count").textContent = list.length === total
    ? `${total} ${noun}`
    : `${list.length} of ${total} ${noun}`;
  document.querySelectorAll(".filters button").forEach((b) => {
    b.classList.toggle("selected", b.dataset.filter === filter);
    b.setAttribute("aria-pressed", String(b.dataset.filter === filter));
  });
  if (!snapshot) return;
  for (const key of ["total", "printing", "attention", "unavailable"])
    $("#" + key).textContent = String(snapshot.summary[key] || 0).padStart(
      2,
      "0",
    );
  for (const choice of ["all", "printing", "idle", "attention", "unavailable"])
    $("#" + choice + "-count").textContent = filterCount(choice);
  $("#idle-filter-help").hidden = filter !== "idle";
  $(".attention-banner").hidden = !snapshot.summary.attention;
  $("#attention-text").textContent =
    `${snapshot.summary.attention} printer(s) have unreviewed actionable conditions. Physical resolution is unconfirmed.`;
  $("#feed-state").textContent = feedUnavailable()
    ? "Dashboard disconnected"
    : snapshot.demo
      ? "● Demo monitoring"
      : snapshot.connected
        ? "● Monitor connected"
        : "Cloud disconnected";
  $("#feed-state").classList.toggle(
    "amber",
    feedUnavailable() || !snapshot.connected,
  );
  $("#updated").textContent = localTime(snapshot.at);
  if (selected && $("#detail").open) renderDetail();
}
function slotCards(slots) {
  if (!slots.length) return "<p>Slot inventory unknown.</p>";
  return `<div class="spools">${slots.map((s) => `<div>${swatch(s.color)}<br>Slot ${unknown(s.id)}<small>${s.present === false ? "Empty (reported)" : s.present === null ? "Presence unknown" : escapeHTML(s.material || "Unknown material")}</small><small>Color: ${escapeHTML(s.color || "Unknown")}</small><small>Remaining: ${s.remaining === null ? "Unknown" : unknown(s.remaining) + "%"}</small></div>`).join("")}</div>`;
}
function renderDetail() {
  const p = snapshot.printers.find((p) => p.id === selected),
    body = $("#detail-body");
  if (!p) {
    $("#detail-title").textContent = "Printer unavailable";
    body.innerHTML =
      "<p>This printer is no longer in the discovered farm. Its removal does not confirm fault resolution.</p>";
    return;
  }
  const scroll = $("#detail").scrollTop;
  const focusedAction = document.activeElement?.dataset.dismiss;
  const focusedToggle = document.activeElement?.id === "toggle-hidden-messages";
  const hiddenCount = p.conditions.filter((c) => c.hidden).length;
  const conditions = p.conditions.filter((c) => showHiddenMessages || !c.hidden);
  $("#detail-title").textContent = p.name;
  const metric = (label, value) =>
    `<div>${prefix(p)}${label}<strong>${value}</strong></div>`;
  body.innerHTML = `<span class="pill ${current(p) ? "" : "warn"}">${escapeHTML(status(p))}</span><p>${escapeHTML(p.model || "Unknown model")} · Last report: ${escapeHTML(localTime(p.last_report))}<br>${current(p) ? "Readings from the latest monitoring snapshot." : "Current state is unconfirmed. Available readings are last-known; missing values remain unknown."}</p><h2>${escapeHTML(p.job || "No reported job")}</h2><p>${prefix(p)}Stage: ${escapeHTML(p.stage || "Unknown")}</p><div class="detail-grid">${metric("Reported state", escapeHTML(stateLabel(p.state)))}${metric("Progress", p.progress === null ? "Unknown" : unknown(p.progress) + "%")}${metric("Layer", unknown(p.layer) + " / " + unknown(p.total_layers))}${metric("Bed / target", temperature(p.bed))}${metric("Chamber", temperature(p.chamber))}${metric("Finish estimate", escapeHTML(remaining(p)))}</div><div class="temperature-list">${p.nozzles.map((n) => `<span>${prefix(p)}${n.id === 0 ? "Right nozzle" : n.id === 1 ? "Left nozzle" : "Nozzle " + unknown(n.id)}: ${temperature(n.temperature)}${n.active ? " · reported active" : ""}</span>`).join("") || "Nozzle readings unknown"}</div><p class="muted">Remaining-time report: ${escapeHTML(localTime(p.remaining_reported))}. Estimates do not confirm completion.</p><div class="conditions"><h2>Reported conditions</h2><p role="status" aria-live="polite">${escapeHTML(commandMessage)}</p><p>Local decision storage: ${escapeHTML(snapshot.decisions || "Unknown")}. Hiding never fixes the printer.</p>${hiddenCount ? `<p><button id="toggle-hidden-messages" aria-pressed="${showHiddenMessages}">${showHiddenMessages ? "Hide hidden messages" : "View hidden messages"} (${hiddenCount})</button></p>` : ""}${conditions.length ? conditions.map((c) => `<div class="condition"><h3>${escapeHTML(c.text)}</h3>${c.guidance ? `<p><strong>Next:</strong> ${escapeHTML(c.guidance)}</p>` : ""}<p class="meta">${escapeHTML(c.level)} · Code: ${escapeHTML(c.code || "Not provided")}<br>Local review: ${c.reviewed ? "Reviewed" : "Unreviewed"}${c.hidden ? " · Hidden locally" : ""}<br>First app-observed: ${escapeHTML(localTime(c.since))}</p><p>Physical resolution is unconfirmed. Inspecting this view makes no local notice decision.</p>${!c.hidden ? `<button data-dismiss="${escapeHTML(c.id)}" aria-disabled="${Boolean(commandPending || feedUnavailable())}">${commandPending === c.id ? "Pending…" : "Hide notice locally"}</button>` : snapshot.decisions === "failed" ? `<button data-dismiss="${escapeHTML(c.id)}" data-retry="true" aria-disabled="${Boolean(commandPending || feedUnavailable())}">Retry saving local decisions</button>` : `<p>Applied locally · ${escapeHTML(snapshot.decisions || "storage unknown")}</p>`}</div>`).join("") : p.conditions.length ? "<p>All reported messages are hidden locally. Physical resolution remains unconfirmed.</p>" : "<p>No active notices in this snapshot. Missing or disconnected data does not confirm physical fault resolution.</p>"}</div><h2 style="margin-top:25px">${prefix(p)}AMS & filament</h2>${p.ams.length ? p.ams.map((unit) => `<p>${escapeHTML(unit.label)} · Humidity: ${unit.humidity === null ? "Unknown" : unknown(unit.humidity) + "%"} · Temperature: ${unit.temperature === null ? "Unknown" : unknown(unit.temperature) + "°C"}</p>${slotCards(unit.slots)}`).join("") : "<p>AMS configuration unknown.</p>"}<h2 style="margin-top:20px">${prefix(p)}External filament</h2>${slotCards(p.external)}`;
  $("#detail").scrollTop = scroll;
  if (focusedAction) {
    const next = document.querySelector(`[data-dismiss="${CSS.escape(focusedAction)}"]`);
    if (next) next.focus({preventScroll: true});
    else $("#close-detail").focus({preventScroll: true});
  }
  if (focusedToggle) {
    ($("#toggle-hidden-messages") || $("#close-detail")).focus({preventScroll: true});
  }
}
function inspect(id) {
  showHiddenMessages = false;
  selected = id;
  renderDetail();
  $("#detail").showModal();
  $("#detail").scrollTop = 0;
}
function updateFeedWarning(message) {
  $("#feed-warning").hidden = !message;
  $("#feed-warning").textContent = message;
}
async function poll() {
  clearTimeout(timer);
  if (document.hidden && !(typeof browserNotifications !== "undefined" && browserNotifications.enabled())) return;
  controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 5000);
  try {
    const response = await fetch("/api/snapshot", {
      cache: "no-store",
      signal: controller.signal,
    });
    if (!response.ok) throw new Error("Snapshot unavailable");
    const data = await response.json();
    if (
      !Array.isArray(data.printers) ||
      !data.summary ||
      !data.at ||
      !Number.isFinite(Date.parse(data.at))
    )
      throw new Error("Invalid snapshot");
    snapshot = data;
    feedError = false;
    if (typeof browserNotifications !== "undefined") {
      if (!feedDelayed()) await browserNotifications.poll();
      else browserNotifications.disconnected();
    }
    updateFeedWarning(
      feedDelayed()
        ? "Monitoring snapshot is delayed. Current printer state is unconfirmed; readings below are last-known."
        : "",
    );
  } catch {
    feedError = true;
    if (typeof browserNotifications !== "undefined") browserNotifications.disconnected();
    updateFeedWarning(
      "Cannot reach the dashboard server. Current printer state is unconfirmed. Last-known data remains below. Check the monitoring process and configured web server port.",
    );
    $("#feed-state").textContent = "Dashboard disconnected";
    $("#feed-state").classList.add("amber");
  } finally {
    clearTimeout(timeout);
    controller = null;
    if (snapshot) render();
    timer = setTimeout(poll, 3000);
  }
}
async function dismissNotice(target, retry) {
  if (commandPending || feedUnavailable()) return;
  commandPending = target;
  commandMessage = "Pending local decision…";
  renderDetail();
  try {
    if (!csrfToken) {
      const session = await fetch("/api/session", {cache: "no-store", signal: AbortSignal.timeout(5000)});
      if (!session.ok) throw new Error();
      csrfToken = (await session.json()).token;
    }
    const response = await fetch("/api/dismiss", {
      method: "POST", headers: {"Content-Type": "application/json", "X-CSRF-Token": csrfToken},
      body: JSON.stringify({target, retry}), signal: AbortSignal.timeout(5000)
    });
    if (!response.ok) {
      csrfToken = null;
      commandMessage = "Request failed or outcome unknown. Refresh the state before retrying; the notice may already be hidden.";
    } else {
      commandMessage = "Applied locally. Check storage status below for saving, saved, session-only, or failed. Physical resolution remains unconfirmed.";
    }
  } catch {
    commandMessage = "Outcome unknown. Refresh the state before retrying; this does not confirm a saved decision.";
  } finally {
    commandPending = null;
    if ($("#detail").open) renderDetail();
  }
}
document.addEventListener("click", (event) => {
  if (event.target.closest("#toggle-hidden-messages")) {
    showHiddenMessages = !showHiddenMessages;
    renderDetail();
    return;
  }
  const action = event.target.closest("[data-dismiss]");
  if (action) { dismissNotice(action.dataset.dismiss, action.dataset.retry === "true"); return; }
  const f = event.target.closest("[data-filter]");
  if (f) {
    filter = f.dataset.filter;
    render();
  }
  const card = event.target.closest("[data-id]");
  if (card) inspect(card.dataset.id);
  if (event.target.id === "reset") {
    filter = "all";
    query = "";
    sort = "name";
    $("#search").value = "";
    $("#sort").value = "name";
    render();
  }
});
$("#search").addEventListener("input", (event) => {
  query = event.target.value;
  render();
});
$("#sort").addEventListener("change", (event) => {
  sort = event.target.value;
  render();
});
$("#review").onclick = () => {
  const p = snapshot?.printers.find((p) => p.attention);
  if (p) inspect(p.id);
};
$("#close-detail").onclick = () => $("#detail").close();
$("#settings").onclick = () => $("#settings-dialog").showModal();
$("#close-settings").onclick = () => $("#settings-dialog").close();
$("#about").onclick = () => $("#about-dialog").showModal();
$("#close-about").onclick = () => $("#about-dialog").close();
// The header sits outside the scrolling workspace. Keep the height it releases
// in the already-scrolled introduction, so collapse never shifts visible cards
// or changes the scroll range. No animation: geometry and focus stay in sync.
function setupWorkspaceLayout() {
  const shell = $(".shell");
  if (!shell) return;
  const header = $("header"), toolbar = $(".toolbar");
  let compact = false;
  let restoreAnchorFrame = 0;
  function measure() {
    // Header resizing already compensates in the introduction. Suppress browser
    // anchoring through the compensated frame, not later telemetry reflow.
    cancelAnimationFrame(restoreAnchorFrame);
    shell.style.overflowAnchor = "none";
    header.classList.remove("compact");
    const expandedHeight = header.getBoundingClientRect().height;
    header.classList.toggle("compact", compact);
    const reclaimed = expandedHeight - header.getBoundingClientRect().height;
    shell.style.setProperty("--header-reclaimed", `${reclaimed}px`);
    const toolbarHeight = toolbar.getBoundingClientRect().height;
    const minimum = Math.min(12 * parseFloat(getComputedStyle(document.documentElement).fontSize), innerHeight / 2);
    const pin = shell.clientHeight >= toolbarHeight + minimum;
    shell.classList.toggle("toolbar-scrolls", !pin);
    restoreAnchorFrame = requestAnimationFrame(() => {
      restoreAnchorFrame = requestAnimationFrame(() => {
        shell.style.removeProperty("overflow-anchor");
      });
    });
  }
  shell.addEventListener("scroll", () => {
    const next = shell.scrollTop > 24;
    if (next === compact) return;
    compact = next;
    measure();
  }, {passive: true});
  // Reflow can change header, toolbar, and footer heights (including 200% text).
  const observer = new ResizeObserver(measure);
  for (const element of [shell, header, toolbar]) observer.observe(element);
  shell.addEventListener("focusin", event => {
    const card = event.target.closest(".card");
    if (!card) return;
    requestAnimationFrame(() => {
      if (document.activeElement !== card || shell.classList.contains("toolbar-scrolls")) return;
      const top = shell.getBoundingClientRect().top + toolbar.getBoundingClientRect().height + 8;
      const cardTop = card.getBoundingClientRect().top;
      if (cardTop < top) shell.scrollTop += cardTop - top;
    });
  });
  measure();
}
setupWorkspaceLayout();
document.addEventListener("visibilitychange", () => {
  if (!controller) poll();
});
poll();
