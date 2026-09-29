# Prompt: Build KubePilot Android (Kotlin) — feature parity with iOS

Copy everything below the line into an agent / Cursor chat when scaffolding the Android app.

---

## Mission

Build a **native Kotlin Android companion app** for [KubePilot](https://github.com/bwalia/kubepilot) with **feature parity** to the existing iOS app under `ios/`.

This is **not** a WebView wrapper of the dashboard. Optimise for **incident response on the go**: cluster health → failing pods → logs/events/YAML → AI root cause → executable remediation.

Reference sources of truth in this monorepo (read before inventing APIs or UI):

| Source | Path |
|--------|------|
| iOS architecture & screens | `docs/IOS_APP.md` |
| iOS API surface | `ios/KubePilot/Core/Networking/KubePilotAPI.swift` |
| iOS models (PascalCase + snake_case) | `ios/KubePilot/Core/Models/APIModels.swift` |
| iOS networking | `ios/KubePilot/Core/Networking/APIClient.swift` |
| Demo / App Review fixtures | `ios/KubePilot/Core/Networking/DemoFixtures.swift`, `cmd/demo-api/`, `docs/DEMO_API.md` |
| Brand / theme | `ios/KubePilot/Core/Design/Theme.swift`, https://kubepilot.org |
| Feature screens | `ios/KubePilot/Features/**` |
| Dashboard API mirror | `dashboard/lib/api.ts` |

Place the new app at **`android/`** in the kubepilot repo (sibling of `ios/`).

---

## Product goals (parity checklist)

Engineers must be able to:

1. Connect to a KubePilot server (URL + auth) or **Try demo**
2. See cluster health on one screen
3. Find failing pods (search + filters)
4. Open a pod → overview, logs (live tail), events, YAML, **Analyse with AI**
5. Chat with the AI assistant (`POST /api/v1/ai/interpret`)
6. Browse alerts: events, anomalies, RCA reports
7. Control Autopilot (mode / pause / resume) and run suggested remediations (with CR-code gate when required)
8. Switch kubeconfig context; manage account + biometric app lock in Settings
9. Glance at cluster health from a **home-screen widget**
10. Launch common actions via **App Shortcuts / Google Assistant** equivalents

Success metric (same as iOS): from alert → remediation path in **&lt; 60 seconds**.

---

## Tech stack (required)

| Layer | Choice |
|-------|--------|
| Language | Kotlin 2.x |
| Min SDK | 26+ (prefer 28+); target / compile SDK current stable |
| UI | Jetpack Compose + Material 3 |
| Architecture | MVVM + unidirectional state (ViewModel + UiState) |
| DI | Hilt (or Koin if you document why) |
| Networking | OkHttp + Retrofit (or Ktor) + kotlinx.serialization |
| Persistence | DataStore (prefs) + Room (offline cache) |
| Secrets | EncryptedSharedPreferences or Android Keystore-backed store (never plaintext SharedPreferences) |
| Biometrics | BiometricPrompt + device credential fallback |
| Widgets | Glance (Jetpack Glance) — S/M/L equivalents |
| Shortcuts | App Shortcuts + optional Assistant App Actions |
| Async | coroutines + Flow |
| Tests | JUnit5 / Truth; model decoding tests mirroring `ios/KubePilotTests` |
| Build | Gradle Kotlin DSL, single `:app` module first; split `:core:network`, `:core:model`, `:feature:*` if needed |
| CI | GitHub Actions on `android/**` (assembleDebug + unit tests) |

Do **not** use Flutter/React Native for this prompt.

---

## App structure (mirror iOS)

```
android/
├── README.md
├── settings.gradle.kts
├── app/
│   └── src/main/java/io/kubepilot/app/
│       ├── KubePilotApp.kt
│       ├── MainActivity.kt
│       ├── navigation/          # Root + bottom nav
│       ├── core/
│       │   ├── network/         # ApiClient, KubePilotApi, AuthInterceptor
│       │   ├── model/           # DTOs matching iOS APIModels
│       │   ├── auth/            # Account store, biometric lock
│       │   ├── data/            # Room cache, repositories
│       │   └── design/          # Theme, brand components
│       ├── feature/
│       │   ├── onboarding/
│       │   ├── dashboard/
│       │   ├── pods/
│       │   ├── ai/
│       │   ├── autopilot/
│       │   ├── alerts/
│       │   ├── nodes/
│       │   ├── logs/
│       │   └── settings/
│       └── widget/
└── app/src/test/                # Model + filter tests
```

Navigation: **bottom bar** with tabs matching iOS `AppTab`:

1. Dashboard  
2. Pods  
3. AI  
4. Autopilot  
5. Alerts  
6. Settings  

Root gate (same as iOS `RootView`):

- If biometric lock required → unlock screen  
- Else if authenticated → main tabs  
- Else → onboarding  

Lock when app goes to background if biometric lock enabled.

---

## Design system (brand parity)

Match kubepilot.org / iOS `Theme.swift`:

| Token | Hex |
|-------|-----|
| Background | `#0a0f1c` |
| Surface | `#111827` |
| Surface elevated | `#1f2937` |
| Accent | `#3b82f6` |
| Accent light | `#60a5fa` |
| Purple | `#a78bfa` |
| Success | `#22c55e` |
| Warning | `#fbbf24` |
| Danger | `#f87171` |
| Text primary | `#f1f5f9` |
| Text secondary | `#cbd5e1` |
| Muted | `#94a3b8` |

- Dark-first UI (force dark / dark ColorScheme)
- Gradient accent (blue → purple) for brand marks and primary CTAs
- Cards with soft borders (`white @ 8%`), 16dp large / 10dp small corners
- Health colours: healthy / degraded / critical
- Reusable composables: `SurfaceCard`, filter chip bar, cluster context banner, empty/error/loading states, primary button, brand wordmark
- App id: `io.kubepilot.app` (or `io.kubepilot.android` if Play Console requires split — document choice)
- App name: **KubePilot**

Feel like a first-class Android product (Predictive Back, edge-to-edge, dynamic type / fontScale, TalkBack labels) — not a dashboard clone.

---

## Authentication & onboarding

### Connect form

- Server URL (e.g. `https://kubepilot.example.com` or LAN `http://192.168.x.x:8383`)
- Auth method picker (model all iOS methods; implement the working ones first):
  - **Bearer** token ✅
  - **Basic** username/password ✅
  - OAuth GitHub / GitLab / Google / Microsoft / OIDC — model ready; stub UI “coming soon” unless backend OIDC is wired
- Toggle: **Require biometric unlock on launch**
- **Connect** — `GET /healthz` (or equivalent probe) then persist account
- **Try demo** — connect to `https://demo.kubepilot.org` with Basic `apple` / `review`; if host unreachable, load **offline DemoFixtures** equivalent to iOS (`DemoFixtures.swift` + demo-api fixtures)

### Credential storage

- Store base URL, auth method, token or username/password in encrypted storage
- Send `Authorization: Bearer …` or `Authorization: Basic …` on `/api/v1/*` when dashboard auth is enabled
- Never log secrets

### Biometric unlock

- On cold start / resume from background: BiometricPrompt before showing cluster data
- Passcode/PIN fallback via device credential

---

## Networking contract

Base: `{server}/api/v1/` with `GET /healthz` at server root (same special-case as iOS).

Implement every method in `KubePilotAPI.swift` / `KubePilotService`:

### Health / config

- `GET /healthz`
- `GET /api/v1/version`
- `GET /api/v1/config`
- `GET /api/v1/ai/health`

### Clusters

- `GET /api/v1/clusters/kubeconfigs`
- `POST /api/v1/clusters/switch-context` `{ "context": "..." }`
- `POST /api/v1/clusters/kubeconfigs/base64` `{ name, content_base64 }` (optional for v1 if Settings only switches)

### Resources

- `GET /api/v1/clusters/pods?namespace=`
- `GET /api/v1/clusters/crashing-pods?namespace=`
- `GET /api/v1/clusters/deployments?namespace=`
- `GET /api/v1/clusters/nodes`
- `GET /api/v1/namespaces`
- `GET /api/v1/clusters/services?namespace=`
- `GET /api/v1/events?namespace=&type=&search=&limit=&since=&sort=desc`
- `GET /api/v1/troubleshooting/summary?namespace=`
- `GET /api/v1/clusters/pods/{ns}/{pod}/diagnostics?tail_lines=`
- `GET /api/v1/clusters/pods/{ns}/{pod}/logs?container=&tail=` → **plain text**
- `GET /api/v1/resource/{kind}/{ns}/{name}/yaml` → **plain text**

### AI / RCA / Autopilot / remediation

- `GET /api/v1/ai/troubleshoot/{ns}/{pod}` (timeout **180s**)
- `POST /api/v1/ai/interpret` `{ "command": "..." }` (timeout **180s**)
- `GET /api/v1/rca`, `GET /api/v1/rca/{id}`
- `GET /api/v1/anomalies?namespace=&since=`
- `GET /api/v1/autopilot?limit=`
- `POST /api/v1/autopilot/mode` `{ "mode": "off"|"dry-run"|"active" }`
- `POST /api/v1/autopilot/pause`, `POST /api/v1/autopilot/resume`
- `POST /api/v1/ai/execute-action` `{ action, change_id?, cr_code? }`
- `POST /api/v1/crcode/authorize` `{ change_id, cr_code }`

### JSON casing (critical)

Go returns **mixed casing**:

- Many cluster resource structs: **PascalCase** (`Name`, `Namespace`, `Phase`, …) — see `PodSummary`, `NodeSummary`, `DeploymentSummary`
- Tagged structs: **snake_case** (`first_seen`, `health_summary`, `problem_pods`, …)

Kotlin serializers **must** match iOS `CodingKeys`. Port models from `APIModels.swift` (including helpers: `displayStatus`, `severity`/`healthScore`, `displayRoles`, `allIPs`, `outputText`, `fixPrompt` for AI reports).

AI timeouts: **180 seconds**. Log tail poll interval: **~3s** (same as iOS performance targets).

---

## Screens (feature parity)

### 1. Dashboard

- Overall health score from `troubleshooting/summary`
- Metric cards: crashloop pods, pending pods, not-ready nodes, warning events
- Resource pressure (CPU/memory %) when metrics available
- AI insights list (copy report + “fix with LLM” prompt share — same as iOS `AIReportCopyBar` / `fixPrompt`)
- Problem pods list → navigate to Pod Detail
- Cluster context banner (active kubeconfig context)
- Pull to refresh

### 2. Pods

- Search
- Filter chips: All / CrashLoop / OOM / Pending / Not Ready (match iOS filters)
- Namespace picker
- Row: name, namespace, status/reason, restarts, node, health tint
- Navigate to Pod Detail

### 3. Pod Detail

Tabs or sections:

- Overview (phase, node, conditions, containers, owner chain, labels)
- Logs (container picker, live tail polling ~3s, monospace, share/copy)
- Events
- YAML (fetch resource yaml)
- **Analyse with AI** → `TroubleshootReport` (root cause, analysis, suggested actions)
- Suggested actions UI: execute via API; if `requires_cr_code`, prompt for change ID + CR code then authorize + execute

### 4. AI Assistant

- Chat-style UI
- Sends natural language to `POST /ai/interpret`
- Shows returned `SuggestedAction`s with execute/copy
- Optional: seed from troubleshooting summary / open from insight

### 5. Autopilot

- Show status + policy (mode, confidence, allowed actions, cooldown, namespaces)
- Mode selector: Off / Dry-run / Active (confirm before Active)
- Pause / Resume
- Decision history list (verdict, resource, root cause, confidence)
- **503 = not configured** → friendly empty state (not a hard error) — match iOS

### 6. Alerts

- Segments/tabs: Events | Anomalies | RCA
- Event filters (type Warning/Normal, search, namespace)
- RCA report detail with copy/fix-prompt helpers
- Anomaly list with severity

### 7. Nodes

- Reachable from Dashboard/Settings or list under Settings/Dashboard secondary nav (iOS has `NodesListView`)
- Show ready, pressure flags, capacity, kubelet version
- Role badges (control-plane / worker)
- LAN / WAN / tunnel / internal IP labels (same classification as iOS `NodeIPLabels`)
- Labels list sorted by key

### 8. Settings

- Active account display name + base URL
- Biometric lock toggle
- Cluster / kubeconfig context switcher
- AI health status
- Sign out (clear secrets + caches)
- App version

### 9. Log viewer

- Dedicated full-screen log viewer if navigated from pod (match `LogViewerView`)
- Monospace, search/filter line optional for v1.1

---

## Offline / cache

Room (or equivalent) cache:

- Latest troubleshooting summary
- Recent RCA reports
- Recently viewed pods / namespaces
- Favourite clusters (if modelled)

Stale-while-revalidate on Dashboard/Pods when network fails; DemoFixtures offline path for Try demo.

---

## Widgets & shortcuts

### Glance widgets (S / M / L)

Show: cluster name, health label, failing pods, crashloop count, alerts.  
Prefer live data via App Widget + shared repository (or WorkManager refresh every ~15–30 min). Scaffold with placeholders first if needed — iOS widgets are scaffolded similarly.

### App shortcuts

Parity with iOS App Intents phrases:

1. Open production / dashboard  
2. Show failed pods  
3. Cluster health summary (short text from troubleshooting summary)

---

## Testing

Port iOS unit tests:

- Decode PascalCase pod/node fixtures
- Decode snake_case events / summary
- Pod filter logic
- Node IP / role helpers

Add:

- Auth header construction tests
- Repository / ViewModel smoke tests with fake API

Document how to run against local server:

```bash
./dist/kubepilot serve --dashboard-port=8383
# Android emulator: http://10.0.2.2:8383
# Physical device: http://<lan-ip>:8383
```

Cleartext: allow HTTP only for debug builds / user-entered http URLs (network security config).

---

## CI / release (scaffold)

- `.github/workflows/android.yml`: checkout, JDK 17+, `./gradlew :app:assembleDebug :app:testDebugUnitTest`
- README under `android/` with Android Studio open steps
- Play Store / Fastlane supply: stub docs only for v1 (`docs/ANDROID_RELEASE.md` outline) — do not block app ship on Play pipeline

---

## Implementation order

1. Gradle project + theme + navigation shell  
2. Network layer + models + auth/onboarding (+ Try demo)  
3. Dashboard + Pods + Pod Detail (logs + AI analyse)  
4. AI Assistant + Alerts + Nodes  
5. Autopilot + execute-action / CR code  
6. Settings + biometric lock  
7. Room cache  
8. Glance widgets + shortcuts  
9. Unit tests + CI workflow + `android/README.md`  
10. Update root `README.md` and `docs/IOS_APP.md` (or add `docs/ANDROID_APP.md`) linking the Android app  

---

## Explicit non-goals (v1)

- Not a full topology canvas like the web dashboard  
- Not kubectl exec / shell  
- Not push notifications (roadmap; mirror iOS)  
- Not OAuth full browser flow unless backend endpoints exist  
- Not Wear OS / Live Activities equivalents in v1  

---

## Definition of done

- [ ] All six tabs work against a live KubePilot server  
- [ ] Try demo works online (`demo.kubepilot.org`) and offline fixtures  
- [ ] Mixed JSON casing decodes without crashes (tests green)  
- [ ] AI troubleshoot + interpret + suggested action execute path works  
- [ ] Autopilot 503 empty state + mode controls  
- [ ] Biometric lock + encrypted credentials  
- [ ] Widget + at least one app shortcut  
- [ ] CI builds on GitHub Actions  
- [ ] Visual brand matches iOS / kubepilot.org dark theme  

When in doubt, **match iOS behaviour** in `ios/KubePilot/` rather than inventing new product semantics.
