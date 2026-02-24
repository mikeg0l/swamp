# swamp

Interactive AWS SSM connection helper powered by AWS CLI (SSO) and `fzf`.

This tool discovers accessible AWS accounts, roles, regions, and EC2 instances, then starts an SSM session to the instance you choose.

## Features

- Uses AWS SSO profile as bootstrap (`aws sso login` supported)
- Scans accessible accounts and viable roles
- Supports both:
  - interactive narrowing by default (account -> role -> region -> instance)
  - fast filtering (`--account`, `--role`, `--regions`)
- Supports concurrent discovery (`--workers`)
- Starts shell session with `aws ssm start-session`
- Redacts SSO access token in error output

## Requirements

- Go 1.21+ (or any modern Go with modules support)
- AWS CLI v2 configured for SSO
- `fzf` installed and available in `PATH`
- AWS Session Manager Plugin installed (required by `aws ssm start-session`)

## Install

### Option 1: Run directly

```bash
go run ./cmd/swamp -p YOUR_SSO_PROFILE
```

### Option 2: Build a binary

```bash
go build -o swamp ./cmd/swamp
./swamp -p YOUR_SSO_PROFILE
```

## Add to PATH

After building, place `swamp` in a directory that is on your `PATH`.

```bash
go build -o swamp ./cmd/swamp
sudo ln -sf "$(pwd)/swamp" /usr/local/bin/swamp
swamp --help
```

## Usage

```bash
swamp -p YOUR_SSO_PROFILE [flags]
```

### Main Flags

- `-p, --profile string` AWS SSO profile name (required)
- `-w, --workers int` Concurrent workers for discovery (default: `12`)
- `-i, --interactive-scope` Pick account, role, and region with `fzf` before instance list (default: `true`; disable with `--interactive-scope=false`)
- `-a, --account string` Account ID exact match, or account-name substring
- `-r, --role string` Exact role name filter
- `-R, --regions string` Comma-separated regions (e.g. `us-east-1,eu-west-1`)
- `-A, --all-regions` Include all regions (including disabled ones)
- `-s, --include-stopped` Include non-running instances in EC2 selection
- `--cache` Enable/disable local discovery cache (default: `true`)
- `--cache-dir string` Cache directory (default: OS user cache dir + `/swamp`)
- `--cache-mode string` Cache behavior: `balanced`, `fresh`, or `speed` (default: `balanced`)
- `--cache-clear` Remove cache contents before discovery
- `--cache-ttl-accounts duration` TTL for accounts cache (default: `6h`)
- `--cache-ttl-roles duration` TTL for roles cache (default: `6h`)
- `--cache-ttl-regions duration` TTL for regions cache (default: `24h`)
- `--cache-ttl-instances duration` TTL for instances cache (default: `60s`)

## Typical Workflows

### 1) Fully interactive scope narrowing

```bash
swamp -p appfire-sso
```

Flow:
1. select account
2. select role
3. select region
4. select instance
5. SSM session starts

### 2) Fast filtered run (account + role)

```bash
swamp -p appfire-sso -a 123456789012 -r AdministratorAccess
```

### 3) Restrict region set for speed

```bash
swamp -p appfire-sso -R us-east-1,eu-west-1 -w 24
```

## Performance Notes

- Lower scope first for speed: `--account`, `--role`, `--regions`
- Start with `--workers 12`; raise to `16-32` if needed
- Very high worker counts can trigger AWS throttling and reduce real performance
- Leave cache on for repeated usage; this avoids repeating most SSO/account/role/region discovery calls

## Caching

Swamp caches discovery data on disk and reuses it across runs.

- `balanced` (default): use fresh cache immediately; if stale cache exists, use it and refresh in background
- `fresh`: bypass cache reads and always refresh from AWS (still writes cache)
- `speed`: aggressively use available cache and refresh stale entries in background

Default TTLs:
- accounts: `6h`
- roles: `6h`
- regions: `24h`
- instances: `60s`

Examples:

```bash
# default balanced mode
swamp -p appfire-sso

# force fresh discovery this run
swamp -p appfire-sso --cache-mode fresh

# speed-first with longer instance TTL
swamp -p appfire-sso --cache-mode speed --cache-ttl-instances 5m

# clear cache before running
swamp -p appfire-sso --cache-clear
```

## Troubleshooting

### "Unable to locate credentials"

- Ensure profile is SSO-configured in `~/.aws/config`
- Run: `aws sso login --profile YOUR_SSO_PROFILE`
- Verify profile works:  
  `aws --profile YOUR_SSO_PROFILE sts get-caller-identity`

### "You must specify a region"

- Ensure your SSO session has an `sso_region`
- Or pass explicit regions with `--regions`

### No instances found

- Try `--include-stopped`
- Verify selected role has EC2 + SSM permissions
- Confirm instances are SSM-managed and online

### `start-session` fails locally

- Install or repair Session Manager Plugin
- Verify with:  
  `aws ssm start-session help`

## Security Notes

- The tool reads SSO access tokens from `~/.aws/sso/cache`
- Access tokens are not printed in command error logs (redacted)
- Temporary AWS config files are cleaned up on exit
