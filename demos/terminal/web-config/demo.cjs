#!/usr/bin/env node
"use strict";

const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");
const { chromium } = require("playwright-core");

const root = process.env.ZE_DEMO_ROOT || "/src";
const run = path.join(root, "tmp/terminal-demos/bin/ze-demo");
const artifactDir = path.join(root, "demos/terminal/artifacts");
const poster = path.join(artifactDir, "web-config.webp");
const speedup = Number.parseFloat(process.env.ZE_DEMO_SPEEDUP || "1");

function pause(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds / speedup));
}

// Only the native runner calls this mode, after the shared authored journey and
// actual render settling. The endpoint belongs to that runner's isolated browser.
async function capture(endpoint) {
  const browser = await chromium.connectOverCDP(endpoint);
  try {
    const pages = browser.contexts().flatMap((context) => context.pages());
    const matches = pages.filter((page) => {
      return new URL(page.url()).pathname === "/show/system/identity/";
    });
    if (matches.length !== 1) {
      throw new Error(`expected one owned identity readback page, found ${matches.length}`);
    }
    const page = matches[0];
    // Unlink before write for the virtiofs dentry reason in newCastWriter.
    fs.rmSync(poster, { force: true });
    const cdp = await page.context().newCDPSession(page);
    try {
      const screenshot = await cdp.send("Page.captureScreenshot", {
        format: "webp",
        quality: 100,
        captureBeyondViewport: false,
      });
      fs.writeFileSync(poster, Buffer.from(screenshot.data, "base64"));
    } finally {
      await cdp.detach();
    }
  } finally {
    // Disconnect the borrowed CDP client; native RunWBCase owns session cleanup.
    await browser.close();
  }
}

async function record() {
  const browser = await chromium.launch({
    executablePath: process.env.AGENT_BROWSER_EXECUTABLE_PATH || "/usr/bin/chromium",
    headless: true,
    args: ["--no-sandbox", "--disable-dev-shm-usage"],
  });
  try {
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1680, height: 1008 },
      colorScheme: "dark",
    });
    const page = await context.newPage();
    await page.setContent(`<!doctype html><html><head><style>
      html,body{margin:0;width:100%;height:100%;background:#1d1133;color:#e6d9f2;font:15px system-ui,sans-serif}
      main{height:100%;display:grid;place-content:center;padding:49px;box-sizing:border-box}
      h1{font-size:34px;margin:0 0 17px;color:#65f0bc}p{line-height:1.5;max-width:574px}.steps{color:#94d3ff}
    </style></head><body><main><h1>Configure Ze in the browser</h1>
      <p>Change a YANG-backed setting, review the generated diff, commit it, and verify the active value.</p>
      <p class="steps">Local Ze daemon · HTTPS · No external service</p></main></body></html>`);
    await pause(8000);
    await context.close();
  } finally {
    await browser.close();
  }
  // No configuration selectors or second step parser live in this script.
  // Validation and recording both parse test/web/recorded-config-save.wb.
  execFileSync(run, ["run", "web-config", "record"], { stdio: "inherit" });
  await pause(8000);
}

(async () => {
  if (process.argv[2] === "capture" && process.argv.length === 4) {
    await capture(process.argv[3]);
    return;
  }
  if (process.argv.length !== 2) {
    throw new Error("usage: demo.cjs [capture <owned-cdp-endpoint>]");
  }
  await record();
})().catch((error) => {
  console.error(error.stack || error);
  process.exit(1);
});
