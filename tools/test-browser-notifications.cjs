// Dependency-free permission and delivery interaction tests. Real-browser
// inspection is separate: these fixtures cannot prove OS presentation.
const {test} = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const path = require("node:path");
function browser({permission = "default", supported = true, secure = true, stored = false} = {}) {
  const elements = new Map(), listeners = {}, notices = [], requests = [];
  for (const id of ["notification-status", "enable-notifications", "test-notifications", "disable-notifications", "about-dialog", "settings-dialog"])
    elements.set("#" + id, {textContent: "", disabled: false, open: false, close() { this.open = false; }});
  let now = Date.now();
  let fetchWait = null;
  class Clock extends Date { static now() { return now; } }
  let value = stored ? "on" : null, requested = 0, response = {cursor: 1, events: []}, fail = false, constructFail = false, inspected = null;
  class FakeNotification {
    static permission = permission;
    static async requestPermission() { requested++; FakeNotification.permission = "granted"; return "granted"; }
    constructor(title, options) { if (constructFail) throw new Error(); this.title = title; this.options = options; notices.push(this); }
    close() { this.closed = true; }
  }
  const context = vm.createContext({
    document: {querySelector: (s) => elements.get(s)},
    window: {isSecureContext: secure, addEventListener: (n, fn) => listeners[n] = fn, focus() {}},
    Notification: supported ? FakeNotification : undefined,
    localStorage: {getItem: () => value, setItem: (_, v) => value = v},
    Date: Clock, AbortSignal,
    fetch: async (url, options) => {
      requests.push({url, options});
      if (fail) throw new Error();
      if (url === "/api/notifications") await fetchWait;
      return {ok: true, json: async () => url === "/api/session" ? {token: "csrf"} : response};
    },
    snapshot: {printers: [{id: "printer"}]}, inspect: (id) => inspected = id,
  });
  vm.runInContext(fs.readFileSync(path.join(__dirname, "../internal/webserver/assets/notifications.js"), "utf8"), context);
  return {
    run: (s) => vm.runInContext(s, context), elements, listeners, notices, requests,
    click: (id) => elements.get("#" + id).onclick(),
    requested: () => requested,
    status: () => elements.get("#notification-status").textContent,
    setResponse: (r) => response = r,
    setFetchWait: (p) => fetchWait = p,
    fail: (v) => fail = v, constructFail: () => constructFail = true,
    permission: (v) => FakeNotification.permission = v,
    inspected: () => inspected, advance: (ms) => now += ms,
  };
}
const event = {id: "opaque", printer: "printer", title: "Synthetic printer", body: "Needs attention", sound: false};

test("permission alone never opts in; Enable is explicit and establishes a baseline", async () => {
  const b = browser({permission: "granted"});
  assert.equal(b.run("browserNotifications.enabled()"), false);
  await b.run("browserNotifications.poll()");
  assert.equal(b.requests.length, 0);
  await b.click("enable-notifications");
  assert.equal(b.requested(), 0);
  assert.equal(b.run("browserNotifications.enabled()"), true);
  const claim = b.requests.find((r) => r.url === "/api/notifications");
  assert.deepEqual(JSON.parse(claim.options.body), {baseline: true, after: 0});
  assert.equal(claim.options.headers["X-CSRF-Token"], "csrf");
  assert.equal(b.notices.length, 0);
});

test("permission request follows a click; Test is synthetic and keeps automatic opt-out", async () => {
  const b = browser();
  assert.equal(b.requested(), 0);
  await b.click("test-notifications");
  assert.equal(b.requested(), 1);
  assert.equal(b.notices.length, 1);
  assert.equal(b.run("browserNotifications.enabled()"), false);
  assert.equal(b.requests.length, 0);
  assert.match(b.status(), /banner and sound unconfirmed/);
});

test("repeated Test clicks submit separate silent notifications without replacement tags", async () => {
  const b = browser();
  for (let i = 0; i < 3; i++) await b.click("test-notifications");
  assert.equal(b.notices.length, 3);
  for (const notice of b.notices) {
    assert.equal(notice.options.tag, "");
    assert.equal(notice.options.silent, true);
  }
  assert.equal(b.requested(), 1);
  assert.equal(b.run("browserNotifications.enabled()"), false);
  assert.equal(b.requests.length, 0);
});

test("denial, unsupported API and insecure context preserve monitoring", async () => {
  for (const options of [{permission: "denied"}, {supported: false}, {secure: false}]) {
    const b = browser(options);
    await b.click("enable-notifications");
    await b.run("browserNotifications.poll()");
    assert.equal(b.run("browserNotifications.enabled()"), false);
    assert.equal(b.notices.length, 0);
    assert.equal(b.requested(), 0);
    assert.match(b.status(), /denied|Unsupported|Unavailable/);
  }
});

