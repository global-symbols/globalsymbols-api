# Server-Side Deployment (Manual Install)

This directory contains the server-side script that installs a CI-uploaded release into the
final runtime location and restarts the running Go API service.

The workflow (Task 2) is responsible for copying the release directory to the configured
upload paths on both servers. This script (Task 3) runs manually on a target server after
upload completes.

## Release Artifact Layout (CI Output)

CI copies each environment the same way:

`<target-upload-directory>/<RELEASE_ID>/`

Expected contents:

```text
<RELEASE_ID>/
  bin/
    go-api
  scripts/
    install_release.sh
  metadata/
    commit_sha.txt
    built_at_utc.txt
```

## Operator Usage

1. Run on the target server (either `pre-production` or `production`).
2. Ensure the runtime environment file exists *outside* the release directory:
   - `PREPRODUCTION_ENV_FILE` / `PRODUCTION_ENV_FILE` (see variables below)
3. Ensure the service is managed by systemd and can be restarted:
   - `PREPRODUCTION_SERVICE_NAME` / `PRODUCTION_SERVICE_NAME`
4. Ensure you have permission to restart the service (typically via `sudo`).

### Command

```bash
./install_release.sh --environment pre-production --release-id <RELEASE_ID>

# or
./install_release.sh --environment production --release-id <RELEASE_ID>
```

## Required Inputs (Configurable Values)

These values are required and are the server-side counterparts of the keys defined in
`documents/deployment-prerequisites.md` (Task 1).

### For `pre-production`

- `PREPRODUCTION_UPLOAD_DIR` (directory where CI copied uploads)
- `PREPRODUCTION_INSTALL_DIR` (final runtime base directory)
- `PREPRODUCTION_SERVICE_NAME` (systemd unit name to restart)
- `PREPRODUCTION_ENV_FILE` (path to existing `.env` file on the server)

### For `production`

- `PRODUCTION_UPLOAD_DIR` (directory where CI copied uploads)
- `PRODUCTION_INSTALL_DIR` (final runtime base directory)
- `PRODUCTION_SERVICE_NAME` (systemd unit name to restart)
- `PRODUCTION_ENV_FILE` (path to existing `.env` file on the server)

## What the Script Does

Given a release directory:

- Installs the release into:
  - `<FINAL_INSTALL_DIR>/releases/<RELEASE_ID>`
- Updates the symlink:
  - `<FINAL_INSTALL_DIR>/current` -> `releases/<RELEASE_ID>`
- Restarts the running service via:
  - `systemctl restart <SERVICE_NAME>`

## Known Unknowns / To Be Supplied

All real values for the environment-specific configuration live in `documents/deployment-prerequisites.md`,
under "Unknowns / To Be Supplied".

