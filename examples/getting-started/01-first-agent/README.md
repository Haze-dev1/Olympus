# 01 - First agent

This is the smallest working Olympus stack. It runs one Hermes agent on your server.

## What you need

- A fresh Ubuntu or Debian server you can reach over SSH.
- An API key for the model Hermes should use.

## Setup

1. Copy `.env.example` to `.env`.
2. Replace `replace-me` with your real API key.

```sh
cp .env.example .env
```

## Run it

Check the stack file and recipes:

```sh
olym check -f examples/getting-started/01-first-agent/stack.yaml -recipes ./recipes
```

Preview what Olympus will do on the server:

```sh
olym plan -f examples/getting-started/01-first-agent/stack.yaml -recipes ./recipes
```

Apply it to the server:

```sh
olym apply -f examples/getting-started/01-first-agent/stack.yaml -recipes ./recipes
```

Check the result:

```sh
olym status -f examples/getting-started/01-first-agent/stack.yaml
```

## What you should see

- `olym check` prints `Stack is valid.`
- `olym plan` lists `+ hermes  new tool` and `1 to add`.
- `olym apply` connects to the server, installs what is needed, and starts Hermes.
- `olym status` shows `hermes` as running and healthy.
