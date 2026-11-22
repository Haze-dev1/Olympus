# 03 - Two agents

This example runs two agents side by side on the same server: Hermes and OpenCode.

## What you need

- A fresh Ubuntu or Debian server you can reach over SSH.
- An API key for Hermes.
- An API key for OpenCode.

## Setup

1. Copy `.env.example` to `.env`.
2. Replace each `replace-me` with your real API key.

```sh
cp .env.example .env
```

## Run it

Check the stack file and recipes:

```sh
olym check -f examples/getting-started/03-two-agents/stack.yaml -recipes ./recipes
```

Preview what Olympus will do on the server:

```sh
olym plan -f examples/getting-started/03-two-agents/stack.yaml -recipes ./recipes
```

Apply it to the server:

```sh
olym apply -f examples/getting-started/03-two-agents/stack.yaml -recipes ./recipes
```

Check the result:

```sh
olym status -f examples/getting-started/03-two-agents/stack.yaml
```

## What you should see

- `olym check` prints `Stack is valid.`
- `olym plan` lists `+ hermes` and `+ opencode` as new tools and `2 to add`.
- `olym apply` connects to the server, installs what is needed, and starts both agents.
- `olym status` shows `hermes` and `opencode` as running and healthy.