test("claimed notifications navigate to printer on click and close makes no decision", async () => {
  const b = browser({permission: "granted", stored: true});
  await b.run("browserNotifications.poll()");
  b.setResponse({cursor: 2, events: [event]});
  await b.run("browserNotifications.poll()");
  assert.equal(b.notices.length, 1);
  assert.equal(b.notices[0].options.silent, true);
  assert.equal(b.notices[0].options.tag, event.id);
  assert.equal(b.notices[0].onclose, undefined);
  b.elements.get("#settings-dialog").open = true;
  b.notices[0].onclick();
  assert.equal(b.elements.get("#settings-dialog").open, false);
  assert.equal(b.inspected(), "printer");
  assert.equal(b.notices[0].closed, true);
  assert.equal(b.requests.some((r) => r.url === "/api/dismiss"), false);
  assert.deepEqual(JSON.parse(b.requests.at(-1).options.body), {baseline: false, after: 1});
});

test("reload, disconnection and long suspension establish a baseline without replay", async () => {
  const b = browser({permission: "granted", stored: true});
  await b.run("browserNotifications.poll()");
  b.fail(true);
  await b.run("browserNotifications.poll()");
  assert.match(b.status(), /outcome unknown/);
  b.fail(false);
  await b.run("browserNotifications.poll()");
  assert.equal(JSON.parse(b.requests.at(-1).options.body).baseline, true);
  b.advance(11000);
  await b.run("browserNotifications.poll()");
  assert.equal(JSON.parse(b.requests.at(-1).options.body).baseline, true);
  // A new page has a new cursor even when opt-in is retained.
  const reload = browser({permission: "granted", stored: true});
  await reload.run("browserNotifications.poll()");
  assert.equal(JSON.parse(reload.requests.at(-1).options.body).baseline, true);
});

for (const phase of ["request", "response body"]) {
  test(`long suspension during an in-flight ${phase} discards claims and resets the baseline`, async () => {
    const b = browser({permission: "granted", stored: true});
    await b.run("browserNotifications.poll()");
    let release;
    const paused = new Promise((resolve) => release = resolve);
    const claimed = {cursor: 2, events: [event]};
    if (phase === "request") {
      b.setResponse(claimed);
      b.setFetchWait(paused);
    } else {
      b.setResponse(paused);
    }
    const pending = b.run("browserNotifications.poll()");
    await new Promise(setImmediate);
    assert.deepEqual(JSON.parse(b.requests.at(-1).options.body), {baseline: false, after: 1});
    b.advance(20000);
    release(claimed);
    await pending;
    assert.equal(b.notices.length, 0);
    assert.match(b.status(), /new baseline without backlog/);

    b.setResponse({cursor: 3, events: []});
    await b.run("browserNotifications.poll()");
    assert.deepEqual(JSON.parse(b.requests.at(-1).options.body), {baseline: true, after: 0});
    b.setResponse({cursor: 4, events: [{...event, id: "fresh"}]});
    await b.run("browserNotifications.poll()");
    assert.deepEqual(JSON.parse(b.requests.at(-1).options.body), {baseline: false, after: 3});
    assert.equal(b.notices.length, 1);
    assert.equal(b.notices[0].options.tag, "fresh");
  });
}

test("constructor and asynchronous submission errors remain visible", async () => {
  const b = browser({permission: "granted", stored: true});
  await b.run("browserNotifications.poll()");
  b.setResponse({cursor: 2, events: [event]}); b.constructFail();
  await b.run("browserNotifications.poll()");
  assert.match(b.status(), /attempt will not be retried/);
  const c = browser({permission: "granted"});
  await c.click("test-notifications");
  c.notices[0].onerror();
  assert.match(c.status(), /submission failed/);
});

test("Disable propagates across tabs; permission revocation stops automatic claims", async () => {
  const b = browser({permission: "granted", stored: true});
  b.listeners.storage({key: "bambu-tui.browser-notifications.v1", newValue: "off"});
  assert.equal(b.run("browserNotifications.enabled()"), false);
  await b.click("enable-notifications");
  await b.click("disable-notifications");
  assert.equal(b.run("browserNotifications.enabled()"), false);
  assert.match(b.status(), /OS permission is unchanged/);
  await b.click("enable-notifications"); b.permission("denied");
  const count = b.requests.length;
  await b.run("browserNotifications.poll()");
  assert.equal(b.requests.length, count);
  assert.match(b.status(), /Permission: denied/);
});

test("server ledger failures are shown without submitting a notification", async () => {
  const b = browser({permission: "granted", stored: true});
  b.setResponse({cursor: 0, events: [], error: "Browser notifications paused: repair storage"});
  await b.run("browserNotifications.poll()");
  assert.match(b.status(), /repair storage/);
  assert.equal(b.notices.length, 0);
});

test("disable and re-enable during an in-flight claim discards the old generation", async () => {
  const b = browser({permission: "granted", stored: true});
  await b.run("browserNotifications.poll()");
  let release;
  b.setResponse(new Promise((resolve) => release = resolve));
  const pending = b.run("browserNotifications.poll()");
  await new Promise(setImmediate);
  await b.click("disable-notifications");
  await b.click("enable-notifications");
  release({cursor: 2, events: [event]});
  await pending;
  assert.equal(b.notices.length, 0);
  b.setResponse({cursor: 2, events: []});
  await b.run("browserNotifications.poll()");
  assert.equal(JSON.parse(b.requests.at(-1).options.body).baseline, true);
});
