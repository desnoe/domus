# 🚑 Disaster Recovery & Storage Operations Guide

This guide provides step-by-step procedures for emergency recovery, data restoration, LUKS header restoration, and hardware replacement for systems managed by the `homecode` infrastructure.

---

## 🏗️ Storage Stack Summary

Each encrypted server (`debian.delarche.fr`, `kodlix.delarche.fr`, `zotac.delarche.fr`) uses the following layered stack:

```mermaid
flowchart TD
    Disk0["Physical Device 0 (/dev/disk/by-id/...)"] -->|cryptsetup luksOpen| Mapper0["Mapper Device (/dev/mapper/data-0)"]
    Disk1["Physical Device 1 (/dev/disk/by-id/...)"] -->|cryptsetup luksOpen| Mapper1["Mapper Device (/dev/mapper/data-1)"]
    Mapper0 -->|Btrfs RAID1 (LABEL=data)| BtrfsRoot["Btrfs Root (~/crypto/data)"]
    Mapper1 -->|Btrfs RAID1 (LABEL=data)| BtrfsRoot
    BtrfsRoot --> SubvolSync["Subvolume: sync (~/crypto/data/sync)"]
    BtrfsRoot --> SubvolSnap["Subvolume: snapshots (~/crypto/data/snapshots)"]
```

- **Passphrase**: Encrypted in Ansible Vault as `luks_pass`.
- **LUKS Header Backup**: Saved locally at `/var/backups/luks/luks-header-<name>-<idx>.img` (permissions `0400`).
- **Filesystem Label**: Matches the storage volume name (e.g. `data`), mounted via `LABEL=data`.

---

## 🚨 Scenario 1: Emergency Manual Unlock Without Ansible

If Ansible is unavailable, or if booting into a Debian Live USB or rescue environment:

### 1. Identify the block device
```bash
ls -l /dev/disk/by-id/
```

### 2. Decrypt and open the LUKS containers
For each physical disk in the pool:
```bash
sudo cryptsetup luksOpen /dev/disk/by-id/<disk-1-id> data-0
sudo cryptsetup luksOpen /dev/disk/by-id/<disk-2-id> data-1  # If RAID1 pool
```
*(Enter the `luks_pass` passphrase when prompted).*

Scan opened member devices into the kernel Btrfs pool:
```bash
sudo btrfs device scan
```

### 3. Mount the Btrfs subvolumes
Mount by label (`LABEL=data`) so Btrfs automatically mounts the complete pool:
```bash
sudo mkdir -p /mnt/recovery/data/sync /mnt/recovery/data/snapshots

# Mount the sync working tree
sudo mount -t btrfs -o subvol=sync,compress=zstd:3,noatime LABEL=data /mnt/recovery/data/sync

# Mount the snapshots tree
sudo mount -t btrfs -o subvol=snapshots,compress=zstd:3,noatime LABEL=data /mnt/recovery/data/snapshots
```

### 4. Unmount and close after operations
```bash
sudo umount /mnt/recovery/data/sync
sudo umount /mnt/recovery/data/snapshots
sudo cryptsetup luksClose data-0
sudo cryptsetup luksClose data-1  # If RAID1 pool
```

---

## 🔄 Scenario 2: Restoring Files from Btrfs Snapshots

Snapshots are stored as read-only Btrfs subvolumes under `~/crypto/data/snapshots/` in the format `YYYY-MM-DD--HH-MM-SS`.

### 1. List available snapshots
```bash
ls -lt ~/crypto/data/snapshots/
```

### 2. Restore an individual file or directory
Because snapshots are regular read-only directory trees, you can copy or `rsync` files directly back to `~/crypto/data/sync/`:

```bash
# Example: restoring a deleted directory
rsync -aHAXv ~/crypto/data/snapshots/2026-09-24--09-06-44/path/to/folder/ ~/crypto/data/sync/path/to/folder/
```

### 3. Trigger a manual snapshot
You can execute an ad-hoc snapshot at any time using the reusable CLI script:

```bash
$HOME/crypto/bin/snapshot.sh $HOME/crypto/data/sync $HOME/crypto/data/snapshots
```
Custom retention (e.g. 14 daily, 8 weekly) can be passed optionally:
```bash
$HOME/crypto/bin/snapshot.sh $HOME/crypto/data/sync $HOME/crypto/data/snapshots 14 8
```

---

## 🛡️ Scenario 3: Corrupted LUKS Header Recovery

The LUKS on-disk header contains the cipher parameters, salt, and keyslots. If bad sectors develop or the header is accidentally overwritten, data cannot be decrypted even with the correct passphrase.

The `crypto` role automatically maintains a header backup for each member device at `/var/backups/luks/luks-header-<name>-<idx>.img`.

### 1. Verify header corruption
```bash
sudo cryptsetup luksDump /dev/disk/by-id/<disk-id>
```
If this returns `Device is not a valid LUKS device`, the header is corrupted.

### 2. Restore the header from backup
```bash
sudo cryptsetup luksHeaderRestore /dev/disk/by-id/<disk-id> \
  --header-backup-file /var/backups/luks/luks-header-<name>-<idx>.img
```
Type `YES` in uppercase when prompted to confirm restoration.

### 3. Verify access
```bash
sudo cryptsetup luksOpen /dev/disk/by-id/<disk-id> <name>-<idx>
```

---

## 🖴 Scenario 4: Physical Disk Replacement

When replacing a failed hard drive or SSD with a new drive:

### 1. Install the new physical disk and retrieve its persistent ID
```bash
ls -l /dev/disk/by-id/
```

### 2. Update host variables
Edit `ansible/envs/<env>/host_vars/<hostname>/main.yml` with the new disk ID:
```yaml
crypto_disks:
  - name: data
    devices:
      - /dev/disk/by-id/ata-Samsung_SSD_870_EVO_<serial_1>
      # - /dev/disk/by-id/ata-Samsung_SSD_870_EVO_<serial_2>  # If RAID1 pool
```

### 3. Run the destructive provisioning playbook
```bash
uv run ansible-playbook -i ansible/envs/<env>/inventory.ini ansible/wipe-and-reencrypt.yml
```
This will:
- Partition and encrypt the new drive with LUKS.
- Format direct Btrfs with compression.
- Create declarative subvolumes (`sync`, `snapshots`) nested under `~/crypto/data/`.
- Configure `/etc/fstab` and backup cron jobs.
- Generate a new LUKS header backup in `/var/backups/luks/`.

### 4. Trigger initial cloud resync
```bash
$HOME/crypto/bin/rclone.sh onedrive
$HOME/crypto/bin/rclone.sh S3 delarche-images
```

---

## 🩺 Scenario 5: Routine Integrity Verification (Btrfs Scrub)

A monthly scrub cron job is automatically configured. To manually run and check data integrity against bit rot:

```bash
# Start scrub in foreground with progress
sudo btrfs scrub start -B ~/crypto/data

# Check last scrub status and error counters
sudo btrfs scrub status ~/crypto/data
```
Verify that `csum_errors`, `verify_errors`, and `uncorrectable_errors` report `0`.
