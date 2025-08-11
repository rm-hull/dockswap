# dockswap

**dockswap** is a CLI tool for performing safe, health-checked rolling restarts of Docker Compose services.

It pulls the latest image, starts a new container, waits until it passes its `HEALTHCHECK`, then stops the old one — ensuring zero downtime.

## Features

-   Sequential rolling updates for Docker Compose services
-   Uses container healthchecks to ensure readiness
-   Works with multiple replicas
-   Pure Docker Engine API (no shell commands)

## Installation

Download the binary for your OS from [Releases](../../releases)
or build from source:

```bash
go install github.com/rm-hull/dockswap@latest
```

## Usage

```bash
dockswap \
  --service web \
  --project myproject \
  --image myimage:latest \
  --wait 5 \
  --timeout 120
```

| Flag        | Description                                     |
| ----------- | ----------------------------------------------- |
| `--service` | Docker Compose service name (required)          |
| `--project` | Docker Compose project name (required)          |
| `--image`   | Image to pull & deploy (required)               |
| `--wait`    | Seconds between health check polls (default: 5) |
| `--timeout` | Max seconds to wait for healthy (default: 120)  |

## Requirements

-   Docker daemon running
-   Containers must have a HEALTHCHECK configured
-   Built for Docker Compose v2 labels
