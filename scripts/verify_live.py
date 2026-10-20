#!/usr/bin/env python3
"""
Live telemetry and dashboard non-regression validation engine.
Verifies that:
1. Dashboard variable selectors ($host) are populated with active hosts.
2. The preselected host is present and online in the fleet.
3. Every panel in each dashboard returns valid telemetry data without query errors.
4. Rendered PNG snapshots are non-empty and visually populated.
"""

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
from typing import Any, Dict, List, Optional, Tuple

GCX_CONTEXT = "domus"
PROMETHEUS_UID = "grafanacloud-prom"
LOKI_UID = "grafanacloud-logs"

# Patterns indicating queries that naturally return 0 series when the system is healthy
ZERO_IS_HEALTHY_PATTERNS = [
    r'state="failed"',
    r'error',
    r'_drop_total',
    r'_errs_total',
    r'node_vmstat_oom_kill',
    r'prometheus_remote_write_dropped',
    r'prometheus_remote_write_retries',
    r'prometheus_remote_storage_samples_failed_total',
    r'prometheus_remote_storage_samples_retried_total',
    r'loki_write_batch_retries_total',
    r'loki_write_dropped',
    r'node_btrfs_device_errors_total',
    r'reallocated',
    r'pending',
    r'uncorrectable',
    r'crc',
    r'pswpin',
    r'pswpout',
    r'Retrans',
    r'readonly',
]


def run_gcx_api(path: str, method: str = "GET", data: Optional[Dict[str, Any]] = None) -> Any:
    cmd = ["gcx", "--context", GCX_CONTEXT, "api", path, "-X", method]
    if data is not None:
        cmd.extend(["-H", "Content-Type: application/json", "-d", json.dumps(data)])
    res = subprocess.run(cmd, capture_output=True, text=True)
    if res.returncode != 0:
        raise RuntimeError(f"gcx api {path} failed: {res.stderr.strip() or res.stdout.strip()}")
    return json.loads(res.stdout)


def get_available_hosts() -> List[str]:
    path = f"/api/datasources/uid/{PROMETHEUS_UID}/resources/api/v1/label/host/values"
    data = run_gcx_api(path)
    return data.get("data", [])


def verify_variable(dashboard: Dict[str, Any], target_host: str, available_hosts: List[str]) -> Tuple[bool, str]:
    templating = dashboard.get("templating", {}).get("list", [])
    host_vars = [v for v in templating if v.get("name") == "host"]
    if not host_vars:
        return False, "No $host template variable defined in dashboard"

    var = host_vars[0]
    preselected = var.get("current", {}).get("value")

    if not available_hosts:
        return False, "Prometheus label values query returned 0 hosts"

    if preselected not in available_hosts:
        return False, f"Preselected host {preselected!r} is not in available hosts: {available_hosts}"

    query_spec = var.get("query", {})
    if isinstance(query_spec, dict):
        qry_type = query_spec.get("qryType")
        if qry_type != 1:
            return False, f"Variable query qryType is {qry_type}, expected 1 for label_values"

    return True, f"Found {len(available_hosts)} active hosts ({', '.join(available_hosts)}); preselected: {preselected}"


def execute_ds_query(expr: str, ds_type: str, ds_uid: str, is_instant: bool, max_lines: int, time_from: str, time_to: str) -> Tuple[bool, int, str]:
    query_payload: Dict[str, Any] = {
        "refId": "A",
        "datasource": {"uid": ds_uid, "type": ds_type},
        "expr": expr,
        "maxDataPoints": 1000,
    }

    if is_instant:
        query_payload["instant"] = True

    if ds_type == "loki":
        if is_instant:
            query_payload["queryType"] = "instant"
        else:
            query_payload["queryType"] = "range"
            query_payload["maxLines"] = max_lines

    payload = {
        "queries": [query_payload],
        "from": time_from,
        "to": time_to,
    }

    try:
        data = run_gcx_api("/api/ds/query", method="POST", data=payload)
    except Exception as e:
        return False, 0, f"API error: {e}"

    res_a = data.get("results", {}).get("A", {})
    if "error" in res_a:
        return False, 0, f"Downstream query error: {res_a['error']}"

    frames = res_a.get("frames", [])
    if not frames:
        return True, 0, "0 frames"

    total_points = 0
    for frame in frames:
        for val_list in frame.get("data", {}).get("values", []):
            total_points += len(val_list)

    return True, total_points, f"{total_points} data points"


