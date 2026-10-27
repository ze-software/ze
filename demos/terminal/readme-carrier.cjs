#!/usr/bin/env node
"use strict";

// Render the verified cast with the same vendored player as the website.
// No Ze session is started and no terminal text is synthesized here.
const fs = require("fs");
const path = require("path");
const http = require("http");
const crypto = require("crypto");
const { execFileSync } = require("child_process");
const { chromium } = require("playwright-core");
const input = JSON.parse(fs.readFileSync("/output/input.json", "utf8"));
const output = "/output";
const fps = 2;

function measure(name) {
  const data = fs.readFileSync(path.join(output, name));
  return { path: name, bytes: data.length, sha256: crypto.createHash("sha256").update(data).digest("hex") };
}

(async () => {
  const routes = new Map([
    ["/hero.cast", ["/output/hero.cast", "application/octet-stream"]],
    ["/player.js", ["/source/website/assets/vendor/asciinema-player.min.js", "text/javascript"]],
    ["/player.css", ["/source/website/assets/vendor/asciinema-player.css", "text/css"]],
  ]);
  const server = http.createServer((request, response) => {
    if (request.url === "/") {
      response.setHeader("Content-Type", "text/html");
      response.end(`<!doctype html><html><head><link rel="stylesheet" href="/player.css"><style>
        html,body{margin:0;background:#121314}#player{width:1680px}
        .ap-terminal{font-family:"Fira Code",monospace!important}
        </style></head><body><div id="player"></div><script src="/player.js"></script></body></html>`);
      return;
    }
    const route = routes.get(request.url);
    if (!route) { response.writeHead(404); response.end(); return; }
    response.setHeader("Content-Type", route[1]);
    fs.createReadStream(route[0]).on("error", (error) => response.destroy(error)).pipe(response);
  });
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  let browser;
  const frames = path.join(output, "frames");
  fs.mkdirSync(frames);
  try {
    browser = await chromium.launch({ executablePath: "/usr/bin/chromium", headless: true,
      args: ["--no-sandbox", "--disable-dev-shm-usage"] });
    const page = await browser.newPage({ viewport: { width: 1680, height: 1200 }, deviceScaleFactor: 1 });
    await page.goto(`http://127.0.0.1:${server.address().port}/`, { waitUntil: "load" });
    await page.evaluate(async () => {
      window.player = AsciinemaPlayer.create("/hero.cast", document.getElementById("player"), {
        controls: false, autoPlay: false, preload: true, fit: "width", terminalFontFamily: "Fira Code",
      });
      await window.player.play();
      window.player.pause();
      await document.fonts.ready;
    });
    const duration = await page.evaluate(() => window.player.getDuration());
    if (!Number.isFinite(duration) || duration <= 0 || duration > 3600) {
      throw new Error(`cast duration ${duration} is outside (0, 3600] seconds`);
    }
    const count = Math.ceil(duration * fps);
    const terminal = page.locator("#player");
    let dimensions;
    const unique = new Map();
    const sequence = [];
    for (let frame = 0; frame < count; frame++) {
      await page.evaluate(async (time) => {
        await window.player.seek(time);
        await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      }, frame / fps);
      const png = await terminal.screenshot({ path: path.join(frames, `${String(frame).padStart(6, "0")}.png`) });
      if (!dimensions) dimensions = { width: png.readUInt32BE(16), height: png.readUInt32BE(20) };
      const digest = crypto.createHash("sha256").update(png).digest("hex");
      if (!unique.has(digest)) unique.set(digest, { id: unique.size, png });
      sequence.push(unique.get(digest).id);
    }
    // Prefer a held opening frame over the empty terminal at time zero.
    const thumbnailFrame = Math.min(count - 1, 3 * fps);
    fs.copyFileSync(path.join(frames, `${String(thumbnailFrame).padStart(6, "0")}.png`), path.join(output, "hero.png"));
    const svg = fs.openSync(path.join(output, "hero.svg"), "wx");
    try {
      fs.writeSync(svg, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="${dimensions.width}" height="${dimensions.height}" viewBox="0 0 ${dimensions.width} ${dimensions.height}"><defs>`);
      for (const { id, png } of unique.values()) {
        fs.writeSync(svg, `<image id="f${id}" width="${dimensions.width}" height="${dimensions.height}" xlink:href="data:image/png;base64,${png.toString("base64")}"/>`);
      }
      fs.writeSync(svg, "</defs>");
      // A non-looping sequence holds the last frame, exactly as the site player does.
      for (let frame = 0; frame < count; frame++) {
        const hold = frame === count - 1 ? "indefinite" : `${1 / fps}s`;
        fs.writeSync(svg, `<use xlink:href="#f${sequence[frame]}" visibility="hidden"><set attributeName="visibility" to="visible" begin="${frame / fps}s" dur="${hold}" fill="remove"/></use>`);
      }
      fs.writeSync(svg, "</svg>\n");
    } finally { fs.closeSync(svg); }
    const videoArgs = ["-nostdin", "-v", "error", "-framerate", String(fps), "-i", path.join(frames, "%06d.png"),
      "-t", String(duration), "-vf", "pad=ceil(iw/2)*2:ceil(ih/2)*2", "-c:v", "libx264", "-preset", "slow", "-crf", "18",
      "-pix_fmt", "yuv420p", "-movflags", "+faststart", path.join(output, "hero.mp4")];
    execFileSync("ffmpeg", videoArgs, { timeout: 1800000, stdio: "inherit" });
    const probe = JSON.parse(execFileSync("ffprobe", ["-v", "error", "-select_streams", "v:0", "-show_entries",
      "stream=width,height,codec_name:format=duration,size", "-of", "json", path.join(output, "hero.mp4")], { encoding: "utf8" }));
    const measurements = {
      ...input, "duration-seconds": duration, "frames-per-second": fps, ...dimensions,
      "thumbnail-seconds": thumbnailFrame / fps,
      renderer: { chromium: browser.version(), playwright: require("playwright-core/package.json").version,
        ffmpeg: execFileSync("ffmpeg", ["-version"], { encoding: "utf8" }).split("\n")[0],
        "video-arguments": videoArgs, "player-sha256": crypto.createHash("sha256").update(fs.readFileSync(routes.get("/player.js")[0])).digest("hex") },
      video: probe, assets: { svg: measure("hero.svg"), video: measure("hero.mp4"), thumbnail: measure("hero.png") },
      "github-observation": "unobserved", choice: "unselected",
    };
    fs.writeFileSync(path.join(output, "carrier.json"), JSON.stringify(measurements, null, 2) + "\n", { flag: "wx" });
  } finally {
    if (browser) await browser.close();
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    fs.rmSync(frames, { recursive: true, force: true });
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
