# 🗺️ domus Roadmap & Architecture Evolution

This document defines the strategic roadmap, planned architectural milestones, and implementation specifications for the `domus` infrastructure.

---

## 🎯 Vision & Core Principles

`domus` manages sovereign, encrypted, self-hosted Debian infrastructure and resilient storage. Every evolution adheres to the core architecture principles:
1. **Zero-Knowledge Security**: All storage is encrypted at rest via LUKS AES-256; credentials and secrets remain strictly protected with `ansible-vault`.
2. **Deterministic Infrastructure as Code**: All system states, daemons, mounts, and services are declared in Ansible roles and versioned.
3. **Native Systemd Sandboxing**: Services and backup units run strictly isolated (`ProtectSystem=strict`, cgroups memory limits, nice/IO scheduling).
4. **Idempotence & Test-Driven Verification**: Every playbook and role is validated through the automated integration test suite (`scripts/test-lab.sh`).

---

## 📅 Roadmap Overview

```
┌────────────────────────────────────────────────────────────────────────┐
│ Phase 1: Observability & Health Monitoring                             │
│   ├── Milestone 1.1: Unified Observability with Grafana Cloud Free Tier│
│   ├── Milestone 1.2: S.M.A.R.T. & Automated Btrfs Scrub Verification   │
│   ├── Milestone 1.3: Declarative Dashboards as Code via Foundation SDK │
│   └── Milestone 1.4: Telemetry Pipeline Optimization & Source Filtering│
├────────────────────────────────────────────────────────────────────────┤
│ Phase 2: Proactive Alerting & Notifications                            │
│   ├── Milestone 2.1: Grafana Cloud Alerting Rules                      │
│   └── Milestone 2.2: Multi-Channel Webhook Notifications               │
├────────────────────────────────────────────────────────────────────────┤
│ Phase 3: Docker Container Platform & Workload Orchestration            │
│   ├── Milestone 3.1: Docker Engine Provisioning & Hardening            │
│   ├── Milestone 3.2: Btrfs Storage Integration & CoW Protection        │
│   ├── Milestone 3.3: Network Isolation & UFW Firewall Compliance       │
│   └── Milestone 3.4: Declarative Application Stacks via Ansible Compose│
└────────────────────────────────────────────────────────────────────────┘
```

---

## 🚀 Phase 1: Observability & Health Monitoring

### Milestone 1.1 — Unified Observability with Grafana Cloud Free Tier `[Implemented]`

Deploy **Grafana Alloy** (the unified, lightweight OpenTelemetry-collector-based agent) across all nodes via a dedicated Ansible role `ansible/roles/alloy`. Alloy replaces multiple disparate collectors, routing metrics and systemd journal logs directly to the Grafana Cloud Free Tier.

#### Telemetry Agent Evaluation: Grafana Alloy vs. Vector
During architectural design, two premier telemetry agents were evaluated:
- **Vector (Datadog)**: Extremely lightweight Rust binary (~25-40 MB RAM) with powerful VRL (Vector Remap Language). While ideal for high-throughput stream remapping and vendor-agnostic pipelines, it lacks native turnkey Grafana Cloud dashboard integration and requires custom mapping for complex Prometheus node/storage metrics.
- **Grafana Alloy (Selected)**: The official Grafana agent based on the OpenTelemetry Collector. Chosen for:
  1. **Turnkey Grafana Cloud Integration**: Instant compatibility with official, pre-built Grafana Cloud dashboards for Linux hosts, storage, and systemd services.
  2. **Native Embedded Exporters**: Direct integration of `prometheus.exporter.unix` (no separate `node_exporter` or scrape targets needed).
  3. **Zero-Friction Journald-to-Loki Pipeline**: Reads `systemd-journald` natively and ships structured logs directly to Loki without complex transformation code.
  4. **Strict Resource Guarantees**: Sandboxed within cgroups (`MemoryHigh=256M`, `MemoryMax=512M`), consuming well under domestic hardware limits.

