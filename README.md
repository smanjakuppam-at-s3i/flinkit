# Flinkit

**A vendor-neutral CLI to diagnose and triage Apache Flink jobs — health checks, bottleneck analysis, and a single-pane live view, straight from your terminal.**

Flinkit is for the moment a Flink job misbehaves and you're bouncing between the
Flink Web UI (one job at a time), a Grafana tab, and yet another system for logs.
It pulls the signals that matter into one place and — crucially — **tells you
what's wrong and what to do**, instead of leaving you to read charts.

> Status: early and moving fast. The diagnosis commands below work today against
> real clusters. Continuous monitoring and a web UI are on the roadmap.

---

## What it does

| Command | What it gives you |
|---|---|
| `flinkit scan` | Discover every job on a platform and write a normalized inventory to `.flinkit/inventory.json`. |
| `flinkit doctor <job>` | Rule-based health checks (checkpoints, backpressure, state TTL, restarts, operator UIDs) as a ranked findings list — with evidence and a suggested fix. Fleet mode (`--all`) and a CI gate (`--fail-on high`). |
| `flinkit bottleneck <job>` | Finds *the* choke point: samples per-subtask busy/backpressure, walks the job graph, and classifies the cause (key skew, slow sink, heavy state, CPU-bound). |
| `flinkit inspect <job>` | A single-pane live triage view — operators, I/O, recent logs, and the bottleneck — in one screen, refreshing with `--watch`. |

Every command speaks `--json` too, so it scripts cleanly and gates CI.

### A taste — `flinkit inspect`

```
Job: orders-enrichment (j1)  state RUNNING  flink 1.20.1
Checkpoints: 120 completed, 3 failed   Restarts: 2

Operators:
ID     NAME                  PAR  BUSY%  BP%
v-src  Source: Kafka orders  2    90     85
v-agg  Window Agg            4    98     3

I/O:
  source  v-src  Source: Kafka orders  in 4.0M / out 4.0M
  sink    v-agg  Window Agg            in 2.0M / out 1.2M

Bottleneck: Window Agg (v-agg) subtask 3 — key-skew
  fix: Salt hot keys or repartition so load spreads evenly across subtasks.

Recent logs:
  [jobmanager] 10:05 ERROR Async checkpoint 119 failed: timed out after 60000 ms
```

---

## Install

Requires **Go 1.23+**.

```sh
git clone git@github.com:smanjakuppam-at-s3i/flinkit.git
cd flinkit
make build        # produces ./flinkit in the repo
```

To use `flinkit` from anywhere, install it onto your PATH:

```sh
make install      # go install into $GOBIN (or ~/go/bin)
```

Make sure the install dir is on your PATH — if `flinkit version` isn't found,
add this to your shell profile (e.g. `~/.zshrc`) and open a new shell:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

## Quickstart

Point it at a Flink cluster with a profile in `~/.flinkit/config.yaml`
(secrets stay out of the file — reference env vars as `${VAR}`):

```yaml
default: local
profiles:
  local:
    source: oss
    endpoint: http://localhost:8081
```

Then:

```sh
flinkit scan                       # inventory every job
flinkit doctor <job-id>            # ranked health findings
flinkit doctor --all --fail-on high   # fleet health as a CI gate
flinkit bottleneck <job-id>        # find the choke point
flinkit inspect <job-id> --watch   # live single-pane triage
```

**On Kubernetes** (Flink Operator): `--source k8s` discovers every
`FlinkDeployment`/`FlinkSessionJob` in the cluster *and* diagnoses them live —
it reaches each job's JobManager REST API for you, no port-forward per job. Set
the namespace once in a profile:

```yaml
profiles:
  prod-k8s:
    source: k8s
    options:
      namespace: flink
      via: proxy          # optional; defaults to auto
```

```sh
flinkit scan    --profile prod-k8s            # inventory the fleet
flinkit doctor  <job-id> --profile prod-k8s   # live findings
flinkit inspect <job-id> --profile prod-k8s   # live triage
```

How it reaches the JobManager is set by `--via` (default `auto`), either as the
flag above or `options.via` in the profile:

| `--via` | Use when | How it connects |
|---|---|---|
| `auto` | default | `direct` in-cluster, `proxy` out-of-cluster |
| `proxy` | flinkit on your laptop/CI | through the Kubernetes apiserver's service proxy, reusing your kubeconfig auth — no port-forward or ingress |
| `direct` | flinkit as an in-cluster pod | service DNS `<job>-rest.<ns>:8081` |
| `port-forward` | single-job dev | a local `kubectl port-forward` (set the profile `endpoint`) |

> **Requirements.** Flinkit only needs each job's **JobManager REST API** to be
> reachable (it never scrapes the web dashboard — the REST API is on by default).
> `--source k8s` discovery needs `get`/`list` on the Flink CRDs, and `--via proxy`
> additionally needs `get`/`create` on `services/proxy` in the namespace (the
> apiserver proxy is built in — nothing to deploy). Ready-to-apply RBAC:
> [`docs/rbac.example.yaml`](docs/rbac.example.yaml). If your account is already
> cluster-admin (common in dev), you need none of this.

---

## How it works

- **Vendor-neutral by design.** Every platform is an adapter behind one
  interface — OSS Flink REST and the Kubernetes Operator today, CP/CC later —
  so the core never leaks platform-specific types.
- **Rules are data.** Health checks are declarative YAML with stable IDs
  (`DOC-CKPT-001`, …), each shipping with fixtures. Adding a rule doesn't touch
  the engine.
- **Deterministic first.** The analysis (bottleneck location, findings) is
  computed, not guessed — fast, repeatable, and safe to put in an alert.
- **Short-lived by nature.** It reads live signals and shows them; it is not a
  metrics store. Grafana keeps your history — Flinkit tells you what's wrong now.

## Roadmap (exploring)

- Continuous, intelligent monitoring: watch a job for a window and alert on
  correlated, Flink-aware findings (not raw thresholds).
- Fleet / multi-job view.
- A web UI over the same snapshot data.

## Project layout

```
cmd/flinkit/        entry point
internal/cli/       commands and flags
internal/model/     the Flinkit Model (Job, Operator, Subtask, …)
internal/adapters/  oss/ (REST) and k8s/ (Operator CRDs)
internal/rules/     rule engine + embedded built-in rules
internal/graph/     job-graph walk + bottleneck classification
internal/report/    terminal and JSON renderers
```

## License

Apache License 2.0. Copyright 2026 S3I Groups LLC.
