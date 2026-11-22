# Feature: Update, Rollback, and Garbage Collection

This walkthrough demonstrates the full Olympus deployment lifecycle:
- Updating pinned versions with `olym update`.
- Previewing changes with `olym plan`.
- Deploying new generations with `olym apply`.
- Monitoring operational state with `olym status`.
- Understanding the automatic rollback safety mechanism.
- Manually reverting changes with `olym rollback`.
- Pruning old generations with `olym gc`.

## The Olympus Generation Model

Olympus deploys infrastructure as immutable, content-addressed **generations** (stored under `/var/lib/olympus/store/`). `/var/lib/olympus/current` points to the active generation.

Because older generations stay on the server, rolling back does not rebuild anything.

## Step-by-Step Walkthrough

### 1. Initial Validation and Check

Prepare your environment:

```sh
cp .env.example .env
```

Validate the stack offline:

```sh
olym check -f stack.yaml
```

`check` validates your YAML syntax, verifies recipes, and creates `stack.lock` if it does not already exist.

### 2. Previewing Changes (`olym plan`)

Preview what will change on the server:

```sh
olym plan -f stack.yaml
```

`plan` connects over SSH, runs read-only server preflights (checking OS, architecture, disk space, and port availability), compares the server's current generation with the desired generation, and displays a diff. Nothing changes on the server.

You can also save the plan to ensure exact execution:

```sh
olym plan -f stack.yaml -out plan.json
```

### 3. Deploying the Stack (`olym apply`)

Deploy the desired generation to your server:

```sh
olym apply -f stack.yaml
```

If you saved a plan:

```sh
olym apply plan.json
```

Olympus uploads new store files, writes secrets securely over standard input (mode 0600), starts containers with Docker Compose, verifies recipe health checks, and confirms the deployment.

### 4. Checking Operational Status (`olym status`)

Check the health and generation of your deployed agents:

```sh
olym status -f stack.yaml
```

Or request structured JSON output:

```sh
olym status -f stack.yaml -json
```

The output shows the active generation ID, container states, and health check results.

### 5. Refreshing Upstream Pins (`olym update`)

When new recipe updates or image patches are released upstream:

```sh
olym update -f stack.yaml
```

`update` contacts GitHub and container registries to resolve the latest catalog revision and multi-architecture image digests. It writes the updated hashes directly to `stack.lock`. Your `stack.yaml` remains untouched.

After running `update`, run `plan` to inspect the changes:

```sh
olym plan -f stack.yaml
```

Only tools whose container image digest or configuration hash changed will be restarted on the next `apply`.

### 6. Automatic Rollback if Confirmation Fails

Olympus incorporates an automatic safety rollback mechanism (ADR 0017):

1. During `apply`, the activator on the server starts the new generation and runs each recipe's health check.
2. If a health check fails, the activator re-activates the previous generation right away and `apply` exits with an error. `current` never moves.
3. If the checks pass, the activator waits for confirmation for `server.confirm_timeout` (default `60s`). The CLI opens a **new** SSH connection and runs `olym confirm`, which proves SSH still works after the change.
4. If no confirmation arrives in time (the network dropped, or the CLI was killed), the activator rolls back by itself: it restarts the previous generation's containers and restores its secret files.
5. Only after confirmation does the activator point `/var/lib/olympus/current` at the new generation.

### 7. Manual Rollback (`olym rollback`)

If an application bug is discovered after a successful deploy:

```sh
olym rollback -f stack.yaml
```

`rollback` connects over SSH and instructs the activator to revert immediately to the previous generation, restoring previous container configurations and secrets.

### 8. Cleaning Inactive Generations (`olym gc`)

Every deployment creates a new generation directory. To prune old generations that are no longer needed:

```sh
olym gc -f stack.yaml
```

You can specify how many generations to keep:

```sh
olym gc -f stack.yaml -keep 3
```

`gc` removes older inactive generations and unreferenced files from `/var/lib/olympus/store/`, freeing disk space while protecting the active generation and recent rollback targets.
