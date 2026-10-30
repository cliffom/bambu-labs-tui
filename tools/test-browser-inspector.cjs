// Synthetic inspector interactions against serve --demo, using real browser focus/layout.
const fs = require("node:fs");
const path = require("node:path");
const assert = require("node:assert/strict");
const {chromium} = require("playwright");
(async () => {
  const url = process.env.DEMO_URL || "http://127.0.0.1:8080";
  const seed = await (await fetch(new URL("/api/snapshot", url))).json();
  assert.equal(seed.demo, true);
  assert.ok(seed.printers.length);
  const browser = await chromium.launch({headless: true,
    ...(process.env.BROWSER_PATH ? {executablePath: process.env.BROWSER_PATH} : {})});
  try {
    for (const [width, height, scale] of [[1280,800,1], [1920,1080,1], [390,844,1],
      [320,568,1], [568,320,1], [1280,240,1], [320,568,2], [390,844,2],
      [780,844,2], [568,320,2], [1280,240,2]]) {
      const page = await browser.newPage({viewport: {width,height}});
      const errors = [];
      page.on("pageerror", error => errors.push(error.message));
      const printer = {...structuredClone(seed.printers[0]), id: "inspection", name: "Inspection fixture",
        attention: true, conditions: Array.from({length: 12}, (_,i) => ({id: `notice-${i}`,
          text: `Synthetic condition ${i}`, guidance: "Inspect the printer before using Bambu's applications.",
          level: "warning", hidden: false, since: new Date().toISOString()})),
        ams: Array.from({length: 6}, (_,i) => ({label: `AMS ${i + 1}`, humidity: 30, temperature: null,
          slots: [{id: 0, material: "PLA", present: true, color: "AABBCC", remaining: 0},
            {id: 1, material: "", present: false, color: "", remaining: null}]})),
        external: [{id: 0, material: "PETG", present: true, color: "112233", remaining: null}]};
      let decisions = "saved", writes = 0;
      await page.route("**/api/snapshot", route => route.fulfill({json: {...seed,
        at: new Date().toISOString(), decisions, printers: [printer],
        summary: {total:1, printing:1, attention:1, unavailable:0}}}));
      await page.route("**/api/session", route => route.fulfill({json: {token: "synthetic"}}));
      await page.route("**/api/dismiss", route => {
        writes++;
        const {target} = route.request().postDataJSON();
        printer.conditions.find(c => c.id === target).hidden = true;
        decisions = "failed";
        return route.fulfill({json: {}});
      });
      await page.goto(url);
      await page.evaluate(scale => { document.documentElement.style.fontSize = `${16 * scale}px`; }, scale);
      const card = page.locator('[data-id="inspection"]');
      await card.focus();
      await page.keyboard.press("Enter");
      const body = page.locator("#detail-body");
      const tabs = page.getByRole("tab");
      assert.deepEqual(await tabs.allTextContents(), ["Status", "Filament", "Notices"]);
      assert.equal(await page.getByRole("tab", {selected:true}).textContent(), "Status");
      for (const name of ["Status", "Filament", "Notices"]) {
        const tab = page.getByRole("tab", {name, exact:true});
        await tab.click();
        assert.equal(await body.getAttribute("aria-labelledby"), await tab.getAttribute("id"));
        assert.equal(await page.locator('[role="tab"][tabindex="0"]').count(), 1);
        await body.evaluate(el => { el.scrollTop = 100; });
        const before = await body.evaluate(el => el.scrollTop);
        await page.evaluate(() => poll());
        assert.equal(await body.evaluate(el => el.scrollTop), before, `${name}: polling keeps reading position`);
        assert.equal(await tab.evaluate(el => el === document.activeElement), true);
        const bounds = await page.locator("#detail").evaluate(el => {
          const body = el.querySelector('[role="tabpanel"]'), tabs = el.querySelector('[role="tablist"]');
          const close = el.querySelector("#close-detail"), r = el.getBoundingClientRect();
          return {top:r.top, bottom:r.bottom, overflow:el.scrollWidth > el.clientWidth || body.scrollWidth > body.clientWidth,
            panel:body.clientHeight, line:parseFloat(getComputedStyle(body).lineHeight), tabBottom:tabs.getBoundingClientRect().bottom,
            panelTop:body.getBoundingClientRect().top, closeBottom:close.getBoundingClientRect().bottom};
        });
        assert.ok(bounds.top >= 0 && bounds.bottom <= height && !bounds.overflow && bounds.panel >= bounds.line &&
          bounds.tabBottom <= bounds.panelTop && bounds.closeBottom <= height,
          `${width}x${height} ${scale}x ${name}: usable panel and fixed controls ${JSON.stringify(bounds)}`);
        if (process.env.SCREENSHOT_DIR) {
          fs.mkdirSync(process.env.SCREENSHOT_DIR, {recursive: true});
          await page.locator("#detail").screenshot({path: path.join(process.env.SCREENSHOT_DIR,
            `inspector-${width}-${height}-${scale}x-${name}.png`)});
        }
        await body.focus();
        await page.keyboard.press("Control+End");
        await body.evaluate(el => { el.scrollTop = el.scrollHeight; });
        assert.equal(await tab.isVisible(), true);
        if (name === "Filament") {
          assert.match(await body.textContent(), /Empty \(reported\)/);
          assert.match(await body.textContent(), /Remaining: 0%/);
          assert.match(await body.textContent(), /External filament/);
        }
      }
      assert.equal(writes, 0, "tab switching makes no decisions");
      await page.getByRole("tab", {name:"Status", exact:true}).focus();
      await page.keyboard.press("ArrowLeft");
      assert.equal(await page.getByRole("tab", {selected:true}).textContent(), "Notices");
      await page.keyboard.press("Home");
      await page.keyboard.press("ArrowRight");
      assert.equal(await page.getByRole("tab", {selected:true}).textContent(), "Filament");
      await page.keyboard.press("End");
      await page.keyboard.press("Tab");
      assert.equal(await body.evaluate(el => el === document.activeElement), true);
      await page.locator('[data-dismiss="notice-0"]').click();
      await page.waitForFunction(() => document.querySelector("#detail-body").textContent.includes("Applied locally"));
      await page.evaluate(() => poll());
      assert.equal(await page.getByRole("tab", {selected:true}).textContent(), "Notices");
      assert.equal(await page.locator("#close-detail").evaluate(el => el === document.activeElement), true);
      await page.locator("#toggle-hidden-messages").click();
      assert.match(await body.textContent(), /Hidden locally/);
      assert.equal(await page.locator('[data-dismiss="notice-0"][data-retry="true"]').count(), 1);
      await page.getByRole("tab", {name:"Filament", exact:true}).click();
      await page.getByRole("tab", {name:"Notices", exact:true}).click();
      assert.match(await body.textContent(), /Hidden locally/);
      for (const c of printer.conditions) c.hidden = true;
      await page.locator("#toggle-hidden-messages").click();
      await page.evaluate(() => poll());
      assert.match(await body.textContent(), /All reported messages are hidden locally/);
      printer.conditions = [];
      await page.evaluate(() => poll());
      assert.match(await body.textContent(), /No active notices in this snapshot/);
      Object.assign(printer, {fresh:false, online:false, connection:"Offline", ams:[], external:[]});
      await page.evaluate(() => poll());
      await page.getByRole("tab", {name:"Filament", exact:true}).click();
      assert.match(await body.textContent(), /Last-known · AMS.*configuration unknown/s);
      await page.getByRole("tab", {name:"Status", exact:true}).click();
      assert.match(await body.textContent(), /Current state is unconfirmed/);
      await page.keyboard.press("Escape");
      assert.equal(await card.evaluate(el => el === document.activeElement), true);
      await page.keyboard.press("Enter");
      assert.equal(await page.getByRole("tab", {selected:true}).textContent(), "Status");
      assert.equal(await body.evaluate(el => el.scrollTop), 0);
      await page.keyboard.press("Escape");
      await page.locator("#review").click();
      assert.equal(await page.getByRole("tab", {selected:true}).textContent(), "Notices");
      assert.deepEqual(errors, []);
      await page.close();
      console.log(`PASS inspector ${width}x${height} ${scale * 100}% text`);
    }
    console.log(`Browser: ${await browser.version()}`);
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
