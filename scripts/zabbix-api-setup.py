#!/usr/bin/env python3
"""
Configures zabbix-postfix on the Zabbix server via JSON-RPC API using an API token.

Steps:
  --import   Import template_postfix_passive.xml
  --link     Link template to hosts
  --macros   Create threshold macros on hosts (2x template defaults)
  --group    Create MAIL_SERVERS host group and add hosts
  --script   Create "Reset Postfix offset" remote script

Usage:
    python3 scripts/zabbix-api-setup.py -u http://zabbix.example.com -t <TOKEN> -H 10683,10684
    python3 scripts/zabbix-api-setup.py -u http://zabbix.example.com -t <TOKEN> --import
"""
import argparse
import json
import os
import sys
import urllib.request
from pathlib import Path

REPO_ROOT    = Path(__file__).resolve().parent.parent
TEMPLATE_XML = REPO_ROOT / "template_postfix_passive.xml"
TEMPLATE_NAME = "Template App Postfix by Zabbix agent (passive)"

# Threshold macros — 2x the template defaults
MACROS = [
    ("{$POSTFIX_MAILQ_WARN}",    "200"),
    ("{$POSTFIX_DEFERRED_WARN}", "100"),
    ("{$POSTFIX_REJECTED_WARN}", "2000"),
]


# ── API client ─────────────────────────────────────────────────────────────────

class ZabbixAPI:
    def __init__(self, url: str, token: str):
        self.url   = url.rstrip("/") + "/api_jsonrpc.php"
        self.token = token
        self._id   = 1

    def call(self, method: str, params: dict | list) -> dict | list:
        payload = {
            "jsonrpc": "2.0",
            "method":  method,
            "params":  params,
            "auth":    self.token,
            "id":      self._id,
        }
        self._id += 1

        req = urllib.request.Request(
            self.url,
            data=json.dumps(payload).encode(),
            headers={"Content-Type": "application/json"},
        )
        resp = json.loads(urllib.request.urlopen(req, timeout=15).read())
        if "error" in resp:
            raise RuntimeError(f"API error [{method}]: {resp['error']['data']}")
        return resp["result"]


# ── helpers ────────────────────────────────────────────────────────────────────

def ok(msg: str)   -> None: print(f"[OK]  {msg}")
def info(msg: str) -> None: print(f"[..] {msg}")
def skip(msg: str) -> None: print(f"[--] {msg}")


# ── steps ──────────────────────────────────────────────────────────────────────

def step_import(zbx: ZabbixAPI) -> None:
    info(f"Importing template from {TEMPLATE_XML}...")
    if not TEMPLATE_XML.exists():
        raise FileNotFoundError(f"Template not found: {TEMPLATE_XML}")

    xml = TEMPLATE_XML.read_text()
    # Remove <date> tag — rejected by Zabbix 7.0 API
    xml = "\n".join(line for line in xml.splitlines() if "<date>" not in line)

    zbx.call("configuration.import", {
        "format": "xml",
        "rules": {
            "template_groups": {"createMissing": True},
            "templates":       {"createMissing": True, "updateExisting": True},
            "items":           {"createMissing": True, "updateExisting": True, "deleteMissing": False},
            "triggers":        {"createMissing": True, "updateExisting": True, "deleteMissing": False},
            "graphs":          {"createMissing": True, "updateExisting": True, "deleteMissing": False},
            "valueMaps":       {"createMissing": True, "updateExisting": True},
        },
        "source": xml,
    })
    ok("Template imported")


def step_link(zbx: ZabbixAPI, host_ids: list[str]) -> None:
    if not host_ids:
        skip("No hosts specified — skipping template link")
        return

    info("Looking up template ID...")
    result = zbx.call("template.get", {
        "output": ["templateid"],
        "filter": {"name": TEMPLATE_NAME},
    })
    if not result:
        raise RuntimeError(f"Template '{TEMPLATE_NAME}' not found — run --import first")
    template_id = result[0]["templateid"]

    info(f"Linking template {template_id} to {len(host_ids)} host(s)...")
    zbx.call("host.massadd", {
        "hosts":     [{"hostid": hid} for hid in host_ids],
        "templates": [{"templateid": template_id}],
    })
    ok(f"Template linked to hosts: {', '.join(host_ids)}")


def step_macros(zbx: ZabbixAPI, host_ids: list[str]) -> None:
    if not host_ids:
        skip("No hosts specified — skipping macros")
        return

    info(f"Creating threshold macros on {len(host_ids)} host(s)...")
    payload = [
        {"hostid": hid, "macro": macro, "value": value}
        for hid in host_ids
        for macro, value in MACROS
    ]
    zbx.call("usermacro.create", payload)
    macro_str = ", ".join(f"{m}={v}" for m, v in MACROS)
    ok(f"Macros created: {macro_str}")


def step_group(zbx: ZabbixAPI, host_ids: list[str]) -> str:
    info("Creating MAIL_SERVERS host group...")
    group_id = _get_or_create_group(zbx, "MAIL_SERVERS")
    ok(f"MAIL_SERVERS group ready (groupid={group_id})")

    if host_ids:
        info(f"Adding {len(host_ids)} host(s) to MAIL_SERVERS...")
        zbx.call("hostgroup.massadd", {
            "groups": [{"groupid": group_id}],
            "hosts":  [{"hostid": hid} for hid in host_ids],
        })
        ok("Hosts added to group")

    return group_id


