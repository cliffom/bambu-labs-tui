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
      [320, 568, 2], [568, 320, 1], [568, 320, 2], [1280, 240, 1],
      [1920, 1080, 1], [1280, 800, 1], [390, 844, 1],
      [320, 568, 1], [780, 844, 2], [390, 844, 2],
    ]) {
      for (const count of [0, 1, 24]) {
        const page = await browser.newPage({viewport: {width, height}});
        page.setDefaultTimeout(10000);
        const printers = Array.from({length: count}, (_, i) => {
          const state = ["IDLE", "FINISH", "RUNNING", "PAUSE", "UNKNOWN", "IDLE", "FINISH", "FAILED"][i % 8];
          const online = i % 8 !== 5, fresh = online && i % 8 !== 6;
          const attention = i % 8 === 1 || i % 8 === 7;
          return {...seed.printers[i % seed.printers.length],
            id: `layout-${i}`, name: `Layout printer ${i + 1}`, state, online, fresh,
            connection: !online ? "Offline" : !fresh ? "Stale" : "Live",
            idle_finished: fresh && (state === "IDLE" || state === "FINISH"), attention,
            progress: state === "FINISH" ? 100 : state === "IDLE" ? 0 : 42,
            ams: [], external: [], // Exercise readable, untruncated unknown material.
            conditions: attention ? [{id: `condition-${i}`, level: "warning", text: "Synthetic intervention",
              guidance: "Inspect the printer", since: new Date().toISOString()}] : [],
          };
        });
        let outage = false, connected = true;
        await page.route("**/api/snapshot", route => outage ? route.abort("failed") : route.fulfill({json: {
          ...seed, at: new Date().toISOString(), printers, connected,
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
          document.querySelector("#result-count").textContent === `${count} ${count === 1 ? "printer" : "printers"}`, count,
          {timeout: 10000});
        const repo = page.getByRole("link", {name: "Bambu TUI on GitHub (opens in a new tab)", exact: true});
        assert.equal(await repo.getAttribute("href"), "https://github.com/cliffom/bambu-tui");
        assert.equal(await page.locator("#clock").count(), 0);
        await repo.focus();
        assert.equal(await repo.evaluate(el => el === document.activeElement), true);

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
              headerClipped: document.querySelector("header").scrollHeight > document.querySelector("header").clientHeight + 1,
              workspaceHeight: shell.clientHeight,
              minimumWorkspace: Math.min(12 * parseFloat(getComputedStyle(document.documentElement).fontSize), innerHeight / 2),
              pageHeight: document.documentElement.scrollHeight, pageY: scrollY,
              pageOverflow: document.documentElement.scrollWidth > innerWidth,
              workspaceOverflow: shell.scrollWidth > shell.clientWidth,
              footerOverflow: footer.scrollWidth > footer.clientWidth,
              borderWidth: border.borderTopWidth, borderStyle: border.borderTopStyle,
              borderColor: border.borderTopColor,
              headerBorder: getComputedStyle(document.querySelector("header")).borderBottomColor,
              repoVisible: document.querySelector(".footer-repo").getBoundingClientRect().bottom <= innerHeight,
              repoRight: document.querySelector(".footer-repo").getBoundingClientRect().right,
              footerPadding: parseFloat(border.paddingRight),
              resultBottom: document.querySelector("#result-count").getBoundingClientRect().bottom,
            };
          });
          assert.ok(bounds.workspaceHeight >= bounds.minimumWorkspace - 1,
            `${state}: workspace retains a usable scroll area (${bounds.workspaceHeight}px)`);
          assert.equal(bounds.headerClipped, false, `${state}: header fits without covering workspace`);
          if (bounds.pageHeight <= height + 1) {
            assert.ok(Math.abs(bounds.bottom - height) <= 1, `${state}: footer at viewport bottom`);
            assert.ok(bounds.repoVisible, "GitHub link remains visible");
          } else {
            assert.ok(Math.abs(bounds.bottom + bounds.pageY - bounds.pageHeight) <= 1,
              `${state}: short-height footer remains at the scrollable page bottom`);
          }
          assert.equal(bounds.left, 0);
          assert.equal(bounds.right, width);
          assert.ok(bounds.workspaceBottom <= bounds.top + 1, `${state}: workspace reserves footer space`);
          assert.equal(bounds.pageOverflow || bounds.workspaceOverflow || bounds.footerOverflow, false,
            `${state}: no horizontal overflow`);
          assert.equal(bounds.borderWidth, "1px");
          assert.equal(bounds.borderStyle, "solid");
          assert.equal(bounds.borderColor, bounds.headerBorder);
          assert.ok(Math.abs(bounds.repoRight - (width - bounds.footerPadding)) <= 1,
            "GitHub link stays aligned to the right");
          return bounds;
        }
        await check("initial");
        assert.equal(await page.locator("aside, .summary button").count(), 0,
          "toolbar is the only filter control; summary is static");
        assert.equal(await page.locator("footer > #feed-state[role=status]").count(), 1,
          "connection/demo status lives on the left of the footer");
        assert.equal(await page.locator("header #feed-state, .readonly, .brand small").count(), 0);
        assert.match(await page.locator("#feed-state").textContent(), /Demo/);
        assert.ok(await page.locator(".tools .sort-control").evaluate(el => {
          const select = el.querySelector("select"), icon = el.querySelector("svg");
          const box = select.getBoundingClientRect(), arrow = icon.getBoundingClientRect();
          return arrow.left > box.left && arrow.right < box.right - arrow.width / 2 &&
            Math.abs((arrow.top + arrow.bottom) / 2 - (box.top + box.bottom) / 2) < 1 &&
            parseFloat(getComputedStyle(select).paddingInlineEnd) >= arrow.width * 2 &&
            getComputedStyle(icon).pointerEvents === "none";
        }), "sort chevron is centered and inset with reserved text space");
        const expectedCounts = {all: count, printing: printers.filter(p => p.state === "RUNNING").length,
          idle: printers.filter(p => p.idle_finished).length, attention: printers.filter(p => p.attention).length,
          unavailable: printers.filter(p => !p.fresh).length};
        for (const [choice, expected] of Object.entries(expectedCounts))
          assert.equal(await page.locator(`#${choice}-count`).textContent(), String(expected));
        const idle = page.locator('[data-filter="idle"]');
        const idleCount = printers.filter(p => p.idle_finished).length;
        assert.equal(await page.locator("#idle-count").textContent(), String(idleCount));
        await idle.click();
        assert.equal(await idle.getAttribute("aria-pressed"), "true");
        assert.equal(await page.locator(".card").count(), idleCount);
        assert.equal(await page.locator("#result-count").textContent(),
          idleCount === count ? `${count} ${count === 1 ? "printer" : "printers"}` : `${idleCount} of ${count} printers`);
        assert.equal(await page.locator("#idle-filter-help").isVisible(), true);
        await check("idle filter");
        if (count === 24 && width === 1280 && height === 800) {
          const card = page.locator('[data-id="layout-0"]');
          const image = await card.locator("img").elementHandle();
          await card.focus();
          await page.evaluate(() => poll());
          assert.equal(await card.evaluate(el => el === document.activeElement), true);
          assert.ok(await image.evaluate(el => el.isConnected), "unchanged sprite stays connected");
          printers[0].state = "RUNNING";
          printers[0].idle_finished = false;
          await page.evaluate(() => poll());
          assert.equal(await page.locator(".card").count(), idleCount - 1);
          assert.equal(await idle.evaluate(el => el === document.activeElement), true,
            "membership removal returns focus to active filter");
          printers[0].state = "IDLE";
          printers[0].idle_finished = true;
          await page.evaluate(() => poll());
          for (const failure of ["network", "cloud"]) {
            outage = failure === "network";
            connected = false;
            await page.evaluate(() => poll());
            assert.equal(await page.locator(".card").count(), 0);
            assert.equal(await page.locator("#idle-count").textContent(), "0");
            outage = false;
            connected = true;
            await page.evaluate(() => poll());
            assert.equal(await page.locator(".card").count(), idleCount);
          }
          await page.locator("#search").fill("Layout printer 1");
          assert.ok(await page.locator(".card").count() < idleCount);
          await page.locator("#sort").selectOption("attention");
          await page.locator("#search").fill("");
          assert.equal(await page.locator(".card").count(), idleCount);
        }
        if (process.env.SCREENSHOT_DIR) {
          fs.mkdirSync(process.env.SCREENSHOT_DIR, {recursive: true});
          await page.screenshot({fullPage: true, path: path.join(process.env.SCREENSHOT_DIR,
            `idle-${width}-${height}-${scale}x-${count}.png`)});
        }
        await page.locator('[data-filter="all"]').click();
        assert.equal(await page.locator(".card").count(), count);
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
          assert.ok(await page.locator(".card").last().evaluate(el => {
            const r = el.getBoundingClientRect();
            const shell = document.querySelector(".shell").getBoundingClientRect();
            return Math.min(r.bottom, shell.bottom, innerHeight) > Math.max(r.top, shell.top, 0);
          }), "focused printer card is reachable within the workspace and viewport");
          await page.keyboard.press("Enter");
          await page.locator("#detail").waitFor({state: "visible"});
          const dialog = await page.locator("#detail").evaluate(el => ({
            top: el.getBoundingClientRect().top, bottom: el.getBoundingClientRect().bottom,
            overflow: el.scrollWidth > el.clientWidth,
            focused: el.contains(document.activeElement),
          }));
          assert.ok(dialog.top >= 0 && dialog.bottom <= height && !dialog.overflow && dialog.focused,
            `printer dialog fits and owns keyboard focus: ${JSON.stringify(dialog)}`);
          await page.keyboard.press("Escape");
          assert.equal(await page.locator(".card").last().evaluate(el => el === document.activeElement), true);
          await page.locator("#search").click();
          await page.locator("#search").fill("no matching layout printer");
          await page.waitForFunction(() => document.querySelectorAll(".card").length === 0);
          assert.equal(await page.locator("#result-count").textContent(), `0 of ${count} ${count === 1 ? "printer" : "printers"}`);
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
        await repo.focus();
        assert.ok(await repo.evaluate(el => {
          const r = el.getBoundingClientRect();
          return r.top >= 0 && r.bottom <= innerHeight;
        }), "footer repository link is reachable by keyboard in short windows");
        if (process.env.SCREENSHOT_DIR) {
          fs.mkdirSync(process.env.SCREENSHOT_DIR, {recursive: true});
          await page.locator(".shell").evaluate(el => { el.scrollTop = el.scrollHeight; });
          await page.evaluate(() => new Promise(resolve =>
            requestAnimationFrame(() => requestAnimationFrame(resolve))));
          await page.screenshot({fullPage: true, path: path.join(process.env.SCREENSHOT_DIR,
            `status-${width}-${height}-${scale}x-${count}.png`)});
        }
        // Returning to a roomy window restores the pinned footer automatically.
        await page.setViewportSize({width: 1280, height: 800});
        await page.evaluate(() => { document.documentElement.style.fontSize = "16px"; });
        assert.ok(await page.evaluate(() =>
          document.documentElement.scrollHeight <= innerHeight + 1 &&
          Math.abs(document.querySelector("footer").getBoundingClientRect().bottom - innerHeight) <= 1),
        "resize restores viewport-pinned footer without page overflow");
        console.log(`PASS ${width}x${height} ${scale * 100}% text: ${count} printers, scroll/poll/filter/dialog/focus`);
        await page.close();
      }
    }
    console.log(`Browser: ${await browser.version()}`);
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
