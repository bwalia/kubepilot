# Mac Studio launchd runtime (versioned)

These files are the **source of truth** for how KubePilot runs on the Mac Studio
(`192.168.1.177`, dashboard `:8383`). Every `Deploy → Mac Studio` workflow
installs them into `~/.kubepilot/` so redeploys cannot drift back to a heavy
thinking model.

| File | Purpose |
|------|---------|
| `kubepilot.env` | Pinned interactive LLM + non-secret serve defaults |
| `start-server.sh` | launchd entrypoint; loads env + secrets, passes `--ollama-model` on CLI |
| `secrets.env.example` | Template for local-only password / kubeconfig |
| `install-runtime.sh` | Copies the above into `~/.kubepilot` and prepares Ollama |
| `pin-ollama-model.py` | Legacy helper (prefer `install-runtime.sh`) |

## Interactive model policy

Default: **`llama3.2:3b-dash`** — `llama3.2:3b` with `num_ctx=4096` (see
`Modelfile.llama3.2-3b-dash`). Stock `llama3.2:3b` allocates a huge context and
is too heavy for interactive RCA.

Do **not** set `KUBEPILOT_OLLAMA_MODEL` to large thinking models (`qwen3.8`,
`qwen3-coder:30b`, …) in `kubepilot.env`. Those routinely hang the UI for
minutes. Change the pin by editing `kubepilot.env` in git and redeploying.

## Secrets

`~/.kubepilot/secrets.env` is created once (migrated from the legacy script /
plist) and is **never** overwritten by deploy. Keep the password out of git.
