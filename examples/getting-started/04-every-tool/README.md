# 04 - Every tool

This example shows the full recipe catalog. It has one entry for every recipe in `recipes/`.

This stack is for checking and learning the layout. Running all of these tools on one server needs a lot of RAM and disk.

## What you need

- A fresh Ubuntu or Debian server you can reach over SSH, if you want to apply it.
- No secrets are required for `olym check` or `olym build` of this example.

## Run it

Check the stack file and recipes:

```sh
olym check -f examples/getting-started/04-every-tool/stack.yaml
```

Preview what Olympus will do on the server:

```sh
olym plan -f examples/getting-started/04-every-tool/stack.yaml
```

Build the generation locally without touching a server:

```sh
olym build -f examples/getting-started/04-every-tool/stack.yaml -o /tmp/olympus-every-tool
```

## What you should see

- `olym check` prints `Stack is valid.` and writes `stack.lock`, which pins the recipe revision and image digests. Commit it.
- `olym plan` shows one entry for each recipe.
- `olym build` writes a generation with the rendered Compose file and configs.