#### Free Tier Budget & Constraints
- **Metrics**: Up to 10,000 active Prometheus series.
- **Logs**: Up to 50 GB Loki logs per month with 14-day retention.
- **Traces**: Up to 50 GB Tempo traces per month (optional/future).
- **Users**: 3 active Grafana Cloud accounts.

#### Architectural Specifications

1. **Daemon Management (`ansible/roles/alloy`)**:
   - Packaged and installed via official Grafana APT repository (`https://apt.grafana.com`).
   - Managed via native `systemd` service (`alloy.service`).
   - Sandboxed with systemd hardening:
     ```ini
     [Service]
     Nice=15
     IOSchedulingClass=best-effort
     IOSchedulingPriority=7
     MemoryHigh=256M
     MemoryMax=512M
     ProtectSystem=strict
     ProtectHome=read-only
     ```

2. **Metrics Collection Pipeline (`prometheus.exporter.unix`)**:
   - **Host Metrics**: CPU, RAM, load average, system temperatures, and network bandwidth.
   - **Storage & Btrfs Metrics**:
     - Block device and LUKS mapper I/O statistics (`diskstats`).
     - Filesystem usage across all subvolumes (`filesystem`).
     - Btrfs internal metadata allocation (`node_exporter` Btrfs collector).
   - **Export Target**: `prometheus.remote_write` to Grafana Cloud Prometheus endpoint with basic auth (`vault_grafana_cloud_metrics_username` and `vault_grafana_cloud_api_key`).
   - **Series Optimization**: Metric filter drop rules discarding high-cardinality, unnecessary metrics (e.g. per-CPU frequency, cgroup memory breakdowns) to keep total active series well under 1,500 series across all domestic nodes.

3. **Log Aggregation Pipeline (`loki.source.journal`)**:
   - Scrapes local `systemd-journald` log streams via native Alloy journal reader.
   - Structured labels: `host`, `unit`, `priority`, `facility`.
   - **Targeted Log Streams**:
     - `sshd.service`: Authentication attempts, failures, and session drops.
     - `ufw` / kernel netfilter: Blocked connection attempts on public interfaces.
     - `kernel`: Btrfs error events, LUKS mapper warnings, disk I/O errors.
     - `rclone-sync-*.service`: Cloud backup duration, file transfers, and rclone errors.
     - `btrfs-snapshot-*.service`: Snapshot creation and retention pruning logs.
     - `smbd.service`: SMB share access, connection events, and audit logs.
   - **Noise Filtering**: Alloy drop stages discarding repetitive low-value informational journal messages to remain well below 5 GB/month of Loki logs.
   - **Export Target**: `loki.write` to Grafana Cloud Loki endpoint.

4. **Ansible Role Configuration Schema**:
   ```yaml
   # ansible/envs/live/group_vars/all/main.yml
   alloy_enabled: true
   alloy_grafana_cloud_metrics_url: "https://prometheus-prod-XX-prod-eu-west-X.grafana.net/api/prom/push"
   alloy_grafana_cloud_logs_url: "https://logs-prod-XXX.grafana.net/loki/api/v1/push"

   # ansible/envs/live/host_vars/<host>/vault.yml (Encrypted AES256)
   vault_grafana_cloud_instance_id: "123456"
   vault_grafana_cloud_api_key: "glc_eyJ..."
   ```

5. **Grafana Cloud Dashboards**:
   - **Host Overview Dashboard**: Live CPU, memory, load, thermal sensor status, network traffic.
   - **domus Storage Dashboard**: LUKS mapper throughput, Btrfs filesystem space, subvolume allocations, and snapshot growth.
   - **Backup & Sync Health Dashboard**: Execution timeline, transfer speeds, snapshot pruning history, and failure indicators for Rclone and Snapshot timers.

---

### Milestone 1.2 — S.M.A.R.T. Hardware Health & Automated Btrfs Scrub `[Implemented]`

Proactive disk degradation detection before silent hardware failure causes data loss, deployed across all hosts and storage pools:

