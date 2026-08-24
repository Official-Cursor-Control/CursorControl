# Cursor Control — First GitHub Launcher Release

This repository is prepared for the first launcher-driven game release.

## What belongs in the normal repository
Commit these folders/files:
- `release/manifest.json`
- `release/launcher_config.example.json`
- `release/SHA256SUMS.txt`
- `launcher/` source and design reference
- `tools/`
- `docs/`
- `README.md`
- `.gitignore`

## What does NOT belong in normal Git commits
Do **not** commit `CursorControl_v462_RUNTIME.zip` to the repository history.
Upload it as the binary asset attached to the GitHub Release tagged `v462`.

## Placeholder to replace
Before the first live test, replace every instance of:
`Official-Cursor-Control`
with the GitHub account or organisation that owns the repository.

Recommended repository name:
`CursorControl`

## Production launcher URL
The launcher manifest URL will be:
`https://raw.githubusercontent.com/Official-Cursor-Control/CursorControl/main/release/manifest.json`

## v462 release asset URL
The manifest expects:
`https://github.com/Official-Cursor-Control/CursorControl/releases/download/v462/CursorControl_v462_RUNTIME.zip`

Do not rename the Runtime ZIP after uploading unless you update `release/manifest.json` too.
