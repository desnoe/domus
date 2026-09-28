# GitHub Actions CI/CD & Self-Hosted Runner Setup Guide

This guide explains the two-stage GitHub Actions pipeline in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) and provides step-by-step instructions to configure the `local-runner` GitHub Environment, provision the dedicated runner VM (`runner.delarche.fr`) with Ansible, and trigger local lab test runs safely.

---

## 🏗️ Pipeline Architecture & Security Model

Running a `self-hosted` runner on a home network requires strict separation between automatic cloud checks and destructive local hardware tests. Furthermore, on a **GitHub Free** account with a **Private repository**, GitHub does not expose the `Required reviewers` manual approval gate inside Environments (that feature is reserved for Public repositories or GitHub Pro/Team).

To guarantee that Stage 2 (`integration-tests-local`) **never** triggers automatically on a normal `git push` or `pull_request`—and only runs when you explicitly request it on an authorized branch—[`.github/workflows/ci.yml`](../.github/workflows/ci.yml) combines an explicit `workflow_dispatch` toggle (`run_lab_tests`) with the `local-runner` environment branch restriction:

```mermaid
flowchart TD
    Trigger["Trigger: push / pull_request (main)\nor workflow_dispatch (Run workflow)"]

    subgraph Cloud ["Stage 1: GitHub-Hosted Cloud (ubuntu-latest — Automatic, Zero Secrets)"]
        direction TB
        UV1["1. setup-uv@v4 + Python 3.11 + uv sync --frozen"]
        Gal1["2. ansible-galaxy collection install"]
        PC["3. uv run pre-commit run --all-files (16 hooks)"]
        Syn["4. ansible-playbook --syntax-check (all playbooks)"]
        UV1 --> Gal1 --> PC --> Syn
    end

    Gate{"Manual Trigger Gate:\nworkflow_dispatch + run_lab_tests == true\n+ Environment: local-runner"}

    subgraph Local ["Stage 2: Self-Hosted Runner (runner.delarche.fr -> debian.delarche.fr)"]
        direction TB
        T0["0. test-lab.sh check-prereqs"]
        T1["1. test-lab.sh system-and-ufw"]
        T2["2. test-lab.sh storage-wipe-and-unlock"]
        T3["3. test-lab.sh backup-and-sync"]
        T4["4. test-lab.sh corner-cases-and-raid1"]
        T5["5. test-lab.sh status"]
        T0 --> T1 --> T2 --> T3 --> T4 --> T5
    end

    Trigger --> UV1
    Syn --> Gate
    Gate -->|"Push / PR (default)"| Skip["Stage 2 Skipped (Cloud check complete)"]
    Gate -->|"Run workflow (run_lab_tests = true)"| T0
```

1. **Stage 1 — `lint-and-syntax` (`runs-on: ubuntu-latest`)**:
   - Runs **automatically** on every `push` to `main`, every `pull_request` targeting `main`, and on manual `workflow_dispatch`.
   - Executes in an ephemeral, unprivileged GitHub-hosted cloud VM with **zero secrets**.
   - Creates a temporary placeholder `~/.ansible_vault_pass` (`0600`) so `ansible.cfg` (`vault_password_file = ~/.ansible_vault_pass`) satisfies static analysis without exposing the real Vault password.
   - Runs all 16 `pre-commit` hooks (`ansible-lint --strict`, `yamllint --strict`, `shellcheck --severity=style`, `check-ansible-vault`, etc.) and validates the syntax of every playbook in `ansible/*.yml`.
2. **Stage 2 — `integration-tests-local` (`runs-on: self-hosted`, `environment: local-runner`)**:
   - Guarded by `if: github.event_name == 'workflow_dispatch' && inputs.run_lab_tests == true` and scoped to `environment: local-runner`.
   - Automatically **skipped** on routine `git push` and `pull_request` events so your lab VM is never wiped unexpectedly.
   - Starts **only** when you trigger **Run workflow** in the GitHub UI with the **"Run full destructive lab integration tests on runner.delarche.fr"** checkbox enabled (and after `lint-and-syntax` succeeds).
   - Executes [`scripts/test-lab.sh`](../scripts/test-lab.sh) on `runner.delarche.fr` against the lab VM (`debian.delarche.fr`):
     - `system-and-ufw`: `system.yml` idempotence (`changed=0`) and UFW firewall drift remediation (`TC-SYS-01`, `TC-FW-03`).
     - `storage-wipe-and-unlock`: Full LUKS2 + Btrfs RAID1 reset (`storage-wipe-and-reset.yml -e confirm_wipe=yes`) and `storage-unlock.yml` idempotence (`TC-STO-01..02`).
     - `backup-and-sync`: `backup.yml` idempotence (`changed=0`), live `OneDrive:/Share/50ans` sync (`rclone-sync-onedrive-share-50ans.service`, verifying all 8 photos), and live Btrfs snapshot creation (`btrfs-snapshot-data.service`) (`TC-BAK-01..03`).
     - `corner-cases-and-raid1`: Unmounted 0ms refusal (`TC-ADV-01`), kernel mount-namespace EROFS sandbox probe (`TC-ADV-03`), LUKS2 header corruption & restore (`TC-ADV-06`), and simulated disk wipe + auto-detected RAID1 rebuild + scrub (`TC-ADV-05` / `TC-STO-04`).
     - `status`: Complete read-only diagnostic report (`status.yml` / `TC-BAK-04`).