#### Architectural Implementation

1. **S.M.A.R.T. Disk Health Monitoring (`ansible/roles/smartctl`)**:
   - Deployed `smartmontools` and the official `prometheus-community/smartctl_exporter` (`v0.15.0`) across all nodes (`hosts: all`).
   - Hardened `smartctl_exporter.service` bound strictly to loopback (`127.0.0.1:9633`), isolated via `ProtectSystem=strict`, `ProtectHome=true`, `PrivateTmp=true`, and `NoNewPrivileges=true`.
   - Enabled native S.M.A.R.T. scraping inside Grafana Alloy (`prometheus.scrape "smartctl"`, 120s interval), forwarding `smartctl_.*` metrics to Grafana Cloud.
   - Tracks key reliability indicators:
     - Reallocated Sector Count (`Attribute 5`)
     - Current Pending Sector Count (`Attribute 197`)
     - Offline Uncorrectable Sector Count (`Attribute 198`)
     - SSD / NVMe Percentage Used & Available Spare
     - Operating temperature thresholds.

2. **Automated Periodic Btrfs Scrub (`ansible/roles/crypto`)**:
   - Implemented declarative `btrfs-scrub-<pool>.timer` and `.service` units in `ansible/roles/crypto`.
   - Frequency: Monthly (1st Sunday of each month at 02:00 via `OnCalendar=Sun *-*-1..7 02:00:00`, `Persistent=true`).
   - Executes background read-and-checksum verification across all member devices of the Btrfs pool:
     ```bash
     btrfs scrub start -B -d /home/olivier/data/sync
     ```
   - Zero disruption: I/O priority set to idle class (`IOSchedulingClass=idle`, `Nice=19`).
   - Sandboxing: `ProtectSystem=strict`, `ProtectHome=read-only`, `ReadWritePaths=/var/lib/btrfs`.
   - Automated error propagation: Scrub output streams into systemd journald (ingested by Loki via Alloy), while any checksum/read errors update kernel stats (`btrfs device stats`), automatically incrementing `node_btrfs_device_errors_total` exported to Prometheus.

---

### Milestone 1.3 — Declarative Dashboards as Code via Grafana Foundation SDK (Go) & Continuous Delivery Pipeline `[Implemented]`

Deploy a reproducible, code-defined Grafana dashboard suite managed in Go via the official **Grafana Foundation SDK** (`github.com/grafana/grafana-foundation-sdk/go`). Dashboards provide complete visibility across all ingested system telemetry, backed by a local pre-commit branch preview workflow and automated GitHub Actions delivery.

#### Architectural Specifications

1. **Dashboard Project Architecture (`dashboards/`)**:
   - Centralized Go project under `dashboards/` generating Kubernetes-compatible Grafana dashboard manifests (`dashboard.grafana.app/v0alpha1` / `v1alpha1`).
   - Manifests are dynamically rendered on the fly (gitignored), avoiding repository noise while maintaining strict versioning of dashboard generator logic.
   - Reusable builder helpers (`dashboards/internal/builder/`) defining shared datasources (`grafanacloud-domus-prom`, `grafanacloud-domus-logs`), query helpers, unified color palettes, units, and auto-grid layouts.

