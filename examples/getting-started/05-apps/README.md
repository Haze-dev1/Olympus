# 05 - Apps and server options

This example shows two things: the `apps:` key for non-agent tools, and common `server:` options.

OpenClaw is deployed as an app. The server block also sets the SSH port, how many generations to keep, and the apply confirmation timeout.

## What you need

- A fresh Ubuntu or Debian server you can reach over SSH.
- An API key for OpenClaw.
- A gateway token for OpenClaw.

## Setup

1. Copy `.env.example` to `.env`.
2. Replace each `replace-me` with your real value.

```sh
cp .env.example .env
```

## Run it

Check the stack file and recipes:

```sh
olym check -f examples/getting-started/05-apps/stack.yaml
```

Preview what Olympus will do on the server:

```sh
olym plan -f examples/getting-started/05-apps/stack.yaml
```

Apply it to the server:

```sh
olym apply -f examples/getting-started/05-apps/stack.yaml
```

Check the result:

```sh
olym status -f examples/getting-started/05-apps/stack.yaml
```

## What you should see

- `olym check` prints `Stack is valid.` and writes `stack.lock`, which pins the recipe revision and image digests. Commit it.
- `olym plan` lists `+ openclaw  new tool` and `1 to add`.
- `olym apply` connects to the server, installs what is needed, and starts OpenClaw.
- `olym status` shows `openclaw` as running and healthy.
