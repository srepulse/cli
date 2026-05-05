# kubectl-srepulse

Terminal client for [**srepulse**](https://srepulse.com) — the
autonomous Kubernetes SRE agent.

```bash
# Install via Krew (once published)
kubectl krew install srepulse

# Or build from source
git clone https://github.com/srepulse/cli.git && cd cli
make install

# Hit it
kubectl srepulse                                  # TUI
kubectl srepulse list                             # incidents (table)
kubectl srepulse list -j | jq '.[] | .id'         # incidents (json)
kubectl srepulse show 8f6fc09b25ac
kubectl srepulse approve <id> --reason "OOMKilled bump verified in staging"
kubectl srepulse reject  <id> --reason "non-prod hours, pause autonomy"
kubectl srepulse logs    <id> -f
```

The CLI talks to a srepulse instance running in your cluster — the
agent never lives outside the cluster, so the binary expects a URL
or `kubectl port-forward`.

## Configuration

Server URL precedence: `--server` flag > `$SREPULSE_URL` >
`~/.config/srepulse/config.json` > `http://localhost:8080`.

```bash
# Persist a default
kubectl srepulse config set-server https://srepulse.your-cluster.example

# Authenticate (currently local-password; OIDC ships alongside SSO)
kubectl srepulse login

# What's stored
kubectl srepulse config show
```

Credentials live at mode 0600 under `~/.config/srepulse/`. Override
the directory with `$SREPULSE_CONFIG_DIR` for per-cluster profiles.

## Subcommand surface

| Command | Behaviour |
| --- | --- |
| (no args) | Launch the TUI |
| `tui` | Same as no args |
| `list` | Incidents as a table; `-j` for JSON, `--status awaiting_approval` to filter |
| `show <id>` | One incident — hypotheses, remediation, summary |
| `approve <id> --reason ""` | Approve a pending remediation. `-y` to skip confirmation |
| `reject <id> --reason ""` | Reject; routes to a manual playbook |
| `logs <id>` | Print the trace; `-f` to stream live (SSE) |
| `login` | Cache a session token |
| `config show \| set-server \| logout` | Inspect / edit the local config |
| `version` | Build version |

Every approval and rejection lands in the agent's audit log alongside
dashboard / Slack approvals — same actor identity, same audit trail.

## How it relates to the agent

This repo (`srepulse/cli`) ships the operator-facing terminal client.
The agent itself lives at [srepulse/srepulse](https://github.com/srepulse/srepulse).
Both are Apache 2.0.

The CLI and TUI consume the agent's REST + SSE surface (`/api/v1/...`,
`/sse/...`). The current shape mirrors the agent's JSON in
`internal/client/types.go`; once the agent extracts a public
`pkg/incidentapi` we'll switch to importing it directly so schema
drift is impossible by construction.

## Build & release

```bash
make build           # → bin/kubectl-srepulse (host arch)
make install         # → $GOBIN/kubectl-srepulse (kubectl plugin discovery)
make test
make lint            # golangci-lint
```

Releases are tagged `vX.Y.Z` and built via Goreleaser
(`.goreleaser.yaml`) — multi-arch (linux/amd64+arm64,
darwin/amd64+arm64, windows/amd64), checksums, GitHub Release, and
the Krew manifest PR'd into `srepulse/krew-index` automatically when
`KREW_GITHUB_TOKEN` is set in CI.

## Visual conventions

The TUI follows the srepulse design system —
[`ui_kits/cli/index.html`](https://github.com/srepulse/srepulse-design-system/blob/main/project/ui_kits/cli/index.html).
Tokens are mirrored verbatim in `internal/tui/styles/styles.go`:

- **pulse teal** (`#14B8A6`) — primary accent, only where it matters
- **status colours** (`ok`, `warn`, `error`, `pending`) — functional, not decorative
- **JetBrains Mono** for everything; 13 px base in the design, adaptive in the TUI
- 3-dot terminal chrome at the top of the alt-screen
- phase strip with `✓ done`, `◆ current` (pulse), `· pending`

## Roadmap

- **v0.1** (this) — CLI subcommands + TUI scaffold + Krew distribution shape
- **v0.2** — three-pane TUI (incident list / detail / streaming thoughts), keybinds, theme support
- **v0.3** — workload timeline view, blast-radius visualisation, fingerprint catalog browser
- **v0.4** — auto-update check, multi-cluster context switch (Ctrl+x like k9s), Krew submission

## License

Apache 2.0. See [`LICENSE`](./LICENSE).
