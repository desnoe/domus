# Feuille de route (Roadmap) - Modernisation & Sécurisation de Homecode

Ce document répertorie l'ensemble des axes d'amélioration identifiés pour le dépôt `homecode`, découpés en phases logiques et ordonnées par priorité.

---

## 🎯 Priorité en cours : Outillage de développement & Reproductibilité
- [x] Initialisation de l'environnement Python virtuel avec `uv` (Python 3.11).
- [x] Fixation des versions des outils (`ansible-core`, `ansible-lint`, `yamllint`, `pre-commit`).
- [x] Définition et fixation des dépendances de collections Ansible (`ansible/requirements.yml`).
- [x] Mise en place de `pre-commit` avec contrôles qualité (`ansible-lint`, `yamllint`, `shellcheck`, etc.).
- [x] Mise à jour du fichier `.gitignore`.

---

## 🔴 Phase 1 : Sécurité Immédiate & Gestion des Secrets (Priorité Haute)

### 1.1. Chiffrement du mot de passe utilisateur
- [ ] Déplacer le hash du mot de passe en clair situé dans [`ansible/roles/sudo/tasks/main.yml`](ansible/roles/sudo/tasks/main.yml) vers les fichiers de variables chiffrés (`vault.yml`).
- [ ] Remplacer par une variable `user_password: "{{ vaulted_user_password }}"`.

### 1.2. Sécurisation du déchiffrement et chiffrement LUKS
- [ ] Supprimer la création du fichier temporaire `/tmp/encdisk.XXXXXXX` dans [`ansible/roles/crypto/tasks/disk-encrypt.yml`](ansible/roles/crypto/tasks/disk-encrypt.yml) et [`ansible/roles/crypto/tasks/disk-decrypt.yml`](ansible/roles/crypto/tasks/disk-decrypt.yml).
- [ ] Utiliser soit le module `community.crypto.luks_device`, soit un passage sécurisé par entrée standard (`cryptsetup ... --key-file -`) avec `no_log: true` pour éviter toute écriture en clair sur disque ou fuite dans les journaux.

### 1.3. Sécurisation de la configuration sudoers
- [ ] Remplacer l'édition directe de `/etc/sudoers` par un fichier drop-in dans `/etc/sudoers.d/wheel` ou `/etc/sudoers.d/99-ansible-wheel`.
- [ ] Réactiver la validation syntaxique systématique `visudo -cf %s`.

### 1.4. Sécurité des scripts de synchronisation rsync
- [ ] Dans [`scripts/rsync-mac-to-external-hdd.sh`](scripts/rsync-mac-to-external-hdd.sh), ajouter une vérification préalable stricte du point de montage (`[ -d "$DST_DIR" ]`) pour éviter d'inonder le disque interne du Mac en cas de disque externe débranché.
- [ ] Remplacer les chemins utilisateur codés en dur (`/Users/olivier/`) par la variable `$HOME`.
- [ ] Créer systématiquement les dossiers parents de logs (`mkdir -p "$(dirname "$LOGFILE")"`) avant l'exécution de `rsync`.

---

## 🟡 Phase 2 : Modernisation Ansible & Fiabilisation Stockage (Priorité Moyenne)

### 2.1. Élimination des syntaxes et directives dépréciées
- [ ] Remplacer les directives dépréciées `include:` par `ansible.builtin.import_tasks` ou `ansible.builtin.include_tasks` dans [`ansible/roles/crypto/tasks/main.yml`](ansible/roles/crypto/tasks/main.yml).
- [ ] Remplacer les boucles inefficaces `with_items` sur le module `apt` dans l'ensemble des rôles (`base`, `apt`, `htop`, `iptables`, `locales`, `ntp`, `vim`) par le passage d'une liste unique `name: [...]`.
- [ ] Corriger l'incohérence du rôle `base` : utiliser [`ansible/roles/base/files/bashrc.root`](ansible/roles/base/files/bashrc.root) pour `/root/.bashrc` au lieu du bashrc utilisateur standard.
- [ ] Ajouter `become: true` sur la tâche d'activation du service dans [`ansible/roles/ntp/tasks/main.yml`](ansible/roles/ntp/tasks/main.yml).
- [ ] Migrer vers les FQCN (`ansible.builtin.*`, `ansible.posix.*`, `community.general.*`).

