#!/usr/bin/env python3
"""Build a native FreeBSD pkg without installing it on the build host."""
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
    raise SystemExit('Build on the target FreeBSD major version and architecture.')
version = re.search(r'var version = "([^"]+)"', (root / 'main.go').read_text())[1]
abi = subprocess.check_output(['pkg', 'config', 'ABI'], text=True).strip()
out = root / 'dist'
out.mkdir(exist_ok=True)
with tempfile.TemporaryDirectory(prefix='solar-pkg-') as tmp:
    stage = Path(tmp) / 'stage'
    binary = stage / 'usr/local/bin/solar-monitor'
    binary.parent.mkdir(parents=True)
    subprocess.run(['go', 'build', '-o', str(binary), '.'], cwd=root,
                   env={**os.environ, 'CGO_ENABLED': '0'}, check=True)
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
