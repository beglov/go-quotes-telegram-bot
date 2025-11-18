# Deploying Go Quotes Bot with Ansible

This playbook builds the Telegram bot locally and deploys only the compiled binary to a remote VPS, where it runs as a
`systemd` service under `/opt/go-quotes-bot`.

## Requirements

- Go toolchain installed on the machine that runs Ansible (the binary is built locally)
- Ansible 2.13+
- SSH access to the target VPS with `sudo` rights

## Setup

1. Copy `inventory.example.ini` to `inventory.ini` and update hostnames/SSH settings.
2. Update `group_vars/all.yml` with the real `TELEGRAM_BOT_TOKEN` (never commit secrets).
3. Optional: override defaults (user, directories, schedule) via extra vars or group vars.

## Running the playbook

```bash
cd ansible
ansible-playbook -i inventory.ini playbook.yml
```

The playbook will:

- Build the binary locally with `go build -o build/go-quotes-bot ./cmd/bot`
- Create the `/opt/go-quotes-bot` hierarchy and service user on the VPS
- Upload the binary and minimal `.env` file
- Install and start the `go-quotes-bot.service` unit under `systemd`

To update the bot, re-run the same command—the binary will be rebuilt and replaced atomically.