2. **Core Dashboard Catalog**:
   - **`domus-host` (Compute & Network)**:
     - Compact 4-metric synthesis header (`Uptime`, `CPU Busy`, `Memory Used`, `Load 1m`).
     - CPU utilization across cores/modes, load averages (1m, 5m, 15m), context switches/interrupts, and process execution states (`node_procs_running`, `node_procs_blocked`).
     - Memory distribution (Used, Available, Buffers, Cached, Dirty/Writeback, Swap) and OOM kill events (`node_vmstat_oom_kill`).
     - System hardware thermals (`node_thermal_zone_temp`), file descriptors, OS and kernel version metadata (`node_uname_info`), and NTP synchronization status & maximum estimated error (`node_timex_sync_status`, `node_timex_offset_seconds`, `node_timex_maxerror_seconds`).
     - Network interface bandwidth (Rx/Tx bytes/s), packet rates, error counters, drop rates, TCP socket states (`node_sockstat_TCP_*`), and TCP retransmission rates.
   - **`domus-storage` (LUKS, Btrfs & Hardware S.M.A.R.T.)**:
     - Compact 4-metric synthesis header (`S.M.A.R.T. Drive Health`, `Critical Sectors`, `Storage Capacity Used`, `Total Disk I/O Rate`).
     - Separated physical block devices (`sd*`/`nvme*`) and LUKS dm-crypt mappers (`dm-*`) read/write throughput, IOPS, average I/O latency, and utilization (`node_disk_*`).
     - Btrfs subvolume filesystem capacity, available space, inode utilization (`node_filesystem_files`), underlying block device error counts (`node_filesystem_device_error`), and read-only health badges (`node_filesystem_readonly`).
     - Btrfs pool label propagation (`* on(uuid) group_left(label) node_btrfs_info`) for member disk physical size, unallocated physical space (`node_btrfs_device_unused_bytes`), device errors, chunk allocations (Data/Metadata/System), redundancy allocation ratio (`node_btrfs_allocation_ratio`), and global reserve (`node_btrfs_global_rsv_size_bytes`).
     - S.M.A.R.T. multi-drive telemetry: overall SMART status, smartctl command execution exit status (`smartctl_device_smartctl_exit_status`), ATA error log entries (`smartctl_device_error_log_count`), drive media type and rotational speed (`smartctl_device_rotation_rate`), operating temperature, raw capacity (`smartctl_device_capacity_bytes`), power-on hours, power cycles (`smartctl_device_power_cycle_count`), dedicated critical sector panels (Reallocated, Current Pending, Offline Uncorrectable, UDMA CRC), and tall Loki hardware exporter audit logs.
   - **`domus-backups` (Backup Units & Audit Logs)**:
     - Compact 4-metric synthesis header (`Failed Services`, `Active Daemons`, `Active Backup Execution`, `Audit Log Errors (24h)`).
     - Interactive Monitored Services Inventory table (`table`) with colored status badges (`ACTIVE`, `STANDBY`, `FAILED`), Active Service Failures multi-stat alert card, and a 24-Hour Service Execution Timeline (`statetimeline`) tracking `rclone-sync-*`, `btrfs-snapshot-*`, `btrfs-scrub-*`, `smbd`, `alloy`, `smartctl_exporter`, `ufw`, and `ssh` units.
     - Full-width tall Loki journald log viewer (`logs`) for backup, scrub, and storage operations.
   - **`domus-alloy-health` (Alloy Telemetry Pipeline)**:
     - Compact 4-metric synthesis header (`Alloy Agent Status`, `Alloy Memory (RSS)`, `Samples Sent Rate`, `Dropped Samples / Logs`).
     - Process-level resource consumption: Alloy CPU, resident memory (RSS), and Go runtime goroutines.
     - Scrape target health, `prometheus.remote_write` throughput, active shards, dropped samples, retries, and Loki log shipping rate, batch retries, and dropped entries.

3. **Pre-Commit Branch Preview & Live Verification Workflow (`scripts/dashboards.sh` & `scripts/verify_live.py`)**:
   - Local command `./scripts/dashboards.sh preview [branch]` dynamically resolves the current Git branch and generates dashboards targeting an isolated Grafana Cloud folder `domus-prev-<slug>`.
   - Manifests are validated and pushed directly via `gcx --context domus resources push`, outputting clickable dashboard URLs in the terminal.
   - Automated live verification `./scripts/dashboards.sh test-live [branch]` (backed by `scripts/verify_live.py`) verifies `$host` template variable population, asserts that all 121 panel queries return live Mimir/Loki telemetry without errors, validates headless PNG snapshot renders, and purges the preview folder on exit.
   - Companion cleanup command `./scripts/dashboards.sh clean-preview [branch]` purges the branch preview folder and test dashboards from Grafana Cloud.

