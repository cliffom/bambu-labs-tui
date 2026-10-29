// Real-browser layout checks against serve --demo; see docs/development/contributing.md.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const {chromium} = require("playwright");

(async () => {
  const url = process.env.DEMO_URL || "http://127.0.0.1:8080";
  const seed = await (await fetch(new URL("/api/snapshot", url), {
    signal: AbortSignal.timeout(5000),
  })).json();
  assert.equal(seed.demo, true, "use a synthetic demo server");
  assert.ok(seed.printers.length, "demo needs a printer to clone");
  const browser = await chromium.launch({
    headless: true,
    ...(process.env.BROWSER_PATH ? {executablePath: process.env.BROWSER_PATH} : {}),
  });
  try {
    for (const [width, height, scale] of [
      [1920, 1080, 1], [1280, 800, 1], [390, 844, 1],
      [320, 568, 1], [780, 844, 2], [390, 844, 2],
    ]) {
      for (const count of [0, 1, 24]) {
        const page = await browser.newPage({viewport: {width, height}});
        page.setDefaultTimeout(10000);
        const printers = Array.from({length: count}, (_, i) => ({
          ...seed.printers[i % seed.printers.length],
          id: `layout-${i}`, name: `Layout printer ${i + 1}`,
        }));
        await page.route("**/api/snapshot", route => route.fulfill({json: {
          ...seed, at: new Date().toISOString(), printers,
          summary: {
            total: count, printing: printers.filter(p => p.state === "RUNNING").length,
            attention: printers.filter(p => p.attention).length,
            unavailable: printers.filter(p => !p.fresh).length,
          },
        }}));
        await page.goto(url);
        await page.evaluate(scale => {
          document.documentElement.style.fontSize = `${16 * scale}px`;
        }, scale);
        assert.equal(await page.locator("html").evaluate(el => getComputedStyle(el).fontSize),
          `${16 * scale}px`);
        await page.waitForFunction(count =>
          document.querySelectorAll(".card").length === count &&
          document.querySelector("#result-count").textContent.includes(`of ${count}`), count,
          {timeout: 10000});
        await page.waitForFunction(() => document.querySelector("#clock").textContent,
          null, {timeout: 10000});

        async function check(state) {
          const bounds = await page.evaluate(() => {
            const shell = document.querySelector(".shell");
            const footer = document.querySelector("footer");
            const rect = footer.getBoundingClientRect();
            const border = getComputedStyle(footer);
            return {
              bottom: rect.bottom, top: rect.top, left: rect.left, right: rect.right,
              height: innerHeight, width: innerWidth,
              workspaceBottom: shell.getBoundingClientRect().bottom,
              pageOverflow: document.documentElement.scrollWidth > innerWidth,
              workspaceOverflow: shell.scrollWidth > shell.clientWidth,
              footerOverflow: footer.scrollWidth > footer.clientWidth,
              borderWidth: border.borderTopWidth, borderStyle: border.borderTopStyle,
              borderColor: border.borderTopColor,
              sidebarBorder: getComputedStyle(document.querySelector("aside")).borderRightColor,
              clock: document.querySelector("#clock").textContent,
              resultBottom: document.querySelector(".fleet-bottom").getBoundingClientRect().bottom,
            };
          });
          assert.ok(Math.abs(bounds.bottom - height) <= 1, `${state}: footer at viewport bottom`);
          assert.equal(bounds.left, 0);
          assert.equal(bounds.right, width);
          assert.ok(bounds.workspaceBottom <= bounds.top + 1, `${state}: workspace reserves footer space`);
          assert.equal(bounds.pageOverflow || bounds.workspaceOverflow || bounds.footerOverflow, false,
            `${state}: no horizontal overflow`);
          assert.equal(bounds.borderWidth, "1px");
          assert.equal(bounds.borderStyle, "solid");
          assert.equal(bounds.borderColor, bounds.sidebarBorder);
          assert.ok(bounds.clock, "clock remains visible");
          return bounds;
        }
        await check("initial");
        // Scroll the actual browser container to its end without smooth-scroll timing.
        await page.locator(".shell").evaluate(el => { el.scrollTop = el.scrollHeight; });
        const bottom = await check("scrolled");
        assert.ok(bottom.resultBottom <= bottom.top + 1, "last content remains above footer");
        const scroll = await page.locator(".shell").evaluate(el => el.scrollTop);
        await page.evaluate(() => poll());
        assert.equal(await page.locator(".shell").evaluate(el => el.scrollTop), scroll,
          "poll preserves workspace scroll");

        if (count) {
          await page.locator(".card").last().focus();
          await page.keyboard.press("Enter");
          await page.locator("#detail").waitFor({state: "visible"});
          const dialog = await page.locator("#detail").evaluate(el => ({
            top: el.getBoundingClientRect().top, bottom: el.getBoundingClientRect().bottom,
            overflow: el.scrollWidth > el.clientWidth,
            focused: el.contains(document.activeElement),
          }));
          assert.ok(dialog.top >= 0 && dialog.bottom <= height && !dialog.overflow && dialog.focused,
            "printer dialog fits and owns keyboard focus");
          await page.keyboard.press("Escape");
          assert.equal(await page.locator(".card").last().evaluate(el => el === document.activeElement), true);
          await page.locator("#search").fill("no matching layout printer");
          await page.waitForFunction(() => document.querySelectorAll(".card").length === 0);
          await check("filtered empty");
          await page.locator("#search").fill("");
          await page.waitForFunction(count => document.querySelectorAll(".card").length === count, count);
        }
        for (const [button, dialog] of [["settings", "settings-dialog"], ["about", "about-dialog"]]) {
          await page.locator(`#${button}`).focus();
          await page.keyboard.press("Enter");
          await page.locator(`#${dialog}`).waitFor({state: "visible"});
          await page.keyboard.press("Escape");
          assert.equal(await page.locator(`#${button}`).evaluate(el => el === document.activeElement), true);
        }
        await check("after dialogs");
        if (process.env.SCREENSHOT_DIR) {
          fs.mkdirSync(process.env.SCREENSHOT_DIR, {recursive: true});
          await page.locator(".shell").evaluate(el => { el.scrollTop = el.scrollHeight; });
          await page.evaluate(() => new Promise(resolve =>
            requestAnimationFrame(() => requestAnimationFrame(resolve))));
          await page.screenshot({path: path.join(process.env.SCREENSHOT_DIR,
            `status-${width}-${height}-${scale}x-${count}.png`)});
        }
        console.log(`PASS ${width}x${height} ${scale * 100}% text: ${count} printers, scroll/poll/filter/dialog/focus`);
        await page.close();
      }
    }
    console.log(`Browser: ${await browser.version()}`);
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
