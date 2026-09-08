"""Verify the VM's SSH key against the authenticated Yandex serial console."""
import argparse
import base64
import hashlib
import ipaddress
import json
from pathlib import Path
import re
import subprocess

from deploy import PRIVATE, ROOT, tool


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--instance-id', required=True)
    parser.add_argument('--host', required=True)
    args = parser.parse_args()
    ipaddress.IPv4Address(args.host)
    if not re.fullmatch('[a-z0-9]{20}', args.instance_id):
        raise SystemExit('Expected a Yandex Compute instance ID.')
    yc = [tool('yc')]
    config = PRIVATE / 'yc-config.yaml'
    if config.exists():
        yc += ['--config', str(config)]
    result = subprocess.run(yc + ['compute', 'instance', 'get-serial-port-output', '--id', args.instance_id, '--format', 'json'],
                            check=True, capture_output=True, text=True, encoding='utf-8')
    serial = json.loads(result.stdout)['contents']
    block = re.search(r'-----BEGIN SSH HOST KEY KEYS-----(.*?)-----END SSH HOST KEY KEYS-----', serial, re.S)
    if not block:
        raise SystemExit('Host keys are not in the serial console yet; wait for cloud-init to finish.')
    keys = re.findall(r'(ssh-ed25519 [A-Za-z0-9+/=]+) root@' + re.escape(args.instance_id), block.group(1))
    if len(keys) != 1:
        raise SystemExit('Expected exactly one Ed25519 host key.')
    algorithm, key = keys[0].split()
    fingerprint = 'SHA256:' + base64.b64encode(hashlib.sha256(base64.b64decode(key)).digest()).decode().rstrip('=')
    if fingerprint + ' root@' + args.instance_id not in serial:
        raise SystemExit('SSH key does not match the VM key in the authenticated serial console.')
    target = PRIVATE / 'known_hosts'
    existing = target.read_text().splitlines() if target.exists() else []
    entries = [line for line in existing if not line.startswith(args.host + ' ')]
    entries.append(f'{args.host} {algorithm} {key}')
    target.write_text('\n'.join(entries) + '\n', encoding='ascii')
    print('Verified SSH host key against Yandex API:', fingerprint)


if __name__ == '__main__':
    main()
