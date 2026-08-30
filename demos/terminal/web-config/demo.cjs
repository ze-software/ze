#!/usr/bin/env node
"use strict";

const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");
const { chromium } = require("playwright-core");

const run = "/src/tmp/terminal-demos/bin/ze-demo";
const artifactDir = "/src/demos/terminal/artifacts";
const videoDir = "/src/tmp/terminal-demos/browser-video";
const output = path.join(artifactDir, "web-config.webm");
const poster = path.join(artifactDir, "web-config.png");
const speedup = Number.parseFloat(process.env.ZE_DEMO_SPEEDUP || "1");

function pause(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds / speedup));
}

(async () => {
  fs.rmSync(videoDir, { recursive: true, force: true });
  fs.mkdirSync(videoDir, { recursive: true });
  execFileSync(run, ["run", "web-config", "start"], { stdio: "inherit" });

  let browser;
  try {
    browser = await chromium.launch({
      executablePath: "/usr/bin/chromium",
      headless: true,
      args: ["--no-sandbox", "--disable-dev-shm-usage"],
    });
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1680, height: 1008 },
      recordVideo: { dir: videoDir, size: { width: 1680, height: 1008 } },
      colorScheme: "dark",
    });
    const page = await context.newPage();
    const video = page.video();

    await page.setContent(`<!doctype html><html><head><style>
      html,body{margin:0;width:100%;height:100%;background:#1d1133;color:#e6d9f2;font:15px system-ui,sans-serif}
      main{height:100%;display:grid;place-content:center;padding:49px;box-sizing:border-box}
      h1{font-size:34px;margin:0 0 17px;color:#65f0bc}p{line-height:1.5;max-width:574px}.steps{color:#94d3ff}
    </style></head><body><main><h1>Configure Ze in the browser</h1>
      <p>Change a YANG-backed setting, review the generated diff, commit it, and verify the active value.</p>
      <p class="steps">Local Ze daemon · HTTPS · No external service</p></main></body></html>`);
    await pause(8000);

    await page.goto("https://127.0.0.1:8443/", { waitUntil: "domcontentloaded" });
    await page.fill("#username", "admin");
    await pause(1000);
    await page.fill("#password", "secret123");
    await pause(1400);
    await Promise.all([
      page.waitForURL(/\/show\//),
      page.click("button[type=submit]"),
    ]);
    await pause(3500);

    await page.goto("https://127.0.0.1:8443/show/system/identity/", { waitUntil: "domcontentloaded" });
    await pause(3500);
    const field = page.locator("#field-hostname");
    await field.fill("edge-demo");
    await pause(2400);
    const save = page.locator("#workbench-form-save");
    await save.click();
    await page.waitForSelector("#commit-bar.visible", { timeout: 10000 });
    await pause(3500);

    const [diffResponse] = await Promise.all([
      page.waitForResponse((response) => {
        const url = new URL(response.url());
        return (
          url.pathname === "/config/diff" &&
          url.search === "" &&
          response.request().method() === "GET"
        );
      }),
      page.click("#commit-review-btn"),
    ]);
    if (!diffResponse.ok()) {
      const detail = (await diffResponse.text()).trim() || diffResponse.url();
      throw new Error(
        `/config/diff returned HTTP ${diffResponse.status()}: ${detail}`,
      );
    }
    await page.waitForSelector("#diff-modal.open .diff-content");
    await pause(6000);
    await page.click('#diff-modal button:has-text("Confirm Commit")');
    await page.waitForFunction(() => !document.querySelector("#commit-bar.visible"));
    await pause(3500);

    await page.goto("https://127.0.0.1:8443/show/system/identity/", { waitUntil: "domcontentloaded" });
    await page.waitForFunction(() => {
      const input = document.querySelector("#field-hostname");
      return input && input.value === "edge-demo";
    });
    // Unlink before write, for the reason newCastWriter states in
    // internal/le/terminaldemo/pty.go: renderDemo deletes the previous artifact
    // from the HOST, and a virtiofs guest keeps its own dentry for that name, so
    // a write that reuses the name resolves to the inode the host removed.
    fs.rmSync(poster, { force: true });
    await page.screenshot({ path: poster });
    await pause(8000);

    await context.close();
    const recorded = await video.path();
    fs.rmSync(output, { force: true });
    fs.copyFileSync(recorded, output);
  } finally {
    if (browser) {
      await browser.close();
    }
    execFileSync(run, ["run", "web-config", "stop"], { stdio: "inherit" });
  }
})().catch((error) => {
  console.error(error.stack || error);
  process.exit(1);
});
