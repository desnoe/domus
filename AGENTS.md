# Instructions for AI Agents & Contributors

## 🌐 Language Policy (Mandatory)

- **All repository content MUST be written in English**:
  - Documentation (`README.md`, `ROADMAP.md`, architecture guides, etc.)
  - Code, task names, variable names, and comments in all files (Python, Bash, Ansible YAML, Jinja2, etc.)
  - Git commit messages and pull request descriptions
  - Configuration comments (`.ansible-lint`, `.yamllint`, `ansible.cfg`, etc.)
- **Conversations with users**: Even if conversations or requests with users take place in French (or any other language), all artifacts, commits, and file modifications in this repository must remain strictly in English.

---

## 🛠️ Tooling & Environment Standards

- **Python & Environment**:
  - Always use `uv` for Python virtual environments and dependency management.
  - Python version is pinned to `3.11` via `.python-version`.
  - Dependencies are managed via `pyproject.toml` and locked with `uv.lock`.
  - Execute Python/Ansible commands using `uv run <command>`.

- **Ansible Standards**:
  - `ansible-core` version is pinned in `pyproject.toml`.
  - Collection dependencies are declared with exact versions in `ansible/requirements.yml` and installed locally under `ansible/collections/`.
  - Always prefer Fully Qualified Collection Names (FQCN) such as `ansible.builtin.*`, `ansible.posix.*`, `community.general.*`, `community.crypto.*`.
  - Avoid deprecated directives (e.g. use `import_tasks` / `include_tasks` instead of `include`).
  - Quote all file permissions (e.g. `mode: "0644"`, `mode: "0755"`).
  - Pass package lists directly to `ansible.builtin.apt` instead of using `with_items` loops.

- **Secrets & Security**:
  - Never commit plaintext secrets, passwords, or password hashes into version control.
  - Encrypt all secrets using `ansible-vault` with password file `~/.ansible_vault_pass`.
  - Reference documented variables in `ansible/vault.yml.example`.

- **Pre-commit & Code Quality**:
  - All contributions must pass `uv run pre-commit run --all-files` with a clean exit code (`0`).
  - Linters enforced: `ansible-lint`, `yamllint`, `shellcheck`, and standard `pre-commit-hooks`.
