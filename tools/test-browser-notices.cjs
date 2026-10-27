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

// Minimal tree operations for card reconciliation; card() is replaced with a
// synthetic tree builder so these tests exercise identity/lifetime, not parsing.
function cardTree(name, attributes = {}, children = []) {
  const node = {
    nodeType: name === "#text" ? 3 : 1, nodeName: name,
    nodeValue: name === "#text" ? String(attributes) : null,
    attrs: name === "#text" ? {} : {...attributes}, childNodes: [], parent: null,
    attributeWrites: 0, moves: 0,
    get attributes() { return Object.entries(this.attrs).map(([name, value]) => ({name, value})); },
    get children() { return this.childNodes.filter((n) => n.nodeType === 1); },
    get dataset() { return {id: this.attrs["data-id"]}; },
    get firstElementChild() { return this.children[0] || null; },
    hasAttribute(name) { return Object.hasOwn(this.attrs, name); },
    getAttribute(name) { return this.attrs[name] ?? null; },
    setAttribute(name, value) { this.attrs[name] = value; this.attributeWrites++; },
    removeAttribute(name) { delete this.attrs[name]; this.attributeWrites++; },
    appendChild(child) { return this.insertBefore(child, null); },
    insertBefore(child, before) {
      child.moves++;
      child.remove();
      const index = before ? this.childNodes.indexOf(before) : this.childNodes.length;
      assert.notEqual(index, -1);
      this.childNodes.splice(index, 0, child);
      child.parent = this;
      return child;
    },
    remove() {
      if (!this.parent) return;
      this.parent.childNodes.splice(this.parent.childNodes.indexOf(this), 1);
      this.parent = null;
    },
    replaceWith(next) {
      const parent = this.parent;
      parent.insertBefore(next, this);
      this.remove();
    },
  };
  for (const child of children) node.appendChild(child);
  return node;
}
function cards() {
  const elements = new Map();
  const container = cardTree("DIV");
  Object.defineProperty(container, "innerHTML", {
    set(value) {
      for (const child of [...this.childNodes]) child.remove();
      this.appendChild(cardTree("DIV", {class: "empty"}, [cardTree("#text", value)]));
    },
  });
  const document = {
    hidden: true, activeElement: null, addEventListener() {},
    querySelector(selector) {
      if (selector === "#cards") return container;
      if (!elements.has(selector)) elements.set(selector, {addEventListener() {}});
      return elements.get(selector);
    },
    createElement(name) {
      assert.equal(name, "template");
      return {content: {}, set innerHTML(printer) {
        this.content.firstElementChild = cardTree("BUTTON", {
          "data-id": printer.id, class: printer.attention ? "card warning" : "card",
          "aria-label": printer.name,
        }, [
          cardTree("H3", {}, [cardTree("#text", printer.name)]),
          cardTree("DIV", {class: "printer-view"}, [
            printer.image ? cardTree("IMG", {src: printer.image, loading: "lazy", decoding: "async"}) : cardTree("SVG"),
            cardTree("SPAN", {}, [cardTree("#text", printer.attention ? "INSPECT CONDITIONS" : "MONITORING")]),
          ]),
          cardTree("STRONG", {}, [cardTree("#text", printer.progress)]),
          ...(printer.attention ? [cardTree("P", {}, [cardTree("#text", "Action needed")])] : []),
        ]);
      }};
    },
  };
  const context = vm.createContext({document, setInterval() {}, clearTimeout() {}});
  vm.runInContext(fs.readFileSync(path.join(__dirname,
    "../internal/webserver/assets/app.js"), "utf8"), context);
  vm.runInContext('card = (printer) => printer; snapshot = {printers: []}', context);
  return {container, update(printers) {
    context.printers = printers;
    vm.runInContext("snapshot.printers = printers; renderCards(printers)", context);
  }};
}

test("polls update telemetry without disconnecting cards or rewriting unchanged image attributes", () => {
  const view = cards();
  const printer = {id: "one", name: "Synthetic printer", image: "/printers/h2d.png", progress: 12};
  view.update([printer]);
  const card = view.container.children[0];
  const image = card.children[1].children[0];
  const moves = card.moves;
  for (let progress = 13; progress <= 20; progress++) {
    view.update([{...printer, name: "Updated name", progress, attention: progress % 2 === 0}]);
    assert.equal(view.container.children[0], card);
    assert.equal(card.children[1].children[0], image);
    assert.equal(image.attributeWrites, 0);
    assert.equal(card.moves, moves);
    assert.equal(image.parent.parent.parent, view.container);
    assert.equal(card.children[0].childNodes[0].nodeValue, "Updated name");
    assert.equal(card.children[2].childNodes[0].nodeValue, String(progress));
    assert.equal(card.children.length, progress % 2 === 0 ? 4 : 3);
    assert.equal(card.getAttribute("aria-label"), "Updated name");
    assert.equal(card.getAttribute("class"), progress % 2 === 0 ? "card warning" : "card");
  }
});

