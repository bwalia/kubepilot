#!/usr/bin/env node
/**
 * Build static blog HTML for pages.kubepilot.org from:
 *   1) local post packs under docs/blog/posts/<slug>/{meta.json,body.html}
 *   2) published posts from opsapi public CMS (when the namespace exists)
 *
 * Opsapi wins on slug collision (CMS is the live catalogue once seeded).
 * Local-only posts still ship so the blog works before the kubepilot namespace exists.
 *
 * Usage:
 *   node scripts/cms/build-blog.mjs
 *   node scripts/cms/build-blog.mjs --out docs/blog
 *   OPSAPI_URL=… CMS_NAMESPACE=kubepilot node scripts/cms/build-blog.mjs
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "../..");
const BLOG_DIR = path.join(ROOT, "docs/blog");
const POSTS_DIR = path.join(BLOG_DIR, "posts");
const CONFIG = JSON.parse(fs.readFileSync(path.join(BLOG_DIR, "config.json"), "utf8"));

const outArg = process.argv.indexOf("--out");
const OUT_DIR = path.resolve(ROOT, outArg >= 0 ? process.argv[outArg + 1] : "docs/blog");

const OPSAPI = (process.env.OPSAPI_URL || CONFIG.opsapiUrl || "").replace(/\/?$/, "/");
const NS = process.env.CMS_NAMESPACE || CONFIG.namespace || "kubepilot";
const SITE_URL = (CONFIG.siteUrl || "https://pages.kubepilot.org").replace(/\/$/, "");

function esc(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function ensureDir(d) {
  fs.mkdirSync(d, { recursive: true });
}

function loadLocalPosts() {
  if (!fs.existsSync(POSTS_DIR)) return [];
  const posts = [];
  for (const slug of fs.readdirSync(POSTS_DIR)) {
    const dir = path.join(POSTS_DIR, slug);
    if (!fs.statSync(dir).isDirectory()) continue;
    const metaPath = path.join(dir, "meta.json");
    const bodyPath = path.join(dir, "body.html");
    if (!fs.existsSync(metaPath) || !fs.existsSync(bodyPath)) {
      console.warn(`skip ${slug}: need meta.json + body.html`);
      continue;
    }
    const meta = JSON.parse(fs.readFileSync(metaPath, "utf8"));
    const status = meta.status || "published";
    if (status !== "published") continue;
    posts.push({
      source: "local",
      title: meta.title,
      slug: meta.slug || slug,
      excerpt: meta.excerpt || "",
      content_html: fs.readFileSync(bodyPath, "utf8"),
      published_at: meta.published_at || "",
      reading_minutes: meta.reading_minutes || estimateMinutes(fs.readFileSync(bodyPath, "utf8")),
      author_name: meta.author_name || CONFIG.authorDefault || "KubePilot",
      tags: (meta.tags || []).map((t) => (typeof t === "string" ? { name: t, slug: slugify(t) } : t)),
      featured_image_url: meta.featured_image_url || null,
      seo_title: meta.seo_title || meta.title,
      seo_description: meta.seo_description || meta.excerpt || "",
      seo_keywords: meta.seo_keywords || "",
    });
  }
  return posts;
}

function slugify(text) {
  return String(text || "")
    .toLowerCase()
    .replace(/[^\w]+/g, "-")
    .replace(/^-|-$/g, "");
}

function estimateMinutes(html) {
  const text = html.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
  return Math.max(1, Math.round(text.split(/\s+/).length / 220));
}

async function fetchOpsapiPosts() {
  if (!OPSAPI || process.env.SKIP_OPSAPI === "1") return [];
  const posts = [];
  try {
    for (let page = 1, total = 1; page <= total; page++) {
      const url = `${OPSAPI}api/v2/public/cms/${NS}/posts?page=${page}&perPage=50`;
      const res = await fetch(url, { headers: { Accept: "application/json" } });
      if (!res.ok) {
        if (res.status === 404) {
          console.warn(`opsapi namespace "${NS}" not found — building from local posts only`);
          return [];
        }
        console.warn(`opsapi list failed (${res.status}) — local posts only`);
        return [];
      }
      const body = await res.json();
      total = body.meta?.totalPages || 1;
      for (const summary of body.data || []) {
        const detailRes = await fetch(
          `${OPSAPI}api/v2/public/cms/${NS}/posts/${encodeURIComponent(summary.slug)}`,
          { headers: { Accept: "application/json" } },
        );
        if (!detailRes.ok) continue;
        const detail = await detailRes.json();
        const p = detail.data || detail;
        posts.push({
          source: "opsapi",
          title: p.title,
          slug: p.slug,
          excerpt: p.excerpt || "",
          content_html: p.content_html || "",
          published_at: (p.published_at || "").slice(0, 10),
          reading_minutes: p.reading_minutes || estimateMinutes(p.content_html || ""),
          author_name: p.author_name || CONFIG.authorDefault || "KubePilot",
          tags: p.tags || [],
          featured_image_url: p.featured_image_url || null,
          seo_title: p.seo_title || p.title,
          seo_description: p.seo_description || p.excerpt || "",
          seo_keywords: p.seo_keywords || "",
        });
      }
    }
  } catch (err) {
    console.warn(`opsapi fetch error: ${err.message} — local posts only`);
    return [];
  }
  return posts;
}

function mergePosts(local, remote) {
  const bySlug = new Map();
  for (const p of local) bySlug.set(p.slug, p);
  for (const p of remote) bySlug.set(p.slug, p); // opsapi wins
  return [...bySlug.values()].sort((a, b) => String(b.published_at).localeCompare(String(a.published_at)));
}

function formatDate(iso) {
  if (!iso) return "";
  const d = new Date(iso.includes("T") ? iso : `${iso}T00:00:00Z`);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString("en-GB", { year: "numeric", month: "short", day: "numeric" });
}

function sharedHead({ title, description, canonical, keywords }) {
  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>${esc(title)}</title>
    <meta name="description" content="${esc(description)}" />
    ${keywords ? `<meta name="keywords" content="${esc(keywords)}" />` : ""}
    <link rel="canonical" href="${esc(canonical)}" />
    <meta name="theme-color" content="#F7F9FC" />
    <meta name="color-scheme" content="light dark" />
    <script>
      (function(){try{var t=localStorage.getItem('kubepilot-theme');if(t==='dark'||t==='light'){document.documentElement.setAttribute('data-theme',t);}}catch(e){}})();
    </script>
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;600&family=IBM+Plex+Sans:wght@400;500;600;700&display=swap" rel="stylesheet" />
    <link rel="stylesheet" href="assets/blog.css" />
  </head>`;
}

function nav(active) {
  return `
    <header class="nav">
      <div class="wrap nav-inner">
        <a href="../index.html" class="logo" aria-label="KubePilot home"><span class="logo-text">Kube<span>Pilot</span></span></a>
        <nav class="nav-links">
          <a href="../index.html#modes">Modes</a>
          <a href="../index.html#features">Features</a>
          <a href="../index.html#install">Install</a>
          <a href="./index.html"${active === "blog" ? ' aria-current="page"' : ""}>Blog</a>
          <a href="https://github.com/bwalia/kubepilot#readme">Docs</a>
        </nav>
        <div class="nav-actions">
          <button type="button" class="theme-toggle" id="theme-toggle" aria-label="Toggle theme">
            <span class="label">Daylight</span>
          </button>
          <a href="https://github.com/bwalia/kubepilot" class="btn btn-gh">GitHub</a>
        </div>
      </div>
    </header>`;
}

function footer() {
  return `
    <footer class="footer">
      <div class="wrap">
        KubePilot &copy; 2025&ndash;2026 &middot;
        <a href="./index.html">Blog</a> &middot;
        <a href="../privacy.html">Privacy</a> &middot;
        <a href="https://github.com/bwalia/kubepilot">GitHub</a>
      </div>
    </footer>
    <script src="assets/theme.js"></script>`;
}

function renderIndex(posts) {
  const cards = posts.length
    ? posts
        .map(
          (p) => `
        <a class="post-card" href="./${esc(p.slug)}.html">
          <div class="tags">${(p.tags || [])
            .map((t) => `<span class="tag">${esc(t.name || t)}</span>`)
            .join("")}</div>
          <h2>${esc(p.title)}</h2>
          <p>${esc(p.excerpt)}</p>
          <div class="meta">
            <span>${esc(formatDate(p.published_at))}</span>
            <span>&middot;</span>
            <span>~${esc(p.reading_minutes)} min read</span>
            <span class="read">Read article &rarr;</span>
          </div>
        </a>`,
        )
        .join("\n")
    : `<p class="empty">No published posts yet. Add a pack under <code>docs/blog/posts/</code> or publish via opsapi.</p>`;

  return `${sharedHead({
    title: "Blog | KubePilot",
    description:
      "Guides and deep-dives on Kubernetes operations, MLOps, GitOps, and AI-assisted troubleshooting from the KubePilot team.",
    canonical: `${SITE_URL}/blog/`,
  })}
  <body>
    ${nav("blog")}
    <main>
      <section class="head">
        <div class="wrap">
          <span class="eyebrow">KubePilot Blog</span>
          <h1>Kubernetes, MLOps &amp; AI Ops</h1>
          <p>Practical guides on running Kubernetes and machine learning in production — GitOps, troubleshooting, and AI-assisted operations.</p>
        </div>
      </section>
      <section class="wrap posts">${cards}</section>
    </main>
    ${footer()}
  </body>
</html>
`;
}

function renderPost(p) {
  const tags = (p.tags || [])
    .map((t) => `<span class="tag">${esc(t.name || t)}</span>`)
    .join("");
  return `${sharedHead({
    title: p.seo_title || `${p.title} | KubePilot Blog`,
    description: p.seo_description || p.excerpt,
    canonical: `${SITE_URL}/blog/${p.slug}.html`,
    keywords: p.seo_keywords,
  })}
  <body>
    ${nav("blog")}
    <main>
      <article class="wrap article">
        <a class="back" href="./index.html">&larr; All posts</a>
        <div class="tags">${tags}</div>
        <h1>${esc(p.title)}</h1>
        <div class="meta">
          <span>${esc(p.author_name)}</span>
          <span>&middot;</span>
          <span>${esc(formatDate(p.published_at))}</span>
          <span>&middot;</span>
          <span>~${esc(p.reading_minutes)} min read</span>
        </div>
        ${p.featured_image_url ? `<img class="hero-img" src="${esc(p.featured_image_url)}" alt="" />` : ""}
        <div class="prose">
${p.content_html}
        </div>
      </article>
    </main>
    ${footer()}
  </body>
</html>
`;
}

async function main() {
  ensureDir(OUT_DIR);
  ensureDir(path.join(OUT_DIR, "assets"));

  // Copy shared assets next to generated HTML
  const assetsSrc = path.join(BLOG_DIR, "assets");
  for (const f of ["blog.css", "theme.js"]) {
    const src = path.join(assetsSrc, f);
    if (fs.existsSync(src)) fs.copyFileSync(src, path.join(OUT_DIR, "assets", f));
  }

  const local = loadLocalPosts();
  const remote = await fetchOpsapiPosts();
  const posts = mergePosts(local, remote);
  console.log(`Building ${posts.length} posts (local=${local.length}, opsapi=${remote.length}) → ${OUT_DIR}`);

  fs.writeFileSync(path.join(OUT_DIR, "index.html"), renderIndex(posts));
  for (const p of posts) {
    fs.writeFileSync(path.join(OUT_DIR, `${p.slug}.html`), renderPost(p));
  }

  // Drop stale generated HTML that is not a current slug and not index
  for (const f of fs.readdirSync(OUT_DIR)) {
    if (!f.endsWith(".html") || f === "index.html") continue;
    const slug = f.replace(/\.html$/, "");
    if (!posts.some((p) => p.slug === slug)) {
      // Keep nothing from the old hand-written dark theme once regenerated
      if (f === "kubeflow-argocd-mlops.html") continue; // regenerated above
    }
  }

  console.log("Done.");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
