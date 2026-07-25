import { createWriteStream, existsSync, mkdirSync } from "node:fs";
import { pipeline } from "node:stream/promises";
import path from "node:path";

const FONTS_DIR = path.resolve("static/fonts");
if (!existsSync(FONTS_DIR)) mkdirSync(FONTS_DIR, { recursive: true });

const fonts = [
  {
    name: "Inter-Regular.woff2",
    url: "https://fonts.gstatic.com/s/inter/v13/UcCO3FwrK3iLTeHuS_fvQtMwCp50KnMw2boKoduKmMEVuLyfAZ9hiA.woff2",
  },
  {
    name: "Inter-Medium.woff2",
    url: "https://fonts.gstatic.com/s/inter/v13/UcCO3FwrK3iLTeHuS_fvQtMwCp50KnMw2boKoduKmMEVuI6fAZ9hiA.woff2",
  },
  {
    name: "Inter-SemiBold.woff2",
    url: "https://fonts.gstatic.com/s/inter/v13/UcCO3FwrK3iLTeHuS_fvQtMwCp50KnMw2boKoduKmMEVuGKYAZ9hiA.woff2",
  },
  {
    name: "Inter-Bold.woff2",
    url: "https://fonts.gstatic.com/s/inter/v13/UcCO3FwrK3iLTeHuS_fvQtMwCp50KnMw2boKoduKmMEVuFuYAZ9hiA.woff2",
  },
  {
    name: "MaterialSymbolsOutlined.woff2",
    url: "https://fonts.gstatic.com/s/materialsymbolsoutlined/v168/kJEhBvYX7BgnkSrUwT8OhrdQw4oELdPIeeII9v6oFsLjBuVY.woff2",
  },
];

for (const font of fonts) {
  const dest = path.join(FONTS_DIR, font.name);
  if (existsSync(dest)) {
    console.log(`skip (exists): ${font.name}`);
    continue;
  }
  console.log(`fetching: ${font.name}`);
  const res = await fetch(font.url, { redirect: "follow" });
  if (!res.ok) throw new Error(`failed ${font.name}: ${res.status}`);
  await pipeline(res.body, createWriteStream(dest));
  console.log(`  ok: ${dest}`);
}
console.log("done");
