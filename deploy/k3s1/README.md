# KubePilot in k3s1

`values.yaml` is the source of truth for the `kubepilot` release in the home cluster (namespace `kubepilot-system`).
Deploy from the repo with it, never with `--set` or `--reuse-values` alone, or settings drift back to whatever was
typed last.

```bash
export KUBECONFIG=~/.kube/k3s1.yaml   # on the Mac Studio
helm upgrade --install kubepilot charts/kubepilot -n kubepilot-system -f deploy/k3s1/values.yaml
```

**Ollama:** it uses KubePilot's own Ollama on the Mac Studio, `192.168.1.177:11438` (`com.ollama.kubepilot`,
installed by `deploy/macstudio/install-runtime.sh`). Never point it at `:11434`: that Ollama serves OpsAPI on every
environment, and KubePilot's 1–3 minute requests there left OpsAPI's assistant timing out everywhere.

**Dashboard login:** `kubepilot-dashboard-auth`, a Secret that Helm doesn't manage, so no password is in git or in
the release. Create it once (or again to change the password), then restart the pod:

```bash
kubectl -n kubepilot-system create secret generic kubepilot-dashboard-auth \
  --from-literal=username=admin --from-literal=password="$(openssl rand -base64 24)" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n kubepilot-system rollout restart deploy/kubepilot
```
