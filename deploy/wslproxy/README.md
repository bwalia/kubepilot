# wslproxy edge specs for demo.kubepilot.org

Mirror of the fishers.cloud pattern: one host JSON + one shared routing rule
pointing at the k3s1 traefik-edge entry (`193.237.176.232:8888`). Traefik then
matches `Host: demo.kubepilot.org` to the `kubepilot-demo` Ingress.

Apply with your usual wslproxy admin / `register-edge-vhost` flow (see fishers
`devops/wslproxy/README.md`). Idempotent upsert of the Cloudflare CNAME is
documented in `docs/DEMO_API.md`.
