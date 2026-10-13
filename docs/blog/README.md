# KubePilot Blog (pages.kubepilot.org/blog)

Static blog for [pages.kubepilot.org](https://pages.kubepilot.org/), sourced from local post packs and optionally synced with the **opsapi CMS** — same authoring pattern as `workstation-website`.

## Why this shape

GitHub Pages cannot call opsapi from the browser (CORS). So:

1. You write / edit a post under `docs/blog/posts/<slug>/`
2. Optionally publish it to opsapi with `scripts/cms/publish-post.mjs`
3. CI (or you locally) runs `scripts/cms/build-blog.mjs`, which merges **local posts + opsapi posts** into static HTML under `docs/blog/`
4. The Landing Pages workflow deploys `docs/blog/` next to the landing page

## Add a post (local → site)

```bash
mkdir -p docs/blog/posts/my-new-post
```

`docs/blog/posts/my-new-post/meta.json`:

```json
{
  "title": "My post title",
  "slug": "my-new-post",
  "excerpt": "One or two sentences for the index card.",
  "published_at": "2026-10-13",
  "reading_minutes": 8,
  "author_name": "Balinder Walia",
  "tags": ["Kubernetes", "SRE"],
  "seo_title": "My post title | KubePilot Blog",
  "seo_description": "Short SEO description.",
  "seo_keywords": "Kubernetes, KubePilot",
  "status": "published"
}
```

`docs/blog/posts/my-new-post/body.html` — HTML fragment (headings, paragraphs, pre/code, embeds). No full page chrome.

Build and preview:

```bash
node scripts/cms/build-blog.mjs
open docs/blog/index.html
```

Commit the post pack **and** the regenerated `docs/blog/index.html` + `docs/blog/<slug>.html`. Push to `master` (paths under `docs/blog/**`) to publish via GitHub Pages.

## Publish to opsapi (CMS)

Create a `kubepilot` namespace in the opsapi dashboard (or set `CMS_NAMESPACE`). Mint an API key with a `cms` scope.

```bash
export OPSAPI_URL=https://int-opsapi.workstation.co.uk/
export OPSAPI_API_KEY='…'
export CMS_NAMESPACE=kubepilot   # default from docs/blog/config.json

# one post
node scripts/cms/publish-post.mjs docs/blog/posts/my-new-post

# all local packs
node scripts/cms/publish-post.mjs --all

# draft only
STATUS=draft node scripts/cms/publish-post.mjs docs/blog/posts/my-new-post
```

Idempotent: matching slugs are updated. After publishing in the CMS UI (or via this script with `status: published`), rebuild:

```bash
node scripts/cms/build-blog.mjs
```

Opsapi posts that are not present locally are pulled into the static site on build.

## Layout

| Path | Role |
|------|------|
| `config.json` | opsapi URL + namespace defaults |
| `posts/<slug>/meta.json` + `body.html` | source of truth for new posts |
| `index.html`, `<slug>.html` | **generated** — do not hand-edit |
| `../landing/` | marketing home; Blog nav points here |

## Deploy

`.github/workflows/landing-pages.yml` builds the blog, then copies landing + blog + screenshots to GitHub Pages (`https://pages.kubepilot.org/`).