### 2.2. Compatibilité avec Debian 11/12+
- [ ] Remplacer le paquet obsolète `btrfs-tools` par `btrfs-progs` dans [`ansible/roles/crypto/tasks/main.yml`](ansible/roles/crypto/tasks/main.yml).
- [ ] Adapter le rôle `ntp` pour installer `systemd-timesyncd` ou `chrony` sur Debian 12 (Bookworm).

### 2.3. Simplification du montage Btrfs
- [ ] Supprimer la logique temporaire de montage et le parsing d'ID de sous-volumes par regex dans [`ansible/roles/crypto/tasks/fs-mount.yml`](ansible/roles/crypto/tasks/fs-mount.yml).
- [ ] Utiliser directement l'option de montage standard par nom de sous-volume Btrfs : `opts: "subvol=sync"` et `opts: "subvol=snapshots"`.

### 2.4. Nettoyage de la double exécution du rôle crypto
- [ ] Corriger la dépendance entre `rclone` et `crypto` dans [`ansible/roles/rclone/meta/main.yml`](ansible/roles/rclone/meta/main.yml) et [`ansible/reconfigure-and-decrypt.yml`](ansible/reconfigure-and-decrypt.yml) pour éviter d'exécuter deux fois les tâches de chiffrement.

---

## 🟢 Phase 3 : Maintenance & Rétention des Données

### 3.1. Politique de rétention des snapshots Btrfs
- [ ] Mettre en place une rotation automatique des snapshots dans [`ansible/roles/crypto/files/snapshot.sh`](ansible/roles/crypto/files/snapshot.sh) (ex. conservation des 7 derniers jours, 4 dernières semaines, puis suppression des plus anciens).

### 3.2. Échelonnement et rétention des sauvegardes rclone
- [ ] Échelonner les horaires de lancement dans crontab pour chaque bucket (`rclone_jobs`) afin d'éviter la contention réseau et CPU à 04:00.
- [ ] Mettre en place un nettoyage des anciens fichiers journaux dans [`ansible/roles/rclone/files/rclone.sh`](ansible/roles/rclone/files/rclone.sh) (rétention de 30 jours).

---

## 🔵 Phase 4 : Refonte des Scripts, CI & Documentation

### 4.1. Refonte des scripts ImageMagick
- [ ] Renommer [`scripts/png2jpga4`](scripts/png2jpga4) en `png2jpga4.sh`.
- [ ] Remplacer l'extraction fragile du format par `magick identify -format "%m" "$infile"`.
- [ ] Ajouter le mode strict Bash (`set -euo pipefail`) sur tous les scripts.
- [ ] *(Optionnel)* Factoriser les scripts de conversion en un utilitaire unique et paramétrable.

### 4.2. Automatisation CI
- [ ] Mettre en place un workflow GitHub Actions (`.github/workflows/lint.yml`) pour vérifier automatiquement les PRs et commits via `pre-commit` (`ansible-lint`, `yamllint`, `shellcheck`).

### 4.3. Documentation complète du projet
- [ ] Enrichir le fichier [`README.md`](README.md) :
  - Architecture détaillée du stockage (LUKS $\rightarrow$ LVM $\rightarrow$ Btrfs $\rightarrow$ Snapshots $\rightarrow$ Rclone).
  - Guide d'utilisation des playbooks (`bootstrap.yml`, `reconfigure-and-decrypt.yml`, `decrypt-only.yml`, `wipe-and-reencrypt.yml`).
  - Procédure d'onboarding, de gestion du vault et d'ajout d'une nouvelle machine.
