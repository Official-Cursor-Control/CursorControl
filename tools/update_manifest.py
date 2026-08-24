#!/usr/bin/env python3
"""Update release/manifest.json hashes/size for a Cursor Control Runtime ZIP."""
from __future__ import annotations
import argparse, hashlib, json, pathlib, zipfile


def sha256_file(path: pathlib.Path) -> str:
    h = hashlib.sha256()
    with path.open('rb') as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument('runtime_zip', type=pathlib.Path)
    ap.add_argument('--version', required=True)
    ap.add_argument('--owner', required=True)
    ap.add_argument('--repo', default='Cursor-Control')
    ap.add_argument('--manifest', type=pathlib.Path, default=pathlib.Path('release/manifest.json'))
    args = ap.parse_args()

    z = args.runtime_zip.resolve()
    manifest_path = args.manifest.resolve()
    data = json.loads(manifest_path.read_text(encoding='utf-8'))
    exe_hash = ''
    with zipfile.ZipFile(z) as archive:
        exe_names = [n for n in archive.namelist() if n.replace('\\','/').endswith('/CursorControl.exe') or n == 'CursorControl.exe']
        if not exe_names:
            raise SystemExit('CursorControl.exe not found in Runtime ZIP')
        h = hashlib.sha256()
        with archive.open(exe_names[0]) as f:
            for chunk in iter(lambda: f.read(1024 * 1024), b''):
                h.update(chunk)
        exe_hash = h.hexdigest()

    data['version'] = args.version
    data['package_url'] = f'https://github.com/{args.owner}/{args.repo}/releases/download/{args.version}/{z.name}'
    data['package_sha256'] = sha256_file(z)
    data['game_exe_sha256'] = exe_hash
    data['package_bytes'] = z.stat().st_size
    manifest_path.write_text(json.dumps(data, indent=2) + '\n', encoding='utf-8')
    print(f'Updated {manifest_path}')
    print(f'Package SHA-256: {data["package_sha256"]}')
    print(f'Game EXE SHA-256: {data["game_exe_sha256"]}')
    print(f'Bytes: {data["package_bytes"]}')

if __name__ == '__main__':
    main()
