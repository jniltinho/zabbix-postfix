#!/usr/bin/env python3
"""
Creates a Zabbix API token from username and password.

Usage:
    python3 scripts/zabbix-create-token.py -u http://zabbix.example.com -U Admin -p secret
    python3 scripts/zabbix-create-token.py -u http://zabbix.example.com -U Admin -p secret --name "zabbix-postfix"
"""
import argparse
import json
import sys
import urllib.request
from datetime import datetime, timedelta


class ZabbixAPI:
    def __init__(self, url: str):
        self.url = url.rstrip("/") + "/api_jsonrpc.php"
        self._id = 1

    def call(self, method: str, params: dict, auth: str | None = None) -> dict:
        payload = {
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
            "id": self._id,
        }
        if auth:
            payload["auth"] = auth
        self._id += 1

        req = urllib.request.Request(
            self.url,
            data=json.dumps(payload).encode(),
            headers={"Content-Type": "application/json"},
        )
        resp = json.loads(urllib.request.urlopen(req, timeout=10).read())
        if "error" in resp:
            raise RuntimeError(f"API error [{method}]: {resp['error']['data']}")
        return resp["result"]


def main():
    parser = argparse.ArgumentParser(
        description="Create a Zabbix API token from username and password.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python3 scripts/zabbix-create-token.py -u http://zabbix.example.com -U Admin -p secret
  python3 scripts/zabbix-create-token.py -u http://zabbix.example.com -U Admin -p secret \\
      --name "zabbix-postfix" --expires 365
        """,
    )
    parser.add_argument("-u", "--url",      required=True,  help="Zabbix server URL")
    parser.add_argument("-U", "--username", default="Admin", help="Zabbix username (default: Admin)")
    parser.add_argument("-p", "--password", required=True,  help="Zabbix password")
    parser.add_argument("--name",    default="zabbix-postfix", help="Token name (default: zabbix-postfix)")
    parser.add_argument("--expires", type=int, default=0,
                        help="Token expiry in days (default: 0 = never expires)")
    parser.add_argument("--print-only", action="store_true",
                        help="Print the token only (useful for shell capture)")
    args = parser.parse_args()

    zbx = ZabbixAPI(args.url)

    # 1. Login
    if not args.print_only:
        print(f"[..] Authenticating as '{args.username}' on {args.url}...")
    session = zbx.call("user.login", {"username": args.username, "password": args.password})
    if not args.print_only:
        print(f"[OK] Login successful")

    # 2. Get current user ID
    user = zbx.call("user.get", {"output": ["userid", "username"], "filter": {"username": args.username}}, auth=session)
    userid = user[0]["userid"]

    # 3. Build token params
    token_params: dict = {
        "name":   args.name,
        "userid": userid,
        "status": "0",  # enabled
    }
    if args.expires > 0:
        expires_at = int((datetime.now() + timedelta(days=args.expires)).timestamp())
        token_params["expires_at"] = str(expires_at)
        if not args.print_only:
            expiry_str = (datetime.now() + timedelta(days=args.expires)).strftime("%Y-%m-%d")
            print(f"[..] Token will expire on {expiry_str} ({args.expires} days)")
    else:
        token_params["expires_at"] = "0"
        if not args.print_only:
            print(f"[..] Token will never expire")

    # 4. Create token
    result = zbx.call("token.create", token_params, auth=session)
    tokenid = result["tokenids"][0]

    # 5. Generate the token value
    generated = zbx.call("token.generate", [tokenid], auth=session)
    token_value = generated[0]["token"]

    # 6. Logout
    zbx.call("user.logout", {}, auth=session)

    if args.print_only:
        print(token_value)
    else:
        print(f"\n[OK] API token created: '{args.name}'")
        print(f"\n  Token: {token_value}\n")
        print("Save this token — it will not be shown again.")
        print()
        print("Use it with zabbix-api-setup.py:")
        print(f"  python3 scripts/zabbix-api-setup.py -u {args.url} -t {token_value} -H <hostids>")
        print()
        print("Or export as environment variable:")
        print(f"  export ZABBIX_URL={args.url}")
        print(f"  export ZABBIX_TOKEN={token_value}")


if __name__ == "__main__":
    try:
        main()
    except RuntimeError as e:
        print(f"[ERR] {e}", file=sys.stderr)
        sys.exit(1)
    except KeyboardInterrupt:
        sys.exit(1)