---

## 🔒 Step 1 — Configure the `local-runner` Environment in GitHub

In your GitHub repository, navigate to **Settings** (top bar) $\rightarrow$ **Code, planning, and automation** (left sidebar) $\rightarrow$ **Environments** $\rightarrow$ **`local-runner`**.

On the **Environments / Configure local-runner** screen, you have three sections:

1. **Deployment branches and tags**:
   - Click the **`No restriction`** dropdown button on the right and change it to **`Selected branches and tags`** (or **`Protected branches only`** once `main` is protected).
   - Click **Add deployment branch or tag rule** and add **`main`** (and **`refactor26`** while testing on your current branch).
   - *Why*: This ensures that even if another branch exists in the repository, `local-runner` only accepts runs originating from your explicitly allowed branches.
2. **Environment secrets**:
   - **Recommended**: Leave **This environment has no secrets** as-is, and keep `~/.ansible_vault_pass` (`0600`) and `~/.config/rclone/rclone.conf` (`0600`) directly on `runner.delarche.fr` (provisioned in Step 2 below). That way, your LUKS passphrase and cloud tokens never leave your local network.
   - *Alternative*: If you prefer GitHub to inject them at runtime, click **Add environment secret** to define `ANSIBLE_VAULT_PASSWORD` and `RCLONE_CONF` (`scripts/test-lab.sh check-prereqs` supports both automatically).
3. **Environment variables**:
   - Leave empty (**This environment has no variables**); no non-sensitive environment variables are required.

> [!NOTE]
> **Why `Required reviewers` is not shown on this page**: On a **GitHub Free** account, GitHub only enables Environment `Required reviewers` on **Public** repositories (for Private repositories, it requires GitHub Pro or Team). Because `.github/workflows/ci.yml` uses `if: github.event_name == 'workflow_dispatch' && inputs.run_lab_tests == true`, Stage 2 is already strictly manual on your Private repository. If you ever switch the repository visibility to **Public**, a new **Deployment protection rules** section will appear at the top of `Configure local-runner`: immediately check **Required reviewers** and add your GitHub account before accepting any external Pull Requests.

---

## 🖥️ Step 2 — Provision & Register the Local Runner

You can run the self-hosted runner either on the dedicated **`runner.delarche.fr`** Debian VM managed by Ansible (**Option A**, recommended) or inside a rootless Podman container (**Option B**).

### Option A — Dedicated Debian VM (`runner.delarche.fr`) via Ansible (`systemd` Service)

`runner.delarche.fr` is declared in [`ansible/envs/live/inventory.ini`](../ansible/envs/live/inventory.ini) under the `[runner]` group and managed by the [`runner`](../ansible/roles/runner) role in [`ansible/system.yml`](../ansible/system.yml).

#### 1. Bootstrap & Provision `runner.delarche.fr` from Your Mac Controller

On a fresh Debian install on `runner.delarche.fr`:

```bash
# 1. Copy your SSH public key to runner.delarche.fr (once)
ssh-copy-id olivier@runner.delarche.fr

# 2. Run Day-0 bootstrap (configures passwordless sudo, SSH hardening, UFW, ~/.ansible_vault_pass, rclone.conf,
#    generates ~/.ssh/id_ed25519 on runner.delarche.fr, authorizes it on debian.delarche.fr, and installs the GitHub Actions runner binary)
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/bootstrap.yml --limit runner.delarche.fr -K
```

Whenever you want to update OS packages, system configuration, `~/.ansible_vault_pass`, `~/.config/rclone/rclone.conf`, or upgrade the GitHub Actions Runner binary on `runner.delarche.fr` to the latest upstream release:

```bash
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/system.yml --limit runner.delarche.fr
```

> [!TIP]
> The `runner` role automatically:
> - Copies your local `~/.ansible_vault_pass` (`0600`) and `~/.config/rclone/rclone.conf` (`0600`) to `runner.delarche.fr`.
> - Generates a dedicated Ed25519 keypair (`/home/olivier/.ssh/id_ed25519`) on `runner.delarche.fr`, adds `debian.delarche.fr` to `/home/olivier/.ssh/known_hosts`, and authorizes the runner's public key in `/home/olivier/.ssh/authorized_keys` on `debian.delarche.fr` (`exclusive: false`, so your Mac's keys and the runner's key coexist without overwriting each other, while `live` hosts only trust your Mac's keys).
> - Queries `https://api.github.com/repos/actions/runner/releases/latest`, stops `actions.runner.*.service` if an update is available, extracts the new release into `/opt/actions-runner`, and restarts the service.

#### 2. Register the Runner with GitHub (One-Time Interactive Step)

