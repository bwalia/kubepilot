#!/usr/bin/env node
/**
 * Publish a local blog post pack to the opsapi CMS (same flow as workstation-website).
 *
 * Auth: OPSAPI API key with a `cms` scope (dashboard → API Keys). API keys skip 2FA.
 *
 * Usage:
 *   OPSAPI_API_KEY=… node scripts/cms/publish-post.mjs docs/blog/posts/my-slug
 *   OPSAPI_API_KEY=… node scripts/cms/publish-post.mjs --all
 *   DRY_RUN=1 … STATUS=draft …
 *
 * Env:
 *   OPSAPI_URL        default from docs/blog/config.json
 *   CMS_NAMESPACE     default from config (kubepilot)
 *   CMS_CATEGORY      default blog
 *   STATUS            published|draft (default: meta.status or published)
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "../..");
const CONFIG = JSON.parse(fs.readFileSync(path.join(ROOT, "docs/blog/config.json"), "utf8"));

const BASE = (process.env.OPSAPI_URL || CONFIG.opsapiUrl || "").replace(/\/?$/, "/");
const KEY = process.env.OPSAPI_API_KEY || "";
const NS = process.env.CMS_NAMESPACE || CONFIG.namespace || "kubepilot";
const CAT_SLUG = process.env.CMS_CATEGORY || CONFIG.category || "blog";
const CAT_NAME = process.env.CMS_CATEGORY_NAME || CONFIG.categoryName || "Blog";
const DRY = process.env.DRY_RUN === "1";

if (!KEY && !DRY) {
  console.error("Set OPSAPI_API_KEY (or DRY_RUN=1). Mint a key in the opsapi dashboard → API Keys.");
  process.exit(1);
}

const headers = {
  "Content-Type": "application/json",
  Authorization: `Bearer ${KEY}`,
  "X-Namespace-Slug": NS,
};

async function api(method, pathAndQuery, body) {
  const res = await fetch(`${BASE}${pathAndQuery.replace(/^\//, "")}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let json;
  try {
    json = JSON.parse(text);
  } catch {
    json = { raw: text };
  }
  return { ok: res.ok, status: res.status, json };
}

async function ensureCategory() {
  const list = await api("GET", "api/v2/cms/categories");
  if (!list.ok && list.status === 404) {
    throw new Error(
      `Namespace "${NS}" not found. Create it in the opsapi dashboard, then re-run.`,
    );
  }
  const found = (list.json?.data || []).find((c) => c.slug === CAT_SLUG);
  if (found) return found.uuid;
  if (DRY) {
    console.log(`[dry-run] would create category ${CAT_SLUG}`);
    return "DRY_CAT_UUID";
  }
  const created = await api("POST", "api/v2/cms/categories", {
    name: CAT_NAME,
    slug: CAT_SLUG,
  });
  if (!created.ok) {
    throw new Error(`create category failed (${created.status}): ${JSON.stringify(created.json)}`);
  }
  return created.json?.data?.uuid || created.json?.uuid;
}

async function findBySlug(slug) {
  const res = await api("GET", `api/v2/cms/posts?search=${encodeURIComponent(slug)}&perPage=50`);
  const items = res.json?.data || [];
  return items.find((p) => p.slug === slug) || null;
}

function loadPack(dir) {
  const meta = JSON.parse(fs.readFileSync(path.join(dir, "meta.json"), "utf8"));
  const content_html = fs.readFileSync(path.join(dir, "body.html"), "utf8");
  if (!meta.title || !(meta.slug || path.basename(dir))) {
    throw new Error(`${dir}: meta.json needs title + slug`);
  }
  return { meta, content_html, slug: meta.slug || path.basename(dir) };
}

function payloadFromPack(pack, categoryUuid) {
  const { meta, content_html, slug } = pack;
  const status = process.env.STATUS || meta.status || "published";
  const tags = (meta.tags || []).map((t) => (typeof t === "string" ? t : t.name)).filter(Boolean);
  return {
    title: meta.title,
    slug,
    excerpt: meta.excerpt || "",
    content_html,
    status,
    visibility: "public",
    featured_image_url: meta.featured_image_url || null,
    tags,
    category_uuids: categoryUuid ? [categoryUuid] : [],
    seo_title: meta.seo_title || meta.title,
    seo_description: meta.seo_description || meta.excerpt || "",
    seo_keywords: meta.seo_keywords || "",
    author_name: meta.author_name || CONFIG.authorDefault || "Balinder Walia",
  };
}

async function publishOne(dir, categoryUuid) {
  const pack = loadPack(dir);
  const payload = payloadFromPack(pack, categoryUuid);
  if (DRY) {
    console.log(`[dry-run] would upsert ${payload.slug} status=${payload.status}`);
    return "dry";
  }
  const existing = await findBySlug(payload.slug);
  if (existing?.uuid) {
    const res = await api("PUT", `api/v2/cms/posts/${existing.uuid}`, payload);
    if (!res.ok) throw new Error(`update ${payload.slug}: ${res.status} ${JSON.stringify(res.json)}`);
    console.log(`~ ${payload.slug} (${existing.uuid})`);
    return "updated";
  }
  const res = await api("POST", "api/v2/cms/posts", payload);
  if (!res.ok) throw new Error(`create ${payload.slug}: ${res.status} ${JSON.stringify(res.json)}`);
  console.log(`+ ${payload.slug}`);
  return "created";
}

function resolveTargets(argv) {
  const postsRoot = path.join(ROOT, "docs/blog/posts");
  if (argv.includes("--all")) {
    return fs
      .readdirSync(postsRoot)
      .map((s) => path.join(postsRoot, s))
      .filter((d) => fs.existsSync(path.join(d, "meta.json")));
  }
  const args = argv.filter((a) => !a.startsWith("-"));
  if (!args.length) {
    console.error("Usage: node scripts/cms/publish-post.mjs <post-dir|--all>");
    process.exit(1);
  }
  return args.map((a) => path.resolve(a));
}

async function main() {
  const targets = resolveTargets(process.argv.slice(2));
  console.log(`Publishing ${targets.length} post(s) → ${BASE} (namespace=${NS}, category=${CAT_SLUG})`);
  const categoryUuid = DRY ? "DRY_CAT_UUID" : await ensureCategory();
  let ok = 0;
  let fail = 0;
  for (const dir of targets) {
    try {
      await publishOne(dir, categoryUuid);
      ok++;
    } catch (err) {
      fail++;
      console.error(`✗ ${dir}: ${err.message}`);
    }
  }
  console.log(`\nDone. ok=${ok} failed=${fail}`);
  if (fail) process.exit(1);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
