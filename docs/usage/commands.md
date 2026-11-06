# CLI Commands

The Olympus command-line interface is `olym`.

The root help output printed by `olym help` (or `olym` with no arguments) is:

```
usage: olym {check|build|update|plan|apply|status|rollback|gc|destroy} [-f stack.yaml] [-recipes recipes] [-o .olympus/build]
```

When invoking any command with `-h`, the CLI prints the supported flags:

```
invalid flags; use -f, -recipes, -o, -known-hosts and --detailed-exitcode
```

The flags printed by the CLI are:
- `-f <file>`: path to the stack file (defaults to `stack.yaml`).
- `-recipes <directory>`: local recipe directory override (defaults to remote catalog).
- `-o <directory>`: output build directory (defaults to `.olympus/build`).
- `-known-hosts <file>`: custom SSH `known_hosts` file.
- `--detailed-exitcode`: exit code 2 when `plan` finds changes.

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
olym plan [-f stack.yaml] [-recipes recipes] [-known-hosts known_hosts] [--detailed-exitcode]
```

### Flags

- `-f`: path to stack file
- `-recipes`: path to local recipe directory override
- `-known-hosts`: path to custom SSH `known_hosts` file
- `--detailed-exitcode`: exit code 2 when changes are present

---

## apply

Deploys the evaluated generation to the target server over SSH. Runs preflight checks, uploads the generation manifest and container files, uploads the server activator, passes secret values securely over standard input, and activates the new generation. If health checks fail or the CLI does not confirm, the activator rolls back to the previous generation.

An optional saved plan file positional argument may be supplied.

### Usage

```sh
olym apply [plan.json] [-f stack.yaml] [-recipes recipes] [-known-hosts known_hosts]
```

### Flags

- `-f`: path to stack file
- `-recipes`: path to local recipe directory override
- `-known-hosts`: path to custom SSH `known_hosts` file

---

## status

Queries the target server over SSH to report the operational state, health status, container image, and active generation for all deployed Olympus tools.

### Usage

```sh
olym status [-f stack.yaml] [-known-hosts known_hosts]
```

### Flags

- `-f`: path to stack file
- `-known-hosts`: path to custom SSH `known_hosts` file

---

## rollback

Instructs the server activator over SSH to revert the active generation to the previously deployed generation, restoring container configurations and previous secret files under health-checked supervision.

### Usage

```sh
olym rollback [-f stack.yaml] [-known-hosts known_hosts]
```

### Flags

- `-f`: path to stack file
- `-known-hosts`: path to custom SSH `known_hosts` file

---

## gc

Executes garbage collection on the target server over SSH. Cleans up older inactive generations and unreferenced store files from `/var/lib/olympus/store/` while retaining active and recent generations.

### Usage

```sh
olym gc [-f stack.yaml] [-known-hosts known_hosts]
```

### Flags

- `-f`: path to stack file
- `-known-hosts`: path to custom SSH `known_hosts` file

---

## destroy

Tears down the deployed Olympus Compose project and stops all containers on the target server over SSH. Asks for confirmation before removing containers.

### Usage

```sh
olym destroy [-f stack.yaml] [-known-hosts known_hosts]
```

### Flags

- `-f`: path to stack file
- `-known-hosts`: path to custom SSH `known_hosts` file
