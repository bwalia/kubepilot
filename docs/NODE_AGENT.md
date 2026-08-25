# Node agent — physical hardware in the dashboard

Kubernetes does not know what a node *is*. `status.nodeInfo` reports the OS,
the kernel, the architecture and the container runtime, and stops there. On a
home lab or a rack of identical workers, `debian001` and `debian002` look the
same from the API even when one is an HP ZBook on a shelf and the other is a
Dell OptiPlex under a desk.

The KubePilot node agent fills that gap. It runs as a DaemonSet — one small pod
per node, the same image as KubePilot itself — reads the machine's own
description out of `/sys` and `/proc`, and writes it back onto the node:

| Key | Example | Source |
| --- | --- | --- |
| `kubepilot.io/hardware` | `HP ZBook 17 G5` | DMI `sys_vendor` + `product_name` |
| `kubepilot.io/serial` | `5CD9131NFM` | DMI `product_serial`, falling back to `board_serial` |
| `kubepilot.io/hw.cpu` | `Intel(R) Core(TM) i7-8850H CPU @ 2.60GHz` | `/proc/cpuinfo` |
| `kubepilot.io/hw.cores` | `12` | `/proc/cpuinfo` |
| `kubepilot.io/hw.memory` | `30.7 GB` | `/proc/meminfo` |
| `kubepilot.io/hw.disks` | `nvme0n1 2.0 TB (CT2000T705SSD3), sda 1.0 TB (HGST …)` | `/sys/block` |
| `kubepilot.io/hw.nics` | `enp0s31f6 100Mb/s c4:65:16:9a:da:c3` | `/sys/class/net` |
| `kubepilot.io/hw.bios` | `HP Q70 Ver. 01.32.00 (02/27/2025)` | DMI |
| `kubepilot.io/hw.board` | `HP 842D` | DMI |
| `kubepilot.io/hw.chassis` | `Notebook` | DMI chassis type |
| `kubepilot.io/hw.version` | `SBKPF` | DMI `product_version` |
| `kubepilot.io/hw.virtualization` | `QEMU/KVM` | DMI, when the machine is a VM |
| `kubepilot.io/lan-ip` (label) | `192.168.1.140` | the host's default route |

The dashboard shows the model under each node's name, and the full set in the
node's **Hardware** tab. What Kubernetes knows about the same node lives in the
**Kubernetes** tab next to it.

## Install

It ships with the chart and is on by default:

```sh
helm upgrade --install kubepilot charts/kubepilot -n kubepilot-system
```

Or through Ansible, which also removes the legacy `node-labeler` DaemonSet the
agent replaced:

```sh
cd deploy/ansible
KUBECONFIG=~/.kube/k3s1.yaml ansible-playbook cluster.yml
```

Turn it off with `--set nodeAgent.enabled=false`.

Verify:

```sh
kubectl get nodes -o custom-columns='NODE:.metadata.name,HARDWARE:.metadata.annotations.kubepilot\.io/hardware'
kubectl -n kubepilot-system logs -l app.kubernetes.io/component=node-agent --tail=1
```

## What it needs, and why

- **`hostNetwork`** — the LAN IP is the source address of the host's default
  route. On a WireGuard-meshed k3s cluster Kubernetes only knows the tunnel
  address, so without this the dashboard shows the wrong network entirely.
- **`/sys` and `/proc`, mounted read-only** — where DMI, block devices and NICs
  live. Nothing is written to the host.
- **`runAsUser: 0`** — DMI serial numbers are mode `0400`. Everything else the
  pod does would work unprivileged.
- **RBAC: `nodes` `get,list,patch`** — nothing else, in any namespace.

Facts are re-published every `nodeAgent.interval` (default 15m), so a node whose
annotations are wiped repairs itself, and a node that joins the cluster is
described within a few minutes without anyone touching it.

## When the hardware is unreadable

Virtual machines report the hypervisor's idea of the hardware (`QEMU Standard
PC`), and some boards ship placeholder DMI strings (`To Be Filled By O.E.M.`),
which the agent discards rather than displaying. Raspberry Pis have no DMI at
all — their model comes from the device tree instead.

Where the answer is unhelpful or missing, write it yourself. A merge patch only
touches the keys it names, so an annotation the agent cannot derive is left
alone:

```sh
kubectl annotate node cloud001 --overwrite \
  kubepilot.io/hardware="Hetzner CX41 (fsn1-dc14)"
```
