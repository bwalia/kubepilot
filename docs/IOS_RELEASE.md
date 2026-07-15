# iOS Release Pipeline (GitHub Actions + fastlane + Vault)

Builds, signs, and ships the native **KubePilot** iOS app from `ios/`.

| Stage | How | When |
|-------|-----|------|
| **TestFlight (internal)** | `workflow_dispatch` → target **testflight** (default) | Everyday QA builds |
| **App Store review** | `workflow_dispatch` → target **app_store** | After TestFlight sign-off |
| **Tag release** | Push `v*.*.*` tag | Formal version cuts (TestFlight only) |

CI runs on the self-hosted Mac Studio runner with labels `[self-hosted, macOS, ios_cicd]`. Signing material is **not** stored in GitHub — it is loaded from Vault and wiped after each run.

## Flow

```
Run workflow (or push v1.2.3 tag)
        │
        ▼
  Load Vault secrets (ASC API key + dist cert private key)
        │
        ▼
  Resolve version (tag → input → ios/project.yml MARKETING_VERSION)
  Build number = latest TestFlight build for version + 1
        │
        ▼
  fastlane prepare_signing  → temp keychain + profiles (app + widget)
  fastlane build_ipa        → xcodegen + archive + export
        │
        ├── testflight → upload_to_testflight (internal only)
        └── app_store  → upload + deliver(submit_for_review: true)
```

## One-time setup

### 1. App Store Connect

1. Create app **KubePilot** with bundle id `io.kubepilot.app`
2. Register App ID `io.kubepilot.app.widgets` for the widget extension
3. Generate an **App Store Connect API** key (App Manager role)
4. Create / reuse an **Apple Distribution** certificate tied to a private key

### 2. Vault secret

Store at **`secret/kubepilot/ios`** (KV v2):

| Key | Description |
|-----|-------------|
| `ASC_KEY_ID` | API key id |
| `ASC_ISSUER_ID` | Issuer id |
| `ASC_PRIVATE_KEY_B64` | base64 of `AuthKey_*.p8` |
| `CERT_PRIVATE_KEY_B64` | base64 of distribution private key `.pem` |
| `APPLE_TEAM_ID` | 10-character team id |
| `APP_STORE_APP_ID` | (optional) numeric ASC app id |

See `ios/ci/ios.vault.env.example`.

### 3. Self-hosted runner

The Mac Studio runner needs:

- Xcode 16+, XcodeGen, CocoaPods (if added later)
- Vault token file (default `VAULT_TOKEN_FILE` in workflow)
- Label `ios_cicd`

### 4. GitHub workflow

Workflow file: `.github/workflows/ios_release.yml`

**TestFlight:**

1. Actions → **iOS Release (TestFlight & App Store)**
2. Run workflow → target **testflight**
3. Version blank = `MARKETING_VERSION` from `ios/project.yml`

**App Store (after QA):**

1. Same workflow → target **app_store**
2. Submits the built binary for review (`automatic_release: false` — you release manually in ASC when approved)

## Local dry run (on the Mac Studio)

```bash
cd ios
export VAULT_TOKEN_FILE=$HOME/.secrets/acc-vault/login-token.json
eval "$(./ci/load-ios-vault-secrets.sh)"
export BUILD_KEYCHAIN_PATH=/tmp/kp-ios.keychain-db
export BUILD_KEYCHAIN_PASSWORD=$(openssl rand -base64 24)
export EXPORT_OPTIONS_PLIST=/tmp/ExportOptions.plist
export VERSION_NAME=1.0.0
bundle install
bundle exec fastlane ios ci_build_number   # note build number
export BUILD_NUMBER=<from output>
bundle exec fastlane ios prepare_signing
bundle exec fastlane ios build_ipa
bundle exec fastlane ios beta
```

## Files

| Path | Purpose |
|------|---------|
| `.github/workflows/ios_release.yml` | Release workflow |
| `ios/fastlane/Fastfile` | Lanes: `ci_build_number`, `prepare_signing`, `build_ipa`, `beta`, `release` |
| `ios/ci/load-ios-vault-secrets.sh` | Vault → env + key files |
| `ios/Gemfile` | fastlane dependency |
| `.github/workflows/ios.yml` | PR CI (simulator build + test) |

## Versioning

- **Marketing version** — `MARKETING_VERSION` in `ios/project.yml`, overridable in workflow dispatch
- **Build number** — auto-incremented per marketing version from TestFlight (`latest + 1`)

Bump `MARKETING_VERSION` in `project.yml` before a new App Store version line.
