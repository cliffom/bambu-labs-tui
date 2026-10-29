// Optional real-Chromium theme checks against serve --demo.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const {chromium} = require("playwright");
(async () => {
  const url = process.env.DEMO_URL || "http://127.0.0.1:8080";
  const seed = await (await fetch(new URL("/api/snapshot", url))).json();
  assert.equal(seed.demo, true, "use a synthetic demo server");
  const catalog = await (await fetch(new URL("/api/themes", url))).json();
  const browser = await chromium.launch({headless: true,
    ...(process.env.BROWSER_PATH ? {executablePath: process.env.BROWSER_PATH} : {})});
  let cases = 0;
  try {
    for (const [width, height, scale] of [[1280,800,1],[390,844,1],[320,568,2],[568,320,2]]) {
      const page = await browser.newPage({viewport: {width, height}});
      const errors = []; page.on("pageerror", error => errors.push(error.message));
      const printers = seed.printers.map((p, i) => i ? {...p, fresh: false, online: false, connection: "Offline", attention: true, conditions: [{id: "theme-condition", text: "Synthetic intervention", guidance: "Inspect the printer", level: "warning"}]} : p);
      await page.route("**/api/snapshot", route => route.fulfill({json: {...seed, at: new Date().toISOString(), printers,
        summary: {total: printers.length, printing: printers.filter(p => p.state === "RUNNING").length,
          attention: printers.filter(p => p.attention).length, unavailable: printers.filter(p => !p.fresh).length},
      }}));
      await page.goto(url);
      await page.waitForFunction(() => !document.querySelector("#theme-select").disabled && document.querySelector(".card"));
      await page.evaluate(scale => document.documentElement.style.fontSize = `${16*scale}px`, scale);
      assert.equal(await page.locator("#theme-select").inputValue(), "host");
      const image = await page.locator(".card img").first().elementHandle();
      for (const theme of catalog.themes) {
        await page.locator("#settings").click();
        await page.locator("#theme-select").selectOption(theme.id);
        await page.locator("#theme-select").focus();
        const actual = await page.evaluate(() => {
          const css = getComputedStyle(document.documentElement);
          const body = getComputedStyle(document.body);
          const select = document.querySelector("#theme-select");
          return {bg: css.getPropertyValue("--bg").trim(), text: css.getPropertyValue("--text").trim(),
            focus: css.getPropertyValue("--focus").trim(), selection: css.getPropertyValue("--selected-background").trim(),
            scheme: css.colorScheme, body: body.backgroundColor,
            focused: document.activeElement === select,
            overflow: document.documentElement.scrollWidth > innerWidth ||
              document.querySelector("#settings-dialog").scrollWidth > document.querySelector("#settings-dialog").clientWidth + 1};
        });
        assert.equal(actual.bg, theme.colors.background || "#101312");
        assert.equal(actual.text, theme.colors.text);
        assert.equal(actual.focus, theme.colors.focus);
        assert.equal(actual.selection, theme.colors.selected_background);
        assert.equal(actual.focused, true);
        if (theme.id !== "host") {
          const contrast = await page.evaluate(() => {
            const rgb = value => value.match(/[\d.]+/g).slice(0,3).map(Number).map(v => { v /= 255; return v <= 0.04045 ? v/12.92 : ((v+0.055)/1.055)**2.4; });
            const luminance = value => rgb(value).reduce((sum,v,i) => sum+v*[0.2126,0.7152,0.0722][i],0);
            const ratio = (a,b) => { const x=luminance(a), y=luminance(b); return (Math.max(x,y)+0.05)/(Math.min(x,y)+0.05); };
            const body=getComputedStyle(document.body), selected=getComputedStyle(document.querySelector(".filters .selected"));
            const footer=getComputedStyle(document.querySelector("footer")), live=getComputedStyle(document.querySelector("#feed-state"));
            const warning=getComputedStyle(document.querySelector(".card-note"));
            const hint=getComputedStyle(document.querySelector("#settings-dialog .dialog-footer")), dialog=getComputedStyle(document.querySelector("#settings-dialog"));
            return [ratio(body.color,body.backgroundColor), ratio(selected.color,selected.backgroundColor), ratio(footer.color,footer.backgroundColor), ratio(live.color,footer.backgroundColor),ratio(warning.color,body.backgroundColor), ratio(hint.color,dialog.backgroundColor)];
          });
          assert.ok(contrast.every(ratio => ratio >= 4.5), `${theme.id}: body, selection, footer and warning contrast ${contrast}`);
        }
        assert.equal(actual.overflow, false, `${theme.id} ${width}: settings reflows`);
        if (process.env.SCREENSHOT_DIR) {
          fs.mkdirSync(process.env.SCREENSHOT_DIR, {recursive: true});
          await page.screenshot({path: path.join(process.env.SCREENSHOT_DIR, `${theme.id}-${width}x${height}-${scale}x-settings.png`)});
        }
        await page.keyboard.press("Escape");
        assert.equal(await page.locator("#settings").evaluate(el => el === document.activeElement), true);
        assert.equal(await image.evaluate(el => el.isConnected), true, "theme switching preserves sprite identity");
        assert.equal(await page.locator(".shell").evaluate(el => el.scrollWidth > el.clientWidth), false);
        for (const card of await page.locator(".card.offline").all()) {
          assert.equal(await card.locator(".pill").textContent(), "Offline");
          assert.equal(await card.evaluate(el => getComputedStyle(el).borderTopStyle), "dashed");
          assert.equal(await card.locator(".printer-view").evaluate(el => getComputedStyle(el).filter), "grayscale(1)");
          assert.equal(await card.locator(".metrics, .progress-track, .card-bottom").count(), 0);
          assert.equal(await card.locator(".card-note").count(), 1, "offline attention remains visible");
        }
        await page.locator(".card").first().click();
        assert.equal(await page.locator("#detail").evaluate(el => el.scrollWidth > el.clientWidth + 1), false);
        await page.keyboard.press("Escape");
        await page.evaluate(() => { document.querySelector(".shell").scrollTop = 0; scrollTo(0,0); });
        if (process.env.SCREENSHOT_DIR) {
          await page.screenshot({path: path.join(process.env.SCREENSHOT_DIR, `${theme.id}-${width}x${height}-${scale}x-farm.png`)});
          await page.locator(".card.offline").first().focus();
          await page.screenshot({path: path.join(process.env.SCREENSHOT_DIR, `${theme.id}-${width}x${height}-${scale}x-offline.png`)});
        }
        cases++;
      }
      await page.reload();
      await page.waitForFunction(() => !document.querySelector("#theme-select").disabled);
      assert.equal(await page.locator("#theme-select").inputValue(), catalog.themes.at(-1).id);
      assert.deepEqual(errors, []);
      await page.close();
    }
    console.log(`${cases} theme/layout combinations passed (${await browser.version()})`);
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
