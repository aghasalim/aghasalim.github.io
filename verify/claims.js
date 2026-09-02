// Independent recomputation of the numbers on the front page, in Node.
//
// Two things happen here. Every phrase in verify/claims.tsv must appear in the
// rendered text of index.html exactly once, and every published value must be
// reproducible from the source values by the stated rule. The page is the only
// place these numbers are written out in prose, so nothing else would notice if
// a hand edit changed one.
//
// Also checks the JSON-LD block with a real JSON parser and requires the
// identity URL in it to match the canonical link and the sitemap entry.

const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const html = fs.readFileSync(path.join(root, "index.html"), "utf8");
const tsv = fs.readFileSync(path.join(root, "verify", "claims.tsv"), "utf8");

let bad = 0;
const fail = (m) => { console.log("FAIL " + m); bad++; };

// Visible text: drop script and style blocks, drop tags, unescape the few
// entities this page uses, collapse whitespace.
const text = html
  .replace(/<script[\s\S]*?<\/script>/gi, " ")
  .replace(/<style[\s\S]*?<\/style>/gi, " ")
  .replace(/<[^>]+>/g, " ")
  .replace(/&amp;/g, "&").replace(/&lt;/g, "<").replace(/&gt;/g, ">")
  .replace(/\s+/g, " ")
  .trim();

const rows = tsv.trim().split("\n").map((l) => l.split("\t"));
const head = rows.shift();
const col = {};
head.forEach((h, i) => { col[h] = i; });
for (const need of ["id", "rule", "a", "b", "published", "phrase"]) {
  if (!(need in col)) { fail("claims.tsv has no column " + need); }
}
if (bad) { process.exit(1); }

const round = (x, n) => {
  const f = Math.pow(10, n);
  return Math.sign(x) * Math.round(Math.abs(x) * f) / f;
};

const recompute = (rule, a, b) => {
  const m = /^(copy|round|ratio|diff)(\d*)$/.exec(rule);
  if (!m) { return null; }
  const n = m[2] === "" ? 0 : parseInt(m[2], 10);
  if (m[1] === "copy") { return a; }
  if (m[1] === "round") { return round(a, n); }
  if (m[1] === "ratio") { return round(a / b, n); }
  return round(a - b, n);
};

let phrases = 0, values = 0;
let worst = 0, worstId = "";
for (const r of rows) {
  const id = r[col.id];
  const phrase = r[col.phrase];
  const occurrences = text.split(phrase).length - 1;
  if (occurrences !== 1) {
    fail(id + ": phrase appears " + occurrences + " times in index.html, want 1: " + phrase);
  } else {
    phrases++;
  }

  const a = Number(r[col.a]);
  const b = r[col.b] === "-" ? NaN : Number(r[col.b]);
  const published = Number(r[col.published]);
  if (!Number.isFinite(a) || !Number.isFinite(published)) {
    fail(id + ": a or published is not a finite number");
    continue;
  }
  const got = recompute(r[col.rule], a, b);
  if (got === null) { fail(id + ": unknown rule " + r[col.rule]); continue; }
  const err = Math.abs(got - published);
  const tol = Math.abs(published) * 1e-12 + 1e-12;
  if (err > tol) {
    fail(id + ": rule " + r[col.rule] + " gives " + got + ", page publishes " + published);
  } else {
    values++;
    if (err > worst) { worst = err; worstId = id; }
  }
}

// JSON-LD, canonical and sitemap must agree on one URL.
const ld = /<script type="application\/ld\+json">([\s\S]*?)<\/script>/.exec(html);
if (!ld) {
  fail("no JSON-LD block in index.html");
} else {
  let obj = null;
  try { obj = JSON.parse(ld[1]); } catch (e) { fail("JSON-LD does not parse: " + e.message); }
  if (obj) {
    const canonical = /<link rel="canonical" href="([^"]+)"/.exec(html);
    const sitemap = fs.readFileSync(path.join(root, "sitemap.xml"), "utf8");
    const loc = /<loc>([^<]+)<\/loc>/.exec(sitemap);
    if (!canonical) { fail("no canonical link"); }
    else if (!loc) { fail("no <loc> in sitemap.xml"); }
    else if (obj.url !== canonical[1] || obj.url !== loc[1]) {
      fail("JSON-LD url " + obj.url + ", canonical " + canonical[1] + ", sitemap " + loc[1] + " do not agree");
    } else {
      console.log("JSON-LD parses; url, canonical and sitemap loc all " + obj.url);
    }
  }
}

if (bad) {
  console.log("JavaScript: " + bad + " failures");
  process.exit(1);
}
console.log("JavaScript: " + phrases + " phrases found once each, " + values +
  " published values reproduced, largest error " +
  (worst === 0 ? "0.0e+00" : worst.toExponential(1) + " on " + worstId));
