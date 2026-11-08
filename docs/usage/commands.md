# CLI Commands

The Olympus command-line interface is `olym`.

The root help output printed by `olym help` (or `olym` with no arguments) is:

```
usage: olym {check|build|update|plan|apply|status|rollback|gc|destroy} [-f stack.yaml] [-recipes recipes] [-o .olympus/build]
```

An unknown flag prints `invalid flags; use -f, -recipes, -o, -known-hosts and --detailed-exitcode`. That message lists only the common flags. Every flag the CLI accepts (from `internal/cli/cli.go`) is:

- `-f <file>`: path to the stack file (defaults to `stack.yaml`).
- `-recipes <directory>`: local recipe directory override (defaults to the locked catalog).
- `-o <directory>`: output build directory for `build` (defaults to `.olympus/build`).
- `-known-hosts <file>`: custom SSH `known_hosts` file.
- `-activator <file>`: explicit Linux activator binary for server commands. Release builds verify it against their embedded checksum; a development build without an embedded checksum refuses it unless `-unsafe-dev-activator` is passed.
- `-unsafe-dev-activator`: development builds only; explicitly allow uploading and running an activator that has no release checksum. Release builds always verify their embedded checksum and this flag changes nothing.
- `-out <file>`: `plan` only; save the plan so `apply <file>` can refuse to run if anything changed since.
- `-yes`: `apply` only; skip the confirmation prompt.
- `-keep <n>`: `gc` only; generations to keep (defaults to `server.keep` in the stack, then 5).
- `-json`: `status` only; print status as JSON.
- `-delete-data`: `destroy` only; also delete the project's volumes and data.
- `--detailed-exitcode`: `plan` only; exit 2 when the plan finds changes.

Single and double dashes are interchangeable (`-yes` and `--yes`).

---

## check

Validates the stack configuration file, verifies recipe definitions, and ensures all secret variables referenced in the stack exist in an adjacent `.env` file without resolving their values. If `stack.lock` does not exist, `check` generates it by resolving the catalog revision and container image index digests.

### Usage

```sh
olym check [-f stack.yaml] [-recipes recipes]
```

### Flags

- `-f`: path to stack file
- `-recipes`: path to local recipe directory override

---

## build

Evaluates the stack locally into a reproducible generation and writes Compose definitions, container files, and a content-addressed manifest to the specified output directory. Does not evaluate runtime secrets.

### Usage

```sh
olym build [-f stack.yaml] [-recipes recipes] [-o .olympus/build]
```

### Flags

- `-f`: path to stack file
- `-recipes`: path to local recipe directory override
- `-o`: destination directory for build output (defaults to `.olympus/build`)

---

## update

Refreshes pins in `stack.lock` from remote sources. Resolves the latest catalog revision from GitHub and queries container registries for updated multi-architecture index digests for all tools in the stack. Requires remote catalog (fails if `-recipes` is provided).

### Usage

```sh
olym update [-f stack.yaml]
```

### Flags

- `-f`: path to stack file

---

## plan

Evaluates the desired generation locally, connects to the target server over SSH, executes a read-only preflight check, inspects the current generation on the server, and prints a diff showing container and file changes.

When `--detailed-exitcode` is passed, `olym plan` exits with status code `2` if differences are detected, or `0` if there are no changes.

### Usage

```sh
olym plan [-f stack.yaml] [-recipes recipes] [-known-hosts known_hosts] [-activator file] [-out plan.json] [--detailed-exitcode]
```

### Flags

- `-f`: path to stack file
- `-recipes`: path to local recipe directory override
- `-known-hosts`: path to custom SSH `known_hosts` file
- `-activator`: explicit Linux activator binary
- `-out`: save the plan (mode 0600) for a later `olym apply plan.json`
- `--detailed-exitcode`: exit code 2 when changes are present

---

## apply

Deploys the evaluated generation to the target server over SSH. Runs preflight checks, uploads the generation manifest and container files, uploads the server activator, passes secret values securely over standard input, and activates the new generation. If health checks fail or the CLI does not confirm, the activator rolls back to the previous generation.

An optional saved plan (from `olym plan -out plan.json`) may be supplied. Apply refuses it if the generation, the server's current generation or the secrets changed since the plan was saved.

### Usage

```sh
olym apply [plan.json] [-f stack.yaml] [-recipes recipes] [-known-hosts known_hosts] [-activator file] [-unsafe-dev-activator] [-yes]
```

### Flags

- `-f`: path to stack file
- `-recipes`: path to local recipe directory override
- `-known-hosts`: path to custom SSH `known_hosts` file
- `-activator`: explicit Linux activator binary
- `-unsafe-dev-activator`: allows an unsigned development activator
- `-yes`: skip the confirmation prompt

---

## status

Queries the target server over SSH to report the operational state, health status, container image, and active generation for all deployed Olympus tools.

### Usage

```sh
olym status [-f stack.yaml] [-known-hosts known_hosts] [-activator file] [-unsafe-dev-activator] [-json]
```

### Flags

- `-f`: path to stack file
- `-known-hosts`: path to custom SSH `known_hosts` file
- `-activator`: explicit Linux activator binary
- `-unsafe-dev-activator`: allows an unsigned development activator
- `-json`: print status as JSON

---

## rollback

Instructs the server activator over SSH to revert the active generation to the previously deployed generation, restoring container configurations and previous secret files under health-checked supervision.

### Usage

```sh
olym rollback [-f stack.yaml] [-known-hosts known_hosts] [-activator file] [-unsafe-dev-activator]
```

### Flags

- `-f`: path to stack file
- `-known-hosts`: path to custom SSH `known_hosts` file
- `-activator`: explicit Linux activator binary
- `-unsafe-dev-activator`: allows an unsigned development activator

---

## gc

Executes garbage collection on the target server over SSH. Cleans up older inactive generations and unreferenced store files from `/var/lib/olympus/store/` while retaining active and recent generations.

### Usage

```sh
olym gc [-f stack.yaml] [-known-hosts known_hosts] [-activator file] [-unsafe-dev-activator] [-keep n]
```

### Flags

- `-f`: path to stack file
- `-known-hosts`: path to custom SSH `known_hosts` file
- `-activator`: explicit Linux activator binary
- `-unsafe-dev-activator`: allows an unsigned development activator
- `-keep`: generations to keep (defaults to `server.keep`, then 5)

---

## destroy

Tears down the deployed Olympus Compose project and stops all containers on the target server over SSH. Asks for confirmation before removing containers.

### Usage

```sh
olym destroy [-f stack.yaml] [-known-hosts known_hosts] [-activator file] [-unsafe-dev-activator] [-delete-data]
```

### Flags

- `-f`: path to stack file
- `-known-hosts`: path to custom SSH `known_hosts` file
- `-activator`: explicit Linux activator binary
- `-unsafe-dev-activator`: allows an unsigned development activator
- `-delete-data`: also delete the project's volumes and data
