const {test} = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const path = require("node:path");
const directory = path.join(__dirname, "../internal/ui/themes");
const defaults = JSON.parse(fs.readFileSync(path.join(directory, "default.json"))).colors;
const catalog = () => ({themes: [
  {id: "host", label: "Follow host (configured theme)", colors: {...defaults, background: "#123456", accent: "#ABCDEF"}},
  ...fs.readdirSync(directory).filter(f => f.endsWith(".json")).map(file => {
    const theme = JSON.parse(fs.readFileSync(path.join(directory, file)));
    return {id: file.slice(0, -5), label: theme.name, colors: {...defaults, ...theme.colors}};
  }),
]});
const flush = () => new Promise(resolve => setImmediate(resolve));
async function browser({saved = null, blocked = false} = {}) {
  const elements = new Map(), events = {}, documentEvents = {}, styles = new Map(), storage = new Map();
  if (saved) storage.set("bambu-tui.browser-theme.v1", saved);
  let response = catalog(), fail = false, replacements = 0;
  for (const id of ["theme-select", "theme-status", "settings"]) {
    elements.set("#" + id, {value: "host", disabled: true,
      addEventListener(name, fn) { this[name] = fn; },
      replaceChildren(...options) { this.options = options; replacements++; },
    });
  }
  const document = {hidden: false, activeElement: elements.get("#theme-select"),
    documentElement: {style: {setProperty(k, v) { styles.set(k, v); }}},
    querySelector(s) { return elements.get(s); }, createElement() { return {}; },
    addEventListener(k, fn) { documentEvents[k] = fn; },
  };
  const context = vm.createContext({document, window: {addEventListener(k, fn) { events[k] = fn; }},
    localStorage: {getItem(k) { if (blocked) throw Error(); return storage.get(k); },
      setItem(k, v) { if (blocked) throw Error(); storage.set(k, v); },
      removeItem(k) { if (blocked) throw Error(); storage.delete(k); }},
    fetch: async () => { if (fail) throw Error(); return {ok: true, json: async () => response}; },
    AbortController, setTimeout() {}, clearTimeout() {}, setInterval() {},
  });
  vm.runInContext(fs.readFileSync(path.join(__dirname, "../internal/webserver/assets/themes.js"), "utf8"), context);
  await flush();
  return {styles, storage, document, events,
    select: elements.get("#theme-select"), status: elements.get("#theme-status"),
    choose(id) { this.select.value = id; this.select.change(); },
    async refresh(value) { if (value) response = value; await elements.get("#settings").click(); },
    replacements: () => replacements,
    async failure() { fail = true; await this.refresh(); },
  };
}

test("host custom palette is default; every bundled TUI theme is selectable without changing the host", async () => {
  const view = await browser();
  assert.equal(view.styles.get("--bg"), "#123456");
  assert.equal(view.styles.get("--green"), "#ABCDEF");
  assert.equal(view.select.options.length, catalog().themes.length);
  for (const theme of catalog().themes.slice(1)) {
    view.choose(theme.id);
    assert.equal(view.styles.get("--text"), theme.colors.text);
    assert.equal(view.styles.get("--bg"), theme.colors.background || "#101312");
    assert.equal(view.styles.get("--selected-text"), theme.colors.selected_text);
    assert.equal(view.styles.get("--selected-background"), theme.colors.selected_background);
    assert.equal(view.styles.get("--footer-warning"), theme.colors.footer_warning);
    assert.equal(view.styles.get("--focus"), theme.colors.focus);
  }
  view.choose("host");
  assert.equal(view.storage.size, 0);
  assert.equal(view.styles.get("--green"), "#ABCDEF");
});

test("saved choices reload safely; unknown choices fall back to host", async () => {
  assert.equal((await browser({saved: "hotdog"})).select.value, "hotdog");
  assert.equal((await browser({saved: "removed-or-private-path"})).select.value, "host");
  const view = await browser({blocked: true});
  view.choose("c64");
  assert.equal(view.select.value, "c64");
  assert.match(view.status.textContent, /cannot be saved/);
});

test("host updates preserve focus and options when unchanged; overrides remain independent", async () => {
  const view = await browser();
  await view.refresh();
  assert.equal(view.replacements(), 1);
  assert.equal(view.document.activeElement, view.select);
  const changed = catalog(); changed.themes[0].colors.text = "#FEDCBA";
  await view.refresh(changed);
  assert.equal(view.styles.get("--text"), "#FEDCBA");
  view.choose("c64");
  await view.refresh(catalog());
  assert.equal(view.select.value, "c64");
  assert.equal(view.document.activeElement, view.select);
});

test("unavailable or malformed catalogs preserve applied colors and allow retry", async () => {
  const view = await browser();
  for (const invalid of [{themes: []}, {themes: [{id: "host", label: "x", colors: {text: "red; url(https://example.org)"}}]},
    {themes: [catalog().themes[0], catalog().themes[0]]}]) {
    await view.refresh(invalid);
    assert.equal(view.styles.get("--bg"), "#123456");
    assert.match(view.status.textContent, /unavailable/);
  }
  await view.refresh(catalog());
  assert.match(view.status.textContent, /Following/);
  await view.failure();
  assert.equal(view.select.disabled, false);
  assert.equal(view.styles.get("--bg"), "#123456");
});

test("theme preferences synchronize across tabs including storage reset", async () => {
  const view = await browser();
  view.events.storage({key: "bambu-tui.browser-theme.v1", newValue: "hotdog"});
  assert.equal(view.select.value, "hotdog");
  assert.equal(view.document.documentElement.style.colorScheme, "light");
  view.events.storage({key: null, newValue: null});
  assert.equal(view.select.value, "host");
  assert.equal(view.document.documentElement.style.colorScheme, "dark");
});