def verify_panel_query(
    panel_title: str,
    target: Dict[str, Any],
    host: str,
    available_hosts: List[str],
    time_from: str,
    time_to: str,
) -> Tuple[bool, str]:
    expr = target.get("expr", "").strip()
    if not expr:
        return True, "No query expression (skipped)"

    ds = target.get("datasource", {})
    ds_type = ds.get("type", "prometheus")
    ds_uid = ds.get("uid", PROMETHEUS_UID)
    is_instant = bool(target.get("instant", False))
    max_lines = int(target.get("maxLines", 500))

    rendered_expr = expr.replace("$host", host)
    ok, points, msg = execute_ds_query(rendered_expr, ds_type, ds_uid, is_instant, max_lines, time_from, time_to)

    if not ok:
        return False, msg

    if points > 0:
        return True, f"{points} data points"

    # If 0 points, check if this is an expected zero-error or healthy metric
    for pattern in ZERO_IS_HEALTHY_PATTERNS:
        if re.search(pattern, rendered_expr, re.IGNORECASE):
            return True, f"0 data points (healthy state / zero errors for {pattern})"

    # If host-specific (e.g. Btrfs chunks), test across alternative online hosts
    for alt_host in available_hosts:
        if alt_host == host:
            continue
        alt_expr = expr.replace("$host", alt_host)
        alt_ok, alt_points, _ = execute_ds_query(alt_expr, ds_type, ds_uid, is_instant, max_lines, time_from, time_to)
        if alt_ok and alt_points > 0:
            return True, f"0 points on {host}, but {alt_points} points on {alt_host} (host-specific service)"

    return False, f"Empty result (0 points) across all hosts for expr: {rendered_expr}"


def sanitize_slug(s: str) -> str:
    s = s.lower().replace("/", "-").replace("_", "-").replace(".", "-")
    s = re.sub(r"[^a-z0-9-]+", "-", s).strip("-")
    if len(s) > 14:
        import hashlib
        h = hashlib.sha256(s.encode()).hexdigest()[:5]
        prefix = s[:8].rstrip("-")
        s = f"{prefix}-{h}"
    return s


