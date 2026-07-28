# Server-Side Deployment (Manual Install)

This directory contains the server-side script that installs a CI-uploaded release into the
final runtime location and restarts the running Go API service.

The workflow (Task 2) is responsible for copying the release directory to the configured
upload paths on both servers. This script (Task 3) runs manually on a target server after
upload completes.

## Release Artifact Layout (CI Output)

CI builds **two** Linux binaries and uploads the correct one per environment:

| Environment | Host arch | GOARCH uploaded |
|-------------|-----------|-----------------|
| pre-production | arm64 (t4g) | `arm64` |
| production | amd64 (t3) | `amd64` |

Each target still receives the same path layout:

`<target-upload-directory>/<RELEASE_ID>/`

Expected contents:

```text
<RELEASE_ID>/
  bin/
    go-api                 # arch-specific binary for that host
  scripts/
    install_release.sh
  metadata/
    commit_sha.txt
    built_at_utc.txt
    goarch.txt             # amd64 or arm64
```

## Operator Usage

1. Run on the target server (either `pre-production` or `production`).
2. Ensure the runtime environment file exists *outside* the release directory:
   - default path: `/var/www/globalsymbols-api/.env`
3. Ensure the service is managed by systemd and can be restarted:
   - default service: `globalsymbols-api.service`
4. Ensure you have permission to restart the service (typically via `sudo`).

### Command

```bash
./install_release.sh --environment pre-production --release-id <RELEASE_ID>

# or
./install_release.sh --environment production --release-id <RELEASE_ID>
```

## Configurable Values

The install script now has built-in defaults for the stable deployment values below. You
only need to set these environment variables if a server differs from the defaults.

### Shared override

- `UPLOAD_RELEASES_TO_KEEP` default: `5`
  - After a successful install and restart, the script removes older directories from
    `/var/www/globalsymbols-api/uploads` and keeps only the newest staged uploads.
  - Set to `0` if you want the upload staging directory emptied after each successful deploy.

### For `pre-production`

- `PREPRODUCTION_UPLOAD_DIR` default: `/var/www/globalsymbols-api/uploads`
- `PREPRODUCTION_INSTALL_DIR` default: `/var/www/globalsymbols-api`
- `PREPRODUCTION_SERVICE_NAME` default: `globalsymbols-api.service`
- `PREPRODUCTION_ENV_FILE` default: `/var/www/globalsymbols-api/.env`

### For `production`

- `PRODUCTION_UPLOAD_DIR` default: `/var/www/globalsymbols-api/uploads`
- `PRODUCTION_INSTALL_DIR` default: `/var/www/globalsymbols-api`
- `PRODUCTION_SERVICE_NAME` default: `globalsymbols-api.service`
- `PRODUCTION_ENV_FILE` default: `/var/www/globalsymbols-api/.env`

## What the Script Does

Given a release directory:

- Installs the release into:
  - `<FINAL_INSTALL_DIR>/releases/<RELEASE_ID>`
- Updates the symlink:
  - `<FINAL_INSTALL_DIR>/current` -> `releases/<RELEASE_ID>`
- Restarts the running service via:
  - `systemctl restart <SERVICE_NAME>`
- Cleans up old upload directories after a successful restart, keeping only the newest
  staged uploads according to `UPLOAD_RELEASES_TO_KEEP`

## Known Unknowns / To Be Supplied

All real values for the environment-specific configuration live in `documents/deployment-prerequisites.md`,
under "Unknowns / To Be Supplied".

