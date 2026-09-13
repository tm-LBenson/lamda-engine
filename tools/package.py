"""Build reproducible source-release assets from a clean, versioned checkout."""
import argparse
import hashlib
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[1]


def git(*args):
    return subprocess.check_output(['git', *args], cwd=ROOT)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, default=ROOT / 'dist')
    args = parser.parse_args()
    if git('status', '--porcelain').strip():
        parser.error('Commit all source changes before packaging a release.')
    source = git('show', 'HEAD:cmd/lamda-engine/main.go').decode()
    version = re.search(r'var version = "(\d+\.\d+\.\d+)"', source).group(1)
    installer = git('show', 'HEAD:windows/install.ps1')
    pinned = re.search(rb"\[string\]\$Ref = '(v[^']+)'", installer).group(1).decode()
    if pinned != 'v' + version:
        parser.error('Installer ref must match the engine version before publishing.')
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    archive = output / f'lamda-engine-{version}-source.zip'
    subprocess.run(['git', 'archive', '--format=zip', f'--output={archive}', 'HEAD'], cwd=ROOT, check=True)
    script = output / 'install.ps1'
    script.write_bytes(installer)
    checksum = output / 'SHA256SUMS.txt'
    checksum.write_text(''.join(f'{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n' for p in (archive, script)))
    print(f'Prepared v{version} from {git("rev-parse", "--short", "HEAD").decode().strip()}')
    for path in (archive, script, checksum):
        print(path)


if __name__ == '__main__':
    main()