def main():
    parser = argparse.ArgumentParser(description="Live dashboard validation tool")
    parser.add_argument("--branch", default="", help="Branch preview slug to test")
    parser.add_argument("--pr", default="", help="Pull request number to test (e.g. 12)")
    parser.add_argument("--prod", action="store_true", help="Validate production dashboards (domus folder)")
    parser.add_argument("--host", default="kodlix.delarche.fr", help="Host to validate against")
    parser.add_argument("--folder", default="", help="Specific folder UID to inspect")
    parser.add_argument("--skip-snapshots", action="store_true", help="Skip headless PNG snapshot render check")
    args = parser.parse_args()

    if args.prod:
        dash_suffix = ""
        folder_uid = "domus"
    elif args.pr:
        dash_suffix = f"-pr-{args.pr}"
        folder_uid = f"domus-pr-{args.pr}"
    elif args.branch:
        slug = sanitize_slug(args.branch)
        dash_suffix = f"-p-{slug}"
        folder_uid = f"domus-prev-{slug}"
    else:
        slug = "ci-test"
        dash_suffix = f"-p-{slug}"
        folder_uid = f"domus-prev-{slug}"

    if args.folder:
        folder_uid = args.folder

    print(f"\033[1;34m[INFO]\033[0m Starting live telemetry verification on folder {folder_uid}...")
    print(f"\033[1;34m[INFO]\033[0m Primary target test host: {args.host}")

    try:
        available_hosts = get_available_hosts()
    except Exception as e:
        print(f"\033[1;31m[ERROR]\033[0m Failed to query available hosts from Prometheus: {e}")
        sys.exit(1)

    if not available_hosts:
        print("\033[1;31m[ERROR]\033[0m Prometheus returned 0 active hosts! Check agent collectors.")
        sys.exit(1)

    print(f"\033[1;34m[INFO]\033[0m Discovered {len(available_hosts)} active hosts in Mimir: {', '.join(available_hosts)}")

    dashboard_uids = [
        f"domus-host{dash_suffix}",
        f"domus-storage{dash_suffix}",
        f"domus-backups{dash_suffix}",
        f"domus-alloy-health{dash_suffix}",
    ]

    total_checks = 0
    failed_checks = 0

    for dash_uid in dashboard_uids:
        print(f"\n\033[1;36m=== Validating Dashboard: {dash_uid} ===\033[0m")
        try:
            res = run_gcx_api(f"/api/dashboards/uid/{dash_uid}")
        except Exception as e:
            print(f"  \033[1;31m[FAIL]\033[0m Failed to fetch dashboard: {e}")
            failed_checks += 1
            continue

        dash = res.get("dashboard", {})
        title = dash.get("title", dash_uid)
        time_from = dash.get("time", {}).get("from", "now-1h")
        time_to = dash.get("time", {}).get("to", "now")

        # 1. Variable check
        total_checks += 1
        var_ok, var_msg = verify_variable(dash, args.host, available_hosts)
        if var_ok:
            print(f"  \033[1;32m[PASS]\033[0m Host Variable: {var_msg}")
        else:
            print(f"  \033[1;31m[FAIL]\033[0m Host Variable: {var_msg}")
            failed_checks += 1

        # 2. Panels check
        panels = dash.get("panels", [])
        print(f"  \033[1;34m[INFO]\033[0m Inspecting {len(panels)} panels (Window: {time_from} to {time_to})...")
        for panel in panels:
            p_title = panel.get("title", "Untitled")
            targets = panel.get("targets", [])
            for target in targets:
                total_checks += 1
                p_ok, p_msg = verify_panel_query(p_title, target, args.host, available_hosts, time_from, time_to)
                if p_ok:
                    print(f"    \033[1;32m[PASS]\033[0m Panel \"{p_title}\": {p_msg}")
                else:
                    print(f"    \033[1;31m[FAIL]\033[0m Panel \"{p_title}\": {p_msg}")
                    failed_checks += 1

        # 3. Snapshot rendering check
        if not args.skip_snapshots:
            total_checks += 1
            with tempfile.TemporaryDirectory() as tmp_dir:
                snap_cmd = [
                    "gcx", "--context", GCX_CONTEXT, "dashboards", "snapshot",
                    dash_uid, "--var", f"host={args.host}", "--output-dir", tmp_dir
                ]
                snap_res = subprocess.run(snap_cmd, capture_output=True, text=True)
                png_file = os.path.join(tmp_dir, f"{dash_uid}.png")
                if snap_res.returncode == 0 and os.path.exists(png_file):
                    size_kb = os.path.getsize(png_file) / 1024.0
                    if size_kb > 60:
                        print(f"  \033[1;32m[PASS]\033[0m Snapshot Rendering: {size_kb:.1f} KB (Populated)")
                    else:
                        print(f"  \033[1;31m[FAIL]\033[0m Snapshot Rendering: {size_kb:.1f} KB (< 60 KB, likely empty)")
                        failed_checks += 1
                else:
                    err_msg = snap_res.stderr.strip() or snap_res.stdout.strip()
                    print(f"  \033[1;31m[FAIL]\033[0m Snapshot Rendering failed: {err_msg}")
                    failed_checks += 1

    print("\n-------------------------------------------------------------")
    if failed_checks == 0:
        print(f"\033[1;32m[SUCCESS]\033[0m All {total_checks} live checks passed successfully!")
        sys.exit(0)
    else:
        print(f"\033[1;31m[ERROR]\033[0m {failed_checks} of {total_checks} checks failed!")
        sys.exit(1)


if __name__ == "__main__":
    main()
