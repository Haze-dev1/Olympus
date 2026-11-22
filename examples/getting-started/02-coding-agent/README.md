# 02 - Coding agent

This example runs Codex on your server as a coding agent.

## What you need

- A fresh Ubuntu or Debian server you can reach over SSH.
- A Codex API key.

## Setup

1. Copy `.env.example` to `.env`.
2. Replace `replace-me` with your real API key.

```sh
cp .env.example .env
```

## Run it

Check the stack file and recipes:

```sh
olym check -f examples/getting-started/02-coding-agent/stack.yaml
```

Preview what Olympus will do on the server:

```sh
olym plan -f examples/getting-started/02-coding-agent/stack.yaml
```

Apply it to the server:

```sh
olym apply -f examples/getting-started/02-coding-agent/stack.yaml
```

Check the result:

```sh
olym status -f examples/getting-started/02-coding-agent/stack.yaml
```

## What you should see

- `olym check` prints `Stack is valid.` and writes `stack.lock`, which pins the recipe revision and image digests. Commit it.
- `olym plan` lists `+ codex  new tool` and `1 to add`.
- `olym apply` connects to the server, installs what is needed, and starts Codex.
- `olym status` shows `codex` as running and healthy.
