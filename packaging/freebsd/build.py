#!/usr/bin/env python3
"""Build a native FreeBSD pkg without installing it on the build host."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import tempfile

root = Path(__file__).resolve().parents[2]
if platform.system() != 'FreeBSD':
    raise SystemExit('Package creation requires FreeBSD and pkg.')
version = re.search(r'var version = "([^"]+)"', (root / 'main.go').read_text())[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--abi', choices=['FreeBSD:13:amd64', 'FreeBSD:15:amd64'],
                    help='Target pkg ABI; static Go binary compatibility is checked below')
args = parser.parse_args()
abi = args.abi or subprocess.check_output(['pkg', 'config', 'ABI'], text=True).strip()
if abi not in ['FreeBSD:13:amd64', 'FreeBSD:15:amd64']:
    raise SystemExit('Validated packaging targets are FreeBSD 13/15 amd64 only.')
out = root / 'dist'
out.mkdir(exist_ok=True)
with tempfile.TemporaryDirectory(prefix='solar-pkg-') as tmp:
    stage = Path(tmp) / 'stage'
    binary = stage / 'usr/local/bin/solar-monitor'
    binary.parent.mkdir(parents=True)
    subprocess.run(['go', 'build', '-ldflags=-linkmode=internal', '-o', str(binary), '.'], cwd=root,
                   env={**os.environ, 'GOOS': 'freebsd', 'GOARCH': 'amd64',
                        'GOAMD64': 'v1', 'CGO_ENABLED': '0'}, check=True)
    # A manifest override alone cannot make host-linked binaries compatible with older FreeBSD.
    headers = subprocess.check_output(['readelf', '-l', str(binary)], text=True)
    notes = subprocess.check_output(['readelf', '-n', str(binary)], text=True)
    required = re.search(r'ABI tag: (\d+)', notes)
    if 'INTERP' in headers or 'DYNAMIC' in headers or not required:
        raise SystemExit('Expected a static binary with a FreeBSD ABI note.')
    if int(required[1]) > int(abi.split(':')[1]) * 100000:
        raise SystemExit('Binary requires a newer FreeBSD ABI than the package target.')
    print('Verified static FreeBSD binary; minimum ABI tag:', required[1])
    for source, target in [('packaging/freebsd/solar_monitor', 'etc/rc.d/solar_monitor'),
                           ('README.md', 'share/doc/solar-monitor/README.md')]:
        dest = stage / 'usr/local' / target
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(root / source, dest)
    (stage / 'usr/local/etc/rc.d/solar_monitor').chmod(0o755)
    shutil.copytree(root / 'docs', stage / 'usr/local/share/doc/solar-monitor/docs')
    manifest = {
        'name': 'solar-monitor', 'version': version, 'origin': 'local/solar-monitor',
        'comment': 'Solar and battery monitoring via ECHONET Lite',
        'desc': 'Solar Monitor with an rc.d service and SQLite storage.',
        'maintainer': 'takemontakashi2@users.noreply.github.com',
        'www': 'https://github.com/takemontakashi2/solar-monitor',
        'prefix': '/usr/local', 'abi': abi, 'arch': abi,
        'files': {'/' + str(p.relative_to(stage)): {'sum': '1$' + hashlib.sha256(p.read_bytes()).hexdigest(), 'uname': 'root', 'gname': 'wheel', 'perm': '0755' if os.access(p, os.X_OK) else '0644'}
                  for p in stage.rglob('*') if p.is_file()},
        'scripts': {
            'pre-install': '''#!/bin/sh
set -e
pw groupshow solar-monitor >/dev/null 2>&1 || pw groupadd solar-monitor
pw usershow solar-monitor >/dev/null 2>&1 || pw useradd solar-monitor -g solar-monitor -d /var/db/solar-monitor -s /usr/sbin/nologin -c "Solar Monitor"
''',
            'post-install': '''#!/bin/sh
set -e
install -d -o solar-monitor -g solar-monitor -m 0750 /var/db/solar-monitor /usr/local/etc/solar-monitor
# Runtime configuration is deliberately not a package-owned file: upgrades must preserve it.
if [ ! -e /usr/local/etc/solar-monitor/config.json ]; then
    printf '{}\\n' > /usr/local/etc/solar-monitor/config.json
    chown solar-monitor:solar-monitor /usr/local/etc/solar-monitor/config.json
    chmod 0640 /usr/local/etc/solar-monitor/config.json
fi
# Preserve user settings and migrate old flags rather than silently dropping them.
if ! /usr/sbin/sysrc -n solar_monitor_option >/dev/null 2>&1; then
    old_flags=$(/usr/sbin/sysrc -n solar_monitor_flags 2>/dev/null || :)
    /usr/sbin/sysrc "solar_monitor_option=$old_flags"
fi
''',
            'pre-deinstall': '''#!/bin/sh
# Stop via the supervisor so the child can flush before package files are removed.
if [ -f /var/run/solar_monitor.pid ]; then
    /usr/local/etc/rc.d/solar_monitor onestop || exit 1
fi
'''
        },
        'messages': [{'message': 'Enable: sysrc solar_monitor_enable=YES\nStart: service solar_monitor start\nDB: /var/db/solar-monitor/solar.db\nConfig: /usr/local/etc/solar-monitor/config.json\nUpgrades do not auto-start the service. Configuration, data and service account are retained on removal.'}]
    }
    manifest_path = Path(tmp) / 'manifest.json'
    manifest_path.write_text(json.dumps(manifest))
    subprocess.run(['pkg', 'create', '-r', str(stage), '-M', str(manifest_path), '-o', str(out)], check=True)
    package = out / ('solar-monitor-' + version + '.pkg')
    target = out / ('solar-monitor-' + version + '-' + abi.replace(':', '-').lower() + '.pkg')
    package.replace(target)
    target.with_suffix('.pkg.sha256').write_text(hashlib.sha256(target.read_bytes()).hexdigest() + '  ' + target.name + '\n')
    print(target)