4. **Continuous Delivery Pipeline (`.github/workflows/dashboards.yml`)**:
   - **Pull Request Lifecycle**:
     - Validates Go syntax, runs unit tests (`go test -v ./...`), and validates manifests against Grafana Cloud.
     - Automatically deploys preview dashboards to temporary folder `domus-pr-<number>` on Grafana Cloud and executes live telemetry verification (`scripts/verify_live.py`).
     - Automatically cleans up and purges folder `domus-pr-<number>` upon PR closure.
   - **Production Release**:
     - On merge to `main`, automatically compiles and deploys production manifests to the primary `domus` folder in Grafana Cloud (`./scripts/dashboards.sh deploy`).

---

### Milestone 1.4 — Telemetry Pipeline Optimization & Metric Filtering at Source

Prune superfluous low-level kernel metrics at the Alloy ingestion layer to maintain strict 1:1 parity between collected Prometheus series and dashboard visualization panels:
1. **Source Filtering Refinement (`ansible/roles/alloy`)**:
   - Review and tighten the `metrics_filter` keep regex in `config.alloy.j2` to drop non-actionable raw kernel counters (e.g. obscure memory buffers, deep paging counters, unused network protocol errors).
   - Ensure every retained metric series directly feeds a visual panel or an alerting rule.
2. **Cardinality & Budget Protection**:
   - Guarantee active Prometheus series usage remains below 1,000 series across all nodes, maximizing headroom under the Grafana Cloud Free Tier budget (10,000 series limit).

---

## 🚨 Phase 2: Proactive Alerting & Notifications

### Milestone 2.1 — Grafana Cloud Alerting Rules

Centralize threshold evaluation and incident detection in Grafana Cloud Alerting (included in the free tier).

#### Key Alert Definitions
| Alert Rule | Condition | Severity | Description |
| :--- | :--- | :--- | :--- |
| **Storage Capacity High** | Subvolume free space $< 15\%$ | Warning | Imminent disk exhaustion |
| **Storage Capacity Critical** | Subvolume free space $< 5\%$ | Critical | Critical write lock risk |
| **Btrfs RAID1 Degraded** | Missing member device or read errors $> 0$ | Critical | Redundancy lost on mirror |
| **Btrfs Checksum Errors** | Scrub checksum errors $> 0$ | Critical | Silent bit rot detected |
| **Backup Unit Failed** | `systemd_unit_state{name=~"rclone-sync.*|btrfs-snapshot.*", state="failed"} == 1` | Warning | Backup or snapshot skipped |
| **Disk S.M.A.R.T. Degradation** | Pending or reallocated sectors $> 0$ | Warning | Drive hardware failure imminent |
| **Host Unreachable** | Alloy instance `up == 0` for $> 5m$ | Critical | Node power loss or kernel panic |
| **Security Brute Force** | SSH failed logins $> 20$ in 5m | Warning | Active SSH attack attempt |

---

### Milestone 2.2 — Multi-Channel Webhook Notifications

Deliver actionable alerts to mobile devices without requiring paid SMS gateways:
1. **Primary Push Channel**: **ntfy.sh** (free, open-source push notification service) or **Telegram Bot API**.
2. **Alert Formatting**: Rich notifications with hostname, affected subvolume/unit, exact severity badge, and direct URL to the Grafana Cloud dashboard.
3. **Escalation**: Critical alerts (Btrfs checksum error, RAID degradation) bypass "Do Not Disturb" via high-priority push flags.

---

## 🐳 Phase 3: Docker Container Platform & Workload Orchestration

### Milestone 3.1 — Docker Engine Provisioning & Hardening

