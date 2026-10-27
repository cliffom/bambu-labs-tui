// Run with: node --test tools/test-browser-notices.cjs
const {test} = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const path = require("node:path");

function inspector() {
  const elements = new Map();
  const listeners = {};
  const document = {
    hidden: true,
    activeElement: null,
    querySelector: (selector) => elements.get(selector) || null,
    addEventListener: (name, callback) => { listeners[name] = callback; },
  };
  function element(selector, dataset = {}) {
    const value = {
      id: selector.slice(1), dataset, scrollTop: 0, open: false,
      addEventListener() {},
      focus(options) {
        document.activeElement = value;
        // Close is at the dialog header; ordinary focus scrolls it into view.
        if (selector === "#close-detail" && !options?.preventScroll)
          elements.get("#detail").scrollTop = 0;
      },
      showModal() { value.open = true; },
      close() { value.open = false; },
    };
    elements.set(selector, value);
    return value;
  }
  for (const id of ["detail", "detail-title", "detail-body", "close-detail",
    "search", "sort", "nav-farm", "nav-attention", "nav-unavailable",
    "review", "about", "close-about", "settings", "settings-dialog", "close-settings"]) element("#" + id);
  let html = "";
  Object.defineProperty(elements.get("#detail-body"), "innerHTML", {
    get: () => html,
    set(value) {
      html = value;
      if (document.activeElement?.dataset.dismiss ||
          document.activeElement?.id === "toggle-hidden-messages")
        document.activeElement = null;
      for (const key of elements.keys())
        if (key.startsWith("[data-dismiss=") || key === "#toggle-hidden-messages")
          elements.delete(key);
      if (value.includes('id="toggle-hidden-messages"')) element("#toggle-hidden-messages");
      for (const match of value.matchAll(/data-dismiss="([^"]+)"/g))
        element('[data-dismiss="' + match[1] + '"]', {dismiss: match[1]});
    },
  });
  const context = vm.createContext({
    document, CSS: {escape: (s) => s}, Date, setInterval() {}, clearTimeout() {},
  });
  vm.runInContext(fs.readFileSync(path.join(__dirname,
    "../internal/webserver/assets/app.js"), "utf8"), context);
  const run = (code) => vm.runInContext(code, context);
  run(`snapshot = {at: new Date().toISOString(), decisions: "saved", printers: [{
    id: "printer", name: "Synthetic printer", fresh: true, connection: "Live",
    state: "IDLE", nozzles: [], ams: [], external: [], conditions: [
      {id: "visible", text: "Visible condition", hidden: false},
      {id: "hidden", text: "Hidden condition", hidden: true, reviewed: true}
    ]}]}; inspect("printer");`);
  return {
    run, html: () => html,
    focus(selector) { elements.get(selector).focus(); },
    focused: () => document.activeElement?.id,
    toggle() {
      const target = elements.get("#toggle-hidden-messages");
      listeners.click({target: {closest: (selector) =>
        selector === "#toggle-hidden-messages" ? target : null}});
    },
  };
}

test("header settings opens and closes independently of live printer inspection", () => {
  const view = inspector();
  view.run('$("#settings").onclick()');
  assert.equal(view.run('$("#settings-dialog").open'), true);
  view.run('snapshot.printers[0].conditions[0].text = "Updated while settings open"; renderDetail()');
  assert.equal(view.run('$("#settings-dialog").open'), true);
  view.run('$("#close-settings").onclick()');
  assert.equal(view.run('$("#settings-dialog").open'), false);
  assert.match(view.html(), /Updated while settings open/);
});

test("dismissed conditions are hidden by default and inspectable without changing decisions", () => {
  const view = inspector();
  assert.match(view.html(), /Visible condition/);
  assert.doesNotMatch(view.html(), /Hidden condition/);
  assert.match(view.html(), /View hidden messages \(1\)/);
  view.focus("#toggle-hidden-messages");
  view.toggle();
  assert.match(view.html(), /Hidden condition/);
  assert.match(view.html(), /aria-pressed="true"/);
  assert.equal(view.focused(), "toggle-hidden-messages");
  view.run("renderDetail()");
  assert.match(view.html(), /Hidden condition/);
  assert.equal(view.focused(), "toggle-hidden-messages");
  assert.equal(view.run("snapshot.printers[0].conditions[1].hidden"), true);
  view.toggle();
  assert.doesNotMatch(view.html(), /Hidden condition/);
  view.toggle();
  view.run('inspect("printer")');
  assert.doesNotMatch(view.html(), /Hidden condition/);
});

test("all-hidden and empty views differ; new conditions remain visible", () => {
  const view = inspector();
  view.run("snapshot.printers[0].conditions[0].hidden = true; renderDetail()");
  assert.match(view.html(), /All reported messages are hidden locally/);
  assert.match(view.html(), /View hidden messages \(2\)/);
  view.run('snapshot.printers[0].conditions.push({id:"new", text:"New occurrence", hidden:false}); renderDetail()');
  assert.match(view.html(), /New occurrence/);
  view.run("snapshot.printers[0].conditions = []; renderDetail()");
  assert.match(view.html(), /No active notices in this snapshot/);
  assert.doesNotMatch(view.html(), /toggle-hidden-messages/);
});

test("live dismissals preserve scroll and fall back to Close; disappearing toggle does too", () => {
  const view = inspector();
  view.focus('[data-dismiss="visible"]');
  view.run('$("#detail").scrollTop = 120; snapshot.printers[0].conditions[0].hidden = true; renderDetail()');
  assert.equal(view.focused(), "close-detail");
  assert.equal(view.run('$("#detail").scrollTop'), 120);
  view.focus("#toggle-hidden-messages");
  view.run("snapshot.printers[0].conditions = []; renderDetail()");
  assert.equal(view.focused(), "close-detail");
  assert.equal(view.run('$("#detail").scrollTop'), 120);
});

test("telemetry removal of a focused condition preserves inspector scroll", () => {
  const view = inspector();
  view.focus('[data-dismiss="visible"]');
  view.run('$("#detail").scrollTop = 120; snapshot.printers[0].conditions = []; renderDetail()');
  assert.equal(view.focused(), "close-detail");
  assert.equal(view.run('$("#detail").scrollTop'), 120);
});

test("ongoing estimate updates retain action focus and inspector scroll", () => {
  const view = inspector();
  view.focus('[data-dismiss="visible"]');
  view.run('$("#detail").scrollTop = 120; snapshot.printers[0].conditions[0].text = "Estimated filament 8%"; renderDetail()');
  assert.match(view.html(), /Estimated filament 8%/);
  assert.equal(view.run("document.activeElement?.dataset.dismiss"), "visible");
  assert.equal(view.run('$("#detail").scrollTop'), 120);
});

test("failed-save retry remains available through hidden messages, including feed outages", () => {
  const view = inspector();
  view.run('snapshot.decisions = "failed"; renderDetail()');
  assert.match(view.html(), /Local decision storage: failed/);
  view.toggle();
  assert.match(view.html(), /data-dismiss="hidden" data-retry="true"/);
  view.run("feedError = true; renderDetail()");
  assert.match(view.html(), /data-retry="true" aria-disabled="true"/);
  view.toggle();
  assert.doesNotMatch(view.html(), /Hidden condition/);
});
