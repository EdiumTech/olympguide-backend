"""Repository-local preparation and SSH deployment, using Python's standard library."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shlex
import shutil
import subprocess
import sys
import tarfile
import time
import urllib.request
import urllib.error

ROOT = Path(__file__).resolve().parents[1]
PRIVATE = ROOT / '.private'
ENV = PRIVATE / 'backend.env'
KEY = PRIVATE / 'deploy_ed25519'
FOLDER = 'b1gum9p3p8pqr9jincsb'
DEPLOY_DIRS = ('api', 'storage_service', 'email_service', 'diploma_loader_service', 'migrations', 'deploy')
EXCLUDED_DIRS = {'.git', '.venv', '__pycache__', 'data', '.tools', '.private'}


def tool(name):
    local = ROOT / '.tools' / (name + ('.exe' if os.name == 'nt' else ''))
    return str(local) if local.exists() else shutil.which(name) or name


def run(args, **kwargs):
    return subprocess.run([str(x) for x in args], check=True, **kwargs)


def private_directory():
    PRIVATE.mkdir(mode=0o700, exist_ok=True)
    if os.name != 'nt':
        PRIVATE.chmod(0o700)


def prepare():
    private_directory()
    if not ENV.exists():
        values = {'API_DOMAIN': 'api.olympguide.ru', 'DATA_ROOT': '/srv/olympguide/data'}
        for name in ('DB_PASSWORD', 'REDIS_PASSWORD', 'MINIO_PASSWORD', 'TOKEN_SECRET', 'SESSION_SECRET', 'BEARER_DATA_LOADER_TOKEN'):
            values[name] = secrets.token_hex(32)
        values.update(COMPOSE_PROFILES='', SMTP_SERVER='smtp.mail.ru', SMTP_PORT='587', SMTP_USERNAME='', SMTP_PASSWORD='')
        with ENV.open('x', encoding='utf-8', newline='\n') as stream:
            stream.write('\n'.join(f'{key}={value}' for key, value in values.items()) + '\n')
        ENV.chmod(0o600)
        print('Created .private/backend.env (secrets are not printed).')
    else:
        print('Kept existing .private/backend.env; secrets were not rotated.')
    if not KEY.exists():
        run(['ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-f', KEY, '-C', 'olympguide-deploy'])
    print('Public SSH key: .private/deploy_ed25519.pub')


def inventory():
    yc = [tool('yc')]
    config = PRIVATE / 'yc-config.yaml'
    if config.exists():
        yc += ['--config', config]
    commands = {
        'instances': ['compute', 'instance', 'list'],
        'addresses': ['vpc', 'address', 'list'],
        'disks': ['compute', 'disk', 'list'],
        'dns_zones': ['dns', 'zone', 'list'],
    }
    result = {}
    for name, command in commands.items():
        proc = run(yc + command + ['--folder-id', FOLDER, '--format', 'json'], capture_output=True, text=True, encoding='utf-8')
        result[name] = json.loads(proc.stdout)
    for zone in result['dns_zones']:
        if zone.get('zone') == 'olympguide.ru.':
            proc = run(yc + ['dns', 'zone', 'list-records', '--id', zone['id'], '--format', 'json'], capture_output=True, text=True, encoding='utf-8')
            result['dns_records'] = json.loads(proc.stdout)
    private_directory()
    (PRIVATE / 'inventory.json').write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf-8')
    # Instance metadata can contain unrelated secrets; never print the raw inventory.
    for kind in commands:
        print(kind + ': ' + ', '.join(f"{x.get('name', '?')} ({x['id']})" for x in result[kind]))
    print('Saved full inventory locally in .private/inventory.json. Review existing resources before any apply.')


def archive():
    private_directory()
    stamp = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    target = PRIVATE / 'release.tar.gz'
    with tarfile.open(target, 'w:gz') as tar:
        for dirname in DEPLOY_DIRS:
            for path in sorted((ROOT / dirname).rglob('*')):
                rel = path.relative_to(ROOT)
                if not path.is_file() or path.is_symlink() or any(part in EXCLUDED_DIRS for part in rel.parts):
                    continue
                if path.name.startswith('.env') or path.suffix in {'.exe', '.pyc', '.pdf', '.zip', '.log'}:
                    continue
                if path.name in {'main', 'main.exe'}:
                    continue
                tar.add(path, arcname=rel.as_posix(), recursive=False)
    digest = hashlib.sha256(target.read_bytes()).hexdigest()
    release = stamp + '-' + digest[:8]
    print(f'Release {release}; archive {target.stat().st_size:,} bytes; SHA256 {digest}')
    return release, target, digest


def ssh_options(key, known_hosts, jump_host=None, port=22):
    hosts_path = known_hosts.as_posix()
    if any(char.isspace() for char in hosts_path):
        hosts_path = '"' + hosts_path + '"'
    options = ['-i', key.as_posix(), '-o', 'IdentitiesOnly=yes', '-o', 'StrictHostKeyChecking=yes', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15',
               '-o', 'UserKnownHostsFile=' + hosts_path]
    if jump_host:
        import ipaddress
        ipaddress.IPv4Address(jump_host)
        command = ['ssh', *options, '-W', '%h:%p', f'deploy@{jump_host}']
        options += ['-o', 'ProxyCommand=' + (subprocess.list2cmdline(command) if os.name == 'nt' else shlex.join(command))]
    options += ['-o', f'Port={port}']
    return options


def deploy(host, key, known_hosts, jump_host=None, port=22):
    import ipaddress
    ipaddress.IPv4Address(host)
    if not ENV.exists() or not key.is_file() or not known_hosts.is_file():
        raise SystemExit('Prepare .private/backend.env, an SSH key and a verified known_hosts file first.')
    # Password changes require an explicit database rotation; never replace a live env here.
    values = dict(line.split('=', 1) for line in ENV.read_text().splitlines() if line and not line.startswith('#'))
    if 'email' in values.get('COMPOSE_PROFILES', '').split(','):
        if not values.get('SMTP_USERNAME') or not values.get('SMTP_PASSWORD'):
            raise SystemExit('Email profile needs SMTP_USERNAME and SMTP_PASSWORD.')
    release, package, digest = archive()
    options = ssh_options(key, known_hosts, jump_host, port)
    destination = f'deploy@{host}'
    run(['ssh', *options, destination, 'sudo cloud-init status --wait && test -f /srv/olympguide/.bootstrap-complete'])
    run(['ssh', *options, destination, f'umask 077 && mkdir -p /srv/olympguide/releases/{release}'])
    run(['scp', *options, package, f'{destination}:/srv/olympguide/releases/{release}/release.tar.gz'])
    # Environment stays encrypted in transit and out of process arguments / Terraform state.
    with ENV.open('rb') as stream:
        run(['ssh', *options, destination,
             'sudo sh -c \'umask 077; if test -f /srv/olympguide/secrets/backend.env; then '
             'cmp -s - /srv/olympguide/secrets/backend.env || { echo "Remote env differs; use an explicit secret rotation" >&2; exit 1; }; '
             'else cat > /srv/olympguide/secrets/backend.env; fi\''], stdin=stream)
    unit = 'olympguide-deploy-' + release
    command = (
        f"cd /srv/olympguide/releases/{release} && "
        f"printf '%s  %s\\n' '{digest}' release.tar.gz | sha256sum --check --status && "
        "tar xzf release.tar.gz && chmod +x deploy/*.sh && "
        f"sudo systemd-run --unit={unit} --property=Type=oneshot --property=RemainAfterExit=yes --property=TimeoutStartSec=1800 "
        f"--no-block /bin/bash /srv/olympguide/releases/{release}/deploy/activate.sh {release}"
    )
    run(['ssh', *options, destination, command])
    (PRIVATE / 'deployment-job.json').write_text(json.dumps({'host': host, 'release': release, 'unit': unit}, indent=2), encoding='utf-8')
    print('Deployment runs on the VM independently of this SSH connection.', flush=True)
    deadline = time.monotonic() + 1800
    last_output = ''
    connection_failed = False
    while time.monotonic() < deadline:
        status_command = (f'sudo systemctl show {unit} -p ActiveState -p SubState -p Result -p ExecMainStatus; '
                          f'sudo journalctl -u {unit} -n 8 --no-pager -o cat')
        result = subprocess.run(['ssh', *options, destination, status_command], capture_output=True, text=True, encoding='utf-8', errors='replace')
        if result.returncode:
            if not connection_failed:
                print('SSH temporarily unavailable; the deployment job continues on the VM.', flush=True)
            connection_failed = True
            time.sleep(10)
            continue
        connection_failed = False
        state = dict(line.split('=', 1) for line in result.stdout.splitlines()
                     if line.startswith(('ActiveState=', 'SubState=', 'Result=', 'ExecMainStatus=')))
        if result.stdout != last_output:
            print(result.stdout.rstrip(), flush=True)
            last_output = result.stdout
        if state.get('ActiveState') == 'failed':
            raise SystemExit('Remote deployment failed; inspect the retained systemd job: ' + unit)
        if state.get('ActiveState') == 'active' and state.get('SubState') == 'exited':
            if state.get('Result') != 'success' or state.get('ExecMainStatus') != '0':
                raise SystemExit('Remote deployment did not complete successfully: ' + unit)
            return
        if state.get('ActiveState') == 'inactive':
            raise SystemExit('Remote deployment job is no longer available; inspect its journal before retrying: ' + unit)
        time.sleep(10)
    raise SystemExit('Timed out waiting; check the remote job before retrying: ' + unit)


def verify(domain):
    if not re.fullmatch(r'[a-z0-9][a-z0-9.-]+[a-z0-9]', domain):
        raise SystemExit('Expected a DNS hostname.')
    for path in ('/healthz', '/readyz', '/api/v1/universities', '/api/v1/olympiads'):
        with urllib.request.urlopen('https://' + domain + path, timeout=20) as response:
            payload = json.load(response)
            if path in ('/api/v1/universities', '/api/v1/olympiads') and not isinstance(payload, list):
                raise SystemExit(f'{path}: expected a JSON array, got {type(payload).__name__}')
            print(f'{path}: HTTPS {response.status}; valid JSON')
    print('HTTPS/API checks passed. Check catalog population and SMTP separately.')


def main():
    # Windows consoles may use cp1251, while Docker/systemd logs contain Unicode.
    sys.stdout.reconfigure(encoding='utf-8', errors='replace')
    sys.stderr.reconfigure(encoding='utf-8', errors='replace')
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    for command in ('prepare', 'inventory', 'package'):
        commands.add_parser(command)
    dep = commands.add_parser('deploy')
    dep.add_argument('--host', required=True)
    dep.add_argument('--key', type=Path, default=KEY)
    dep.add_argument('--known-hosts', type=Path, default=PRIVATE / 'known_hosts')
    dep.add_argument('--jump-host', help='Optional administrator SSH gateway IPv4; uses the same verified key and known_hosts.')
    dep.add_argument('--port', type=int, default=22)
    check = commands.add_parser('verify')
    check.add_argument('--domain', default='api.olympguide.ru')
    args = parser.parse_args()
    try:
        if args.command == 'prepare':
            prepare()
        elif args.command == 'inventory':
            inventory()
        elif args.command == 'package':
            archive()
        elif args.command == 'deploy':
            if not 1 <= args.port <= 65535:
                raise SystemExit('SSH port must be between 1 and 65535.')
            deploy(args.host, args.key.resolve(), args.known_hosts.resolve(), args.jump_host, args.port)
        else:
            verify(args.domain)
    except subprocess.CalledProcessError as exc:
        if args.command == 'inventory':
            raise SystemExit('Cloud inventory failed. Authorize yc, then check folder access.') from exc
        raise SystemExit(f'{Path(str(exc.cmd[0])).name} failed with exit code {exc.returncode}') from exc
    except urllib.error.URLError as exc:
        raise SystemExit(f'HTTPS check failed: {exc.reason}') from exc


if __name__ == '__main__':
    main()