def step_script(zbx: ZabbixAPI, group_id: str) -> None:
    if not group_id:
        info("Looking up MAIL_SERVERS group...")
        result = zbx.call("hostgroup.get", {
            "output": ["groupid"],
            "filter": {"name": "MAIL_SERVERS"},
        })
        if not result:
            skip("MAIL_SERVERS group not found — run --group first or provide group ID")
            return
        group_id = result[0]["groupid"]

    info(f"Creating 'Reset Postfix offset' script (groupid={group_id})...")
    zbx.call("script.create", {
        "name":        "Reset Postfix offset",
        "command":     "sudo /opt/zabbix_postfix/zabbix-reset-offset.sh",
        "scope":       "2",   # manual host action
        "type":        "0",   # script (bash)
        "execute_on":  "0",   # Zabbix agent
        "description": (
            "Resets the pygtail offset to the end of mail.log and clears the stats file. "
            "Use when Postfix counters accumulate historical data on first collection."
        ),
        "groupid":     group_id,
        "usrgrpid":    "7",   # Zabbix administrators
        "host_access": "2",   # write
    })
    ok("Script created — available at Monitoring → Hosts → (host) → Scripts")


# ── internal ───────────────────────────────────────────────────────────────────

def _get_or_create_group(zbx: ZabbixAPI, name: str) -> str:
    try:
        result = zbx.call("hostgroup.create", {"name": name})
        return result["groupids"][0]
    except RuntimeError as e:
        if "already exists" not in str(e):
            raise
        result = zbx.call("hostgroup.get", {
            "output": ["groupid"],
            "filter": {"name": name},
        })
        return result[0]["groupid"]


# ── CLI ────────────────────────────────────────────────────────────────────────

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Configure zabbix-postfix on the Zabbix server via API token.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Steps (default: all):
  --import   Import template_postfix_passive.xml
  --link     Link template to hosts (-H required)
  --macros   Create threshold macros on hosts (-H required)
  --group    Create MAIL_SERVERS group and add hosts
  --script   Create "Reset Postfix offset" remote script

How to get an API token:
  python3 scripts/zabbix-create-token.py -u http://zabbix.example.com -U Admin -p secret

Examples:
  # Full setup
  python3 scripts/zabbix-api-setup.py \\
      -u http://zabbix.example.com -t <TOKEN> \\
      -H 10683,10684,10685,10686,10687,10688,10689

  # Import template only
  python3 scripts/zabbix-api-setup.py -u http://zabbix.example.com -t <TOKEN> --import

  # Using environment variables
  export ZABBIX_URL=http://zabbix.example.com
  export ZABBIX_TOKEN=<token>
  python3 scripts/zabbix-api-setup.py -H 10683,10684
        """,
    )
    parser.add_argument("-u", "--url",     default=os.environ.get("ZABBIX_URL"),
                        help="Zabbix server URL  [env: ZABBIX_URL]")
    parser.add_argument("-t", "--token",   default=os.environ.get("ZABBIX_TOKEN"),
                        help="Zabbix API token   [env: ZABBIX_TOKEN]")
    parser.add_argument("-H", "--hosts",   default=os.environ.get("ZABBIX_HOSTIDS", ""),
                        help="Comma-separated host IDs  [env: ZABBIX_HOSTIDS]")

    steps = parser.add_argument_group("steps (default: all)")
    steps.add_argument("--import",  dest="do_import",  action="store_true")
    steps.add_argument("--link",    dest="do_link",    action="store_true")
    steps.add_argument("--macros",  dest="do_macros",  action="store_true")
    steps.add_argument("--group",   dest="do_group",   action="store_true")
    steps.add_argument("--script",  dest="do_script",  action="store_true")

    return parser.parse_args()


def main() -> None:
    args = parse_args()

    if not args.url:
        print("[ERR] Zabbix URL required (-u or ZABBIX_URL)", file=sys.stderr)
        sys.exit(1)
    if not args.token:
        print("[ERR] API token required (-t or ZABBIX_TOKEN)", file=sys.stderr)
        print("      Generate one with: python3 scripts/zabbix-create-token.py", file=sys.stderr)
        sys.exit(1)

    # If no step flag given, run all
    run_all = not any([args.do_import, args.do_link, args.do_macros, args.do_group, args.do_script])

    host_ids = [h.strip() for h in args.hosts.split(",") if h.strip()] if args.hosts else []

    zbx = ZabbixAPI(args.url, args.token)

    # Verify token
    info(f"Connecting to {args.url}...")
    zbx.call("apiinfo.version", {})
    ok("Token valid")
    print()

    group_id = ""

    if run_all or args.do_import:
        step_import(zbx)

    if run_all or args.do_link:
        step_link(zbx, host_ids)

    if run_all or args.do_macros:
        step_macros(zbx, host_ids)

    if run_all or args.do_group:
        group_id = step_group(zbx, host_ids)

    if run_all or args.do_script:
        step_script(zbx, group_id)

    print()
    ok("Done.")


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, FileNotFoundError) as e:
        print(f"\n[ERR] {e}", file=sys.stderr)
        sys.exit(1)
    except KeyboardInterrupt:
        sys.exit(1)
