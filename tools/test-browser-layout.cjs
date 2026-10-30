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
      [1280, 240, 2],
    ]) {
      for (const count of [0, 1, 24]) {
        const page = await browser.newPage({viewport: {width, height}});
        const pageErrors = [];
        page.on("pageerror", error => pageErrors.push(error.message));
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
        assert.equal(await page.title(), "Bambu Stand · Monitoring");
        assert.equal((await page.locator(".brand").innerText()).replace(/\s/g, ""), "BambuStand");
        await page.evaluate(scale => {
          document.documentElement.style.fontSize = `${16 * scale}px`;
        }, scale);
        assert.equal(await page.locator("html").evaluate(el => getComputedStyle(el).fontSize),
          `${16 * scale}px`);
        await page.waitForFunction(count =>
          document.querySelectorAll(".card").length === count &&
          document.querySelector("#result-count").textContent === `${count} ${count === 1 ? "printer" : "printers"}`, count,
          {timeout: 10000});
        const repo = page.getByRole("link", {name: "Bambu Stand on GitHub (opens in a new tab)", exact: true});
        assert.equal(await repo.getAttribute("href"), "https://github.com/cliffom/bambustand");
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
        const settleLayout = () => page.evaluate(() => new Promise(resolve =>
          requestAnimationFrame(() => requestAnimationFrame(resolve))));
        if (count === 24) {
          await page.evaluate(() => { scrollTo(0, 0); document.querySelector(".shell").scrollTop = 0; });
          await settleLayout();
          const expanded = await page.locator("header").evaluate(el => el.getBoundingClientRect().height);
          for (const distanceFromBottom of [0, 1, 12]) {
            const beforeCollapse = await page.locator(".shell").evaluate((el, distance) => {
              el.scrollTop = el.scrollHeight - el.clientHeight - distance;
              return {
                scroll: el.scrollTop,
                resultTop: document.querySelector("#result-count").getBoundingClientRect().top,
                pageFits: document.documentElement.scrollHeight <= innerHeight + 1,
              };
            }, distanceFromBottom);
            await settleLayout();
            assert.equal(await page.locator(".shell").evaluate(el => el.scrollTop), beforeCollapse.scroll,
              `first scroll ${distanceFromBottom}px from bottom survives intermediate header measurement`);
            assert.ok(Math.abs(await page.locator("#result-count").evaluate(el => el.getBoundingClientRect().top) -
              beforeCollapse.resultTop) < 1, "collapse near the bottom keeps visible content steady");
            if (beforeCollapse.pageFits) {
              assert.ok(Math.abs(await page.locator(".shell").evaluate(el => el.scrollHeight - el.clientHeight - el.scrollTop) -
                distanceFromBottom) <= 1, "collapse retains the distance from the bottom");
            }
            await page.locator(".shell").evaluate(el => { el.scrollTop = 0; });
            await settleLayout();
            assert.equal(await page.locator("header").evaluate(el => el.getBoundingClientRect().height), expanded);
          }
          const pageRange = await page.evaluate(() => document.documentElement.scrollHeight - innerHeight);
          if (pageRange > 1) {
            for (const pageFraction of [0.5, 1]) {
              for (const workspaceEnd of [false, true]) {
                await page.evaluate(() => { document.querySelector(".shell").scrollTop = 0; scrollTo(0, 0); });
                await settleLayout();
                await page.evaluate(fraction => scrollTo(0,
                  (document.documentElement.scrollHeight - innerHeight) * fraction), pageFraction);
                await settleLayout();
                const beforeCollapse = await page.locator(".shell").evaluate((el, end) => {
                  el.scrollTop = end ? el.scrollHeight - el.clientHeight : 48;
                  return {
                    page: scrollY, combined: scrollY + el.scrollTop,
                    resultTop: document.querySelector("#result-count").getBoundingClientRect().top,
                  };
                }, workspaceEnd);
                assert.ok(beforeCollapse.page > 0, "outer page is scrolled before the header collapses");
                await settleLayout();
                const afterCollapse = await page.locator(".shell").evaluate(el => ({
                  combined: scrollY + el.scrollTop,
                  resultTop: document.querySelector("#result-count").getBoundingClientRect().top,
                }));
                assert.ok(Math.abs(afterCollapse.combined - beforeCollapse.combined) <= 1,
                  `collapse with page ${pageFraction * 100}% scrolled preserves the combined page/workspace offset`);
                assert.ok(Math.abs(afterCollapse.resultTop - beforeCollapse.resultTop) <= 1,
                  "collapse with a scrolled outer page keeps workspace content steady");
                const beforeExpand = await page.locator(".shell").evaluate(el => {
                  el.scrollTop = 12;
                  return {
                    combined: scrollY + el.scrollTop,
                    resultTop: document.querySelector("#result-count").getBoundingClientRect().top,
                  };
                });
                await settleLayout();
                assert.equal(await page.locator("header.compact").count(), 0);
                assert.ok(await page.locator(".shell").evaluate((el, before) =>
                  Math.abs(scrollY + el.scrollTop - before.combined) <= 1 &&
                  Math.abs(document.querySelector("#result-count").getBoundingClientRect().top - before.resultTop) <= 1,
                beforeExpand), "expanding the header preserves combined offset and content position");
              }
            }
            await page.evaluate(() => { document.querySelector(".shell").scrollTop = 0; scrollTo(0, 0); });
            await settleLayout();
          }
          // Crossing the collapse threshold must preserve the introduction's
          // viewport position and the combined page/workspace scroll range.
          const before = await page.locator(".title-row").evaluate(el => el.getBoundingClientRect().top);
          const range = await page.locator(".shell").evaluate(el => el.scrollHeight - el.clientHeight + document.documentElement.scrollHeight - innerHeight);
          await page.locator(".shell").evaluate(el => { el.scrollTop = 48; });
          await settleLayout();
          assert.ok(await page.locator("header").evaluate(el => el.getBoundingClientRect().height) < expanded,
            "scroll compacts the header");
          assert.ok(Math.abs(await page.locator(".title-row").evaluate(el => el.getBoundingClientRect().top) - (before - 48)) < 1,
            "collapse preserves the visible content position");
          assert.equal(await page.locator(".shell").evaluate(el => el.scrollTop), 48);
          assert.ok(Math.abs(await page.locator(".shell").evaluate(el => el.scrollHeight - el.clientHeight + document.documentElement.scrollHeight - innerHeight) - range) < 1,
            "collapse preserves scroll range");
          assert.ok(await page.locator(".toolbar").evaluate(el =>
            el.getBoundingClientRect().top > document.querySelector(".shell").getBoundingClientRect().top),
            "toolbar remains in flow before reaching the header");
          await page.locator(".shell").evaluate(el => { el.scrollTop = 900; });
          await settleLayout();
          const pinned = await page.locator(".shell").evaluate(el => !el.classList.contains("toolbar-scrolls"));
          if (height >= 800 && scale === 1) assert.equal(pinned, true, "roomy workspaces pin the toolbar");
          if (height <= 320) assert.equal(pinned, false, "short windows let the toolbar scroll");
          if (pinned) {
            assert.ok(await page.locator(".toolbar").evaluate(el => Math.abs(
              el.getBoundingClientRect().top - document.querySelector("header").getBoundingClientRect().bottom) < 1),
              "toolbar pins immediately beneath the compact header");
            await page.locator("#search").focus();
            await page.keyboard.type("Layout printer");
            await page.locator("#sort").selectOption("state");
            const scroll = await page.locator(".shell").evaluate(el => el.scrollTop);
            await page.evaluate(() => poll());
            await settleLayout();
            assert.equal(await page.locator("#search").inputValue(), "Layout printer");
            assert.equal(await page.locator(".shell").evaluate(el => el.scrollTop), scroll,
              "pinned search/sort and polling retain scroll");
            await page.locator(".card").first().focus();
            await settleLayout();
            assert.ok(await page.locator(".card").first().evaluate(el => el.getBoundingClientRect().top >=
              document.querySelector(".toolbar").getBoundingClientRect().bottom),
              "keyboard-focused card heading is not covered by the toolbar");
            await page.locator("#search").fill("");
            await page.locator("#sort").selectOption("name");
          } else {
            assert.equal(await page.locator(".toolbar").evaluate(el => getComputedStyle(el).position), "static",
              "small workspace lets the toolbar scroll to leave cards reachable");
          }
          await page.locator(".shell").evaluate(el => { el.scrollTop = 0; });
          await settleLayout();
          assert.equal(await page.locator("header").evaluate(el => el.getBoundingClientRect().height), expanded,
            "return to top restores expanded header");
          assert.equal(await page.locator("header.compact").count(), 0);
          // Resize while collapsed; the measured offset must follow wrapping.
          await page.locator(".shell").evaluate(el => { el.scrollTop = 400; });
          await settleLayout();
          const beforeResize = await page.locator(".title-row").evaluate(el => el.getBoundingClientRect().top);
          await page.setViewportSize({width: width >= 760 ? 390 : 1280, height});
          await settleLayout();
          await page.setViewportSize({width, height});
          await settleLayout();
          await check("resized while compact");
          assert.ok(Math.abs(await page.locator(".title-row").evaluate(el => el.getBoundingClientRect().top) - beforeResize) < 1,
            "resize across toolbar/header wrapping restores content position");
          await page.locator(".shell").evaluate(el => { el.scrollTop = 0; });
          await settleLayout();
        }
        if (count === 24 && width === 1280 && height === 800) {
          await page.evaluate(() => {
            document.activeElement?.blur();
            document.querySelector(".shell").scrollTop = 1800;
          });
          await settleLayout();
          const scroll = await page.locator(".shell").evaluate(el => el.scrollTop);
          for (const selector of ["#search", "#sort", '[data-filter="all"]']) {
            await page.locator(selector).click();
            if (selector === "#sort") await page.keyboard.press("Escape");
            await settleLayout();
            assert.equal(await page.locator(".shell").evaluate(el => el.scrollTop), scroll,
              `${selector}: clicking a visible pinned control preserves card position`);
          }
          await page.locator("#search").focus();
          await settleLayout();
          assert.equal(await page.locator(".shell").evaluate(el => el.scrollTop), scroll,
            "keyboard focus on pinned search preserves card position");
          await page.locator("#search").blur();
          const anchor = await page.locator(".card").evaluateAll(cards => {
            const toolbarBottom = document.querySelector(".toolbar").getBoundingClientRect().bottom;
            const shellBottom = document.querySelector(".shell").getBoundingClientRect().bottom;
            const card = cards.find(el => {
              const r = el.getBoundingClientRect();
              return r.top >= toolbarBottom && r.top < shellBottom;
            });
            return {id: card.dataset.id, top: card.getBoundingClientRect().top};
          });
          const earlierIDs = await page.locator(".card").evaluateAll(cards => cards.slice(0, 3).map(el => {
            if (el.getBoundingClientRect().bottom >= document.querySelector(".shell").getBoundingClientRect().top)
              throw new Error("telemetry reflow fixture must change a row entirely above the viewport");
            return el.dataset.id;
          }));
          const earlier = printers.filter(p => earlierIDs.includes(p.id)).map(p => ({...p}));
          for (const p of printers.filter(p => earlierIDs.includes(p.id)))
            Object.assign(p, {fresh: false, connection: "Stale"});
          await page.evaluate(() => poll());
          await settleLayout();
          assert.ok(await page.locator(`[data-id="${earlier[0].id}"] .telemetry-note`).count(),
            "earlier cards grow to display last-known warnings");
          assert.ok(Math.abs(await page.locator(`[data-id="${anchor.id}"]`).evaluate(el =>
            el.getBoundingClientRect().top) - anchor.top) < 1,
            "telemetry reflow above the viewport preserves visible card position");
          for (const p of earlier) Object.assign(printers.find(printer => printer.id === p.id), p);
          await page.evaluate(() => poll());
          await settleLayout();
          assert.ok(Math.abs(await page.locator(`[data-id="${anchor.id}"]`).evaluate(el =>
            el.getBoundingClientRect().top) - anchor.top) < 1,
            "removing earlier last-known warnings preserves visible card position");
          // Equal-height active cards isolate focus restoration from normal
          // scroll anchoring when a telemetry update changes row heights.
          const mixedPrinters = printers.map(p => ({...p}));
          const uniform = {...printers[2], state: "RUNNING", attention: false, conditions: []};
          for (const p of printers) Object.assign(p, uniform, {id: p.id, name: p.name});
          await page.evaluate(() => poll());
          await page.locator("#sort").selectOption("state");
          const movingPrinter = printers[10];
          const movingCard = page.locator(`[data-id="${movingPrinter.id}"]`);
          const movingNode = await movingCard.elementHandle();
          await movingCard.focus();
          await settleLayout();
          await page.locator(".shell").evaluate(el => { el.scrollTop = 2200; });
          await settleLayout();
          assert.ok(await movingCard.evaluate(el => el.getBoundingClientRect().bottom <
            document.querySelector(".shell").getBoundingClientRect().top),
            "focused printer has been scrolled above the viewport before telemetry reorders it");
          const beforeReorder = await page.locator(".shell").evaluate(el => el.scrollTop);
          const oldIndex = await movingCard.evaluate(el => [...el.parentElement.children].indexOf(el));
          movingPrinter.state = "FAILED";
          await page.evaluate(() => poll());
          await settleLayout();
          assert.ok(await movingCard.evaluate(el => [...el.parentElement.children].indexOf(el)) < oldIndex,
            "telemetry moves the focused printer earlier in State sorting");
          assert.equal(await movingNode.evaluate(el => el.isConnected && el === document.activeElement), true,
            "telemetry restores focus to the same card after moving its DOM node");
          assert.equal(await page.locator(".shell").evaluate(el => el.scrollTop), beforeReorder,
            "telemetry focus restoration preserves the scrolled viewport");
          movingPrinter.state = "RUNNING";
          await page.evaluate(() => poll());
          await settleLayout();
          assert.equal(await page.locator(".shell").evaluate(el => el.scrollTop), beforeReorder,
            "restoring the printer state also preserves the scrolled viewport");
          await page.keyboard.press("Tab");
          await settleLayout();
          assert.ok(await page.evaluate(() => document.activeElement.classList.contains("card") &&
            document.activeElement.getBoundingClientRect().top >= document.querySelector(".toolbar").getBoundingClientRect().bottom),
            "subsequent keyboard navigation still brings the card clear of the pinned toolbar");
          for (let i = 0; i < printers.length; i++) Object.assign(printers[i], mixedPrinters[i]);
          await page.evaluate(() => poll());
          await page.locator("#sort").selectOption("name");
          await page.locator(".shell").evaluate(el => { el.scrollTop = 0; });
          await settleLayout();
        }
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
        for (const printer of printers.filter(p => !p.online)) {
          const card = page.locator(`[data-id="${printer.id}"]`);
          assert.equal(await card.locator(".job, .progress-label, .progress-track, .metrics, .card-bottom, .telemetry-note").count(), 0);
          assert.equal(await card.locator(".pill").textContent(), "Offline");
          assert.equal(await card.evaluate(el => getComputedStyle(el).borderTopStyle), "dashed");
          assert.equal(await card.locator(".printer-view").evaluate(el => getComputedStyle(el).filter), "grayscale(1)");
        }
        if (count === 24 && width === 1280 && height === 800) {
          const printer = printers[5];
          const card = page.locator(`[data-id="${printer.id}"]`);
          const cardNode = await card.elementHandle();
          const image = await card.locator("img").elementHandle();
          await card.focus();
          for (const online of [true, false, true, false]) {
            Object.assign(printer, {online, fresh: online, connection: online ? "Live" : "Offline"});
            await page.evaluate(() => poll());
            assert.equal(await cardNode.evaluate(el => el.isConnected && el === document.activeElement), true,
              "online/offline transitions preserve card identity and focus through reordering");
            assert.equal(await image.evaluate(el => el.isConnected), true,
              "online/offline transitions preserve the connected sprite");
            assert.equal(await card.locator(".metrics").count(), online ? 1 : 0);
          }
          await page.keyboard.press("Enter");
          await page.locator("#detail").waitFor({state: "visible"});
          assert.match(await page.locator("#detail-body").textContent(), /Last-known/);
          await page.keyboard.press("Escape");
          // Feed and cloud outages retain explicitly offline cards while online
          // devices keep the full last-known layout.
          for (const failure of ["network", "cloud"]) {
            outage = failure === "network";
            connected = false;
            await page.evaluate(() => poll());
            assert.equal(await card.locator(".pill").textContent(), "Offline");
            assert.equal(await page.locator('[data-id="layout-0"] .metrics').count(), 1);
            outage = false;
            connected = true;
            await page.evaluate(() => poll());
          }
        }
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
        await settleLayout();
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
        assert.deepEqual(pageErrors, [], "layout produces no browser script errors");
        await page.close();
      }
    }
    console.log(`Browser: ${await browser.version()}`);
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