Deploy Docker Engine via a dedicated Ansible role `ansible/roles/docker`:
- **Repository & Packages**: Pinned official Docker APT repository (`https://download.docker.com/linux/debian`) installing `docker-ce`, `docker-ce-cli`, `containerd.io`, and `docker-compose-plugin`.
- **Daemon Configuration (`/etc/docker/daemon.json`)**:
  ```json
  {
    "log-driver": "journald",
    "log-opts": {
      "tag": "{{.Name}}"
    },
    "live-restore": true,
    "userland-proxy": false,
    "exec-opts": ["native.cgroupdriver=systemd"],
    "no-new-privileges": true
  }
  ```
- **Zero-Effort Observability**: By configuring `log-driver: "journald"`, all container stdout/stderr streams flow natively into `systemd-journald`. Grafana Alloy (Phase 1) automatically captures them with container name tags, forwarding structured logs to Grafana Cloud Loki without requiring third-party Docker log plugins.
- **Service Isolation**: Hardened systemd unit overrides enforcing resource limits (`TasksMax=infinity`, memory protection).

---

### Milestone 3.2 — Btrfs Storage Integration & Copy-on-Write (CoW) Protection

Integrate Docker persistent state and application volumes with `domus`'s encrypted Btrfs storage:
- **Root & Layer Storage**: Docker graph driver utilizes `overlay2` or native `btrfs` snapshot driver located on a dedicated Btrfs subvolume.
- **Database CoW Fragmentation Mitigation**:
  - Stateful databases (PostgreSQL, MariaDB, SQLite) and write-heavy workloads experience severe fragmentation and write amplification on Copy-on-Write filesystems.
  - Persistent volume paths for database data directories are declaratively provisioned with `crypto_no_compression_paths` (`chattr -R +m` / `nodatacow`) before container initialization, preventing CoW amplification while keeping data secure inside LUKS containers.
- **Volume Backup Synergy**: Application directories residing on Btrfs subvolumes benefit directly from `domus`'s automated snapshot retention (`snapshot` role) and offsite sync (`rclone` role).

---

### Milestone 3.3 — Network Isolation & UFW Firewall Compliance

Tackle the standard Docker firewall bypass risk to maintain `domus`'s strict perimeter defense:
- **The Issue**: By default, Docker manipulates `iptables` directly (`DOCKER` chain), bypassing the `INPUT` chain and exposing published container ports publicly even if UFW is configured to block them.
- **Resolution Strategy**:
  1. **Strict Localhost Binding**: Application compose files bind exposed ports exclusively to loopback (`127.0.0.1:<port>`), preventing external LAN access unless routed through a controlled proxy or SSH tunnel.
  2. **UFW-Docker Integration**: Integrate iptables routing rules into `ansible/roles/ufw` (via `/etc/ufw/after.rules`) ensuring that Docker container ports are subject to the same strict LAN-only subnets as native services.

---

### Milestone 3.4 — Declarative Application Stacks via Ansible & Docker Compose

Provide a standardized framework for deploying self-hosted domestic applications:
- **Directory Hierarchy**: Clean per-application structure under `/home/{{ ansible_user }}/apps/<app_name>/`:
  - `docker-compose.yml`: Declarative stack definition.
  - `.env`: Host-specific environment variables templated via Ansible.
  - `data/`: Volume mount point on encrypted Btrfs subvolumes.
- **Ansible Automation**: Managed through `community.docker.docker_compose_v2` with state verification and idempotence.
- **Target Applications**: Home Assistant (domotics), Vaultwarden (password management), Paperless-ngx (document archiving), and media services.

---

## 🔒 Public Repository Security & Governance

Following the public release of `domus`:
1. **GitHub Actions Environment Gate**:
   - The integration test suite (`integration-tests-local`) is bound to the `lab` environment.
   - Any execution on the self-hosted lab runner requires explicit manual approval by repository maintainers.
2. **Fork Protection**:
   - External PRs from forks execute exclusively on cloud GitHub-hosted runners (`ubuntu-latest`) running static linters.
   - Self-hosted runners are physically insulated from public untrusted code.
3. **Credential Segregation**:
   - Sensitive production variables remain strictly encrypted in host vaults.
   - Example templates (`ansible/vault.yml.example`) document required variable schemas without leaking real parameters.