1. In your GitHub repository, go to **Settings** $\rightarrow$ **Code, planning, and automation** $\rightarrow$ **Actions** $\rightarrow$ **Runners** and click **New self-hosted runner**.
2. Select **Linux** and **x64** (or **ARM64**), and copy the registration `--token` shown under the **Configure** section.
3. On `runner.delarche.fr`, register the runner and enable its `systemd` service:

```bash
cd /opt/actions-runner

# Register the runner with your repository
./config.sh \
  --url https://github.com/desnoe/homecode \
  --token <YOUR_REGISTRATION_TOKEN> \
  --name runner.delarche.fr \
  --labels self-hosted,linux,lab \
  --unattended

# Install and start the native systemd service running as user 'olivier'
sudo ./svc.sh install olivier
sudo ./svc.sh start
```

4. Verify the runner health from your Mac controller via `status.yml`:

```bash
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/status.yml --limit runner.delarche.fr --tags runner
```

---

### Option B — Rootless Podman Container (Alternative)

If you prefer to run the GitHub Actions runner inside a rootless Podman container instead of a native `systemd` service:

1. Enable the rootless Podman socket for your user on the runner host:
   ```bash
   systemctl --user enable --now podman.socket
   loginctl enable-linger "$USER"
   export DOCKER_HOST="unix://${XDG_RUNTIME_DIR}/podman/podman.sock"
   ```
2. Launch the runner container mounting your read-only SSH key, `.ansible_vault_pass`, and `rclone.conf`:
   ```bash
   podman run -d \
     --name github-runner \
     --restart always \
     --network host \
     -e REPO_URL="https://github.com/desnoe/homecode" \
     -e RUNNER_NAME="podman-runner" \
     -e RUNNER_TOKEN="<YOUR_REGISTRATION_TOKEN>" \
     -e LABELS="self-hosted,linux,lab" \
     -v "${HOME}/.ssh:/home/runner/.ssh:ro,Z" \
     -v "${HOME}/.ansible_vault_pass:/home/runner/.ansible_vault_pass:ro,Z" \
     -v "${HOME}/.config/rclone:/home/runner/.config/rclone:Z" \
     docker.io/myoung34/github-runner:latest
   ```

---

## 🔄 Step 3 — Daily Workflow (Automatic Cloud Checks vs On-Demand Lab Tests)

1. **Routine `git push` or Pull Request (Automatic Cloud Validation — ~45 seconds)**:
   - Every push or PR targeting `main` automatically runs **Lint & Syntax Check (Cloud)** on `ubuntu-latest` (`uv sync --frozen`, all 16 `pre-commit` hooks, and `ansible-playbook --syntax-check` across all 7 playbooks).
   - **Full Lab Non-Regression Campaign (Self-Hosted)** is automatically **skipped**, keeping commits fast and leaving `debian.delarche.fr` untouched.
2. **Triggering the Full Lab Campaign on `runner.delarche.fr` (On-Demand via GitHub UI)**:
   - Open your repository on GitHub and click the **Actions** tab (top bar).
   - In the left sidebar, click **CI**.
   - Click the **Run workflow** dropdown button on the right:
     - Select the branch (e.g., `main` or `refactor26`).
     - Check **[x] Run full destructive lab integration tests on runner.delarche.fr**.
     - Click the green **Run workflow** button.
   - GitHub first runs **Lint & Syntax Check (Cloud)** on `ubuntu-latest`, and as soon as it passes, `runner.delarche.fr` picks up **Full Lab Non-Regression Campaign (Self-Hosted)** and executes all 5 stages of [`scripts/test-lab.sh`](../scripts/test-lab.sh) against `debian.delarche.fr`.

> [!TIP]
> You can also run [`scripts/test-lab.sh`](../scripts/test-lab.sh) directly from your terminal on your Mac or on `runner.delarche.fr` at any time:
> ```bash
> # Run a single stage
> ./scripts/test-lab.sh check-prereqs
> ./scripts/test-lab.sh backup-and-sync
>
> # Or run the entire lab non-regression campaign end-to-end
> ./scripts/test-lab.sh all
> ```

---

## 💰 Step 4 — GitHub Free Tier Quotas & Billing Rules

| Runner Type | Public Repository (`GitHub Free`) | Private Repository (`GitHub Free`) | Where Jobs Run |
| :--- | :--- | :--- | :--- |
| **GitHub-Hosted (`ubuntu-latest`)** *(Stage 1: `lint-and-syntax`)* | **100% Free & Unlimited** | **2,000 minutes / month included** *(~1,500–2,000 Stage 1 runs/month with `uv` cache)* | Microsoft Azure Cloud |
| **Self-Hosted (`self-hosted`)** *(Stage 2: `integration-tests-local`)* | **100% Free & Unlimited** | **100% Free & Unlimited** *(0 minutes deducted from your cloud quota)* | Your local VM (`runner.delarche.fr`) |

- **Zero Surprise Charges**: By default on a GitHub Free account, your spending limit for Actions is set to **$\$0.00$**. Even on a private repository, if you ever reached the 2,000 cloud minutes in a month, GitHub simply pauses cloud jobs until the next billing cycle without charging a cent, and **self-hosted runner jobs never consume any GitHub Actions minutes**.