test("reordering, filtering, discovery, and empty views keep the correct printer identities", () => {
  const view = cards();
  const a = {id: 'a"quoted', name: "Alpha", image: "/a.png", progress: 0};
  const b = {id: "b", name: "Beta", image: "/b.png", progress: 0};
  view.update([a, b]);
  const [first, second] = view.container.children;
  view.update([b, a]);
  assert.deepEqual(view.container.children, [second, first]);
  const firstMoves = first.moves;
  view.update([a]);
  assert.equal(first.moves, firstMoves);
  assert.deepEqual(view.container.children, [first]);
  assert.equal(second.parent, null);
  view.update([a, b, {id: "c", name: "New", progress: 1}]);
  assert.equal(view.container.children[0], first);
  assert.equal(view.container.children.length, 3);
  view.update([]);
  assert.equal(first.parent, null);
  assert.equal(view.container.children[0].getAttribute("class"), "empty");
  view.update([b]);
  assert.equal(view.container.children.length, 1);
  assert.equal(view.container.children[0].dataset.id, "b");
});

test("changed model images and transitions to/from generic artwork update in place", () => {
  const view = cards();
  const printer = {id: "one", name: "Printer", progress: 1, image: "/a.png"};
  view.update([printer]);
  const card = view.container.children[0];
  const image = card.children[1].children[0];
  view.update([{...printer, image: "/b.png"}]);
  assert.equal(card.children[1].children[0], image);
  assert.equal(image.getAttribute("src"), "/b.png");
  assert.equal(image.attributeWrites, 1);
  view.update([{...printer, image: null}]);
  assert.equal(card.children[1].children[0].nodeName, "SVG");
  view.update([printer]);
  assert.equal(card.children[1].children[0].nodeName, "IMG");
  assert.equal(card.children[1].children[0].getAttribute("src"), "/a.png");
  assert.equal(view.container.children[0], card);
});

for (const mode of ["name", "attention", "state"]) {
  test(`${mode} sorting groups non-idle, idle/done, and offline devices with deterministic ties`, () => {
    const view = inspector();
    const printers = [
      {id: "offline-idle", name: "A", online: false, state: "IDLE", attention: true},
      {id: "idle", name: "A idle", online: true, state: "IDLE", attention: true},
      {id: "done", name: "B done", online: true, state: "FINISH"},
      {id: "b", name: "same", online: true, state: "RUNNING"},
      {id: "a", name: "Same", online: true, state: "RUNNING"},
      {id: "paused", name: "Z paused", online: true, state: "PAUSE", attention: true},
      {id: "offline-running", name: "B", online: false, state: "RUNNING"},
    ];
    view.run(`snapshot.printers = ${JSON.stringify(printers)}; sort = ${JSON.stringify(mode)}`);
    const ids = () => JSON.parse(view.run('JSON.stringify(visible().map(p => p.id))'));
    const want = mode === "name"
      ? ["a", "b", "paused", "idle", "done", "offline-idle", "offline-running"]
      : mode === "attention"
        ? ["paused", "a", "b", "idle", "done", "offline-idle", "offline-running"]
        : ["paused", "a", "b", "done", "idle", "offline-idle", "offline-running"];
    assert.deepEqual(ids(), want);
    // A cloud outage masks even offline devices' connection labels. Device
    // flags and last-known states must still control grouping, never freshness.
    view.run('feedError = true; snapshot.connected = false; for (const p of snapshot.printers) { p.connection = "Cloud disconnected"; p.fresh = false; }');
    assert.deepEqual(ids(), want);
    view.run('filter = "printing"');
    assert.deepEqual(ids(), ["a", "b", "offline-running"]);
    view.run('filter = "attention"');
    assert.deepEqual(ids(), ["paused", "idle", "offline-idle"]);
    view.run('filter = "all"; query = "same"');
    assert.deepEqual(ids(), ["a", "b"]);
    view.run('query = "no matches"');
    assert.deepEqual(ids(), []);
  });

  test(`${mode} sorting follows state/online changes without classifying unknown as idle`, () => {
    const view = inspector();
    for (const state of ["RUNNING", "PREPARE", "SLICING", "PAUSE", "FAILED", "UNKNOWN", "", "FUTURE_STATE", null]) {
      view.run(`sort = ${JSON.stringify(mode)}; snapshot.printers = [
        {id:"active", name:"Z", online:true, state:${JSON.stringify(state)}},
        {id:"idle", name:"A", online:true, state:"IDLE"},
        {id:"offline", name:"0", online:false, state:"RUNNING"}
      ]`);
      assert.equal(view.run('visible()[0].id'), "active");
      assert.equal(view.run('visible()[2].id'), "offline");
      view.run('snapshot.printers[0].state = "FINISH"');
      assert.equal(view.run('visible()[0].id'), mode === "state" ? "active" : "idle");
      view.run('snapshot.printers[0].online = false');
      assert.equal(view.run('visible()[0].id'), "idle");
      view.run('snapshot.printers[0].online = true; snapshot.printers[0].state = "RUNNING"');
      assert.equal(view.run('visible()[0].id'), "active");
    }
  });
}
