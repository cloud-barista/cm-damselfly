# Computing Infrastructure Migration Model

This repository provides computing infrastructure migration features.
This is a sub-system of [Cloud-Barista platform](https://cloud-barista.github.io/technology/), and intended to deploy a multi-cloud infra as a target computing infrastructure.

## Overview

The Cloud Computing Infrastructure Migration Model Framework (codename: cm-damselfly) manages the **migration user models** used across the Cloud-Migrator subsystems.
It stores and serves the models through a REST API, and supports:

- **Infra migration models** (on-premise / cloud), defined by [cm-beetle/imdl](https://github.com/cloud-barista/cm-beetle/tree/main/imdl)
- **Software migration models** (source / target), defined by [cm-grasshopper/smdl](https://github.com/cloud-barista/cm-grasshopper/tree/main/smdl)
- **Model stages** : `source` → `planning` → `target`
- **Model version management** : create and validate models against any tagged version of the model definitions
- **Migration lineage** : track which models were derived from which (source → planning → target)

## Key features

### Model stages
Every migration user model belongs to one of the three stages, given by the `modelStage` query parameter.

| Stage | Description |
| :---- | :---------- |
| `source` | Model of the source (e.g. on-premise) computing environment |
| `planning` | Intermediate model being designed/recommended for migration |
| `target` | Final model of the target cloud environment |

### Model version management
- `GET /damselfly/model/version` returns all tagged versions of the migration models.
  - `onpremModelVersion` / `cloudModelVersion` : the `imdl/vX.Y.Z` tags of cm-beetle
  - `softwareModelVersion` : the `smdl/vX.Y.Z` tags of cm-grasshopper
- When creating a model, the model version can be selected by the query parameters `onpremModelVersion`, `cloudModelVersion` or `softwareModelVersion`. If omitted, the latest tag is used.
- The model body is validated against (and stored in the form of) the struct of the selected tagged version. The Go source of that version is downloaded and parsed at runtime ([pkg/modelschema](pkg/modelschema)), so a new model version can be used without rebuilding CM-Damselfly.
- On update, the stored model version is preserved and the body is validated against that version.
- Swagger UI lists an API definition per tagged version (`imdl/{version}/doc.json`, `smdl/{version}/doc.json`) in the **Select a definition** drop-down, so that the request body schema of each version can be checked.

> [!NOTE]
> The tagged versions and their source code are fetched from GitHub (`api.github.com`) and the Go module proxy (`proxy.golang.org`, or the proxies in `GOPROXY`). CM-Damselfly needs outbound internet access to use the model version features. Fetched results are cached in memory.

### Migration lineage
The models derived from one another are linked by the `lineage` field (`sourceGroupId`, `sourceModelId`, `planningModelId`, `targetModelId`).
(Refer to [Damselfly model metadata - lineage](https://github.com/cloud-barista/cm-beetle/blob/main/docs/announcements/damselfly-model-metadata-lineage.md))

- On create/update, give `parentId` (the ID of a model of the same category in the previous stage) to derive the upstream lineage fields from the parent. `lineage` can also be given explicitly.
- The lineage field of the model's own stage (e.g. `planningModelId` of a planning model) is set by the server.
- The list APIs can be filtered by `sourceGroupId`, `sourceModelId`, `planningModelId` and `targetModelId` to find all the models derived from a model.
- A model referred to by the lineage of other models cannot be deleted (409 Conflict) unless `force=true`, and its stage cannot be changed.

## REST API overview

All APIs are served under `http://localhost:8088/damselfly` with basic auth (default: `default` / `default`).

### Migration user models (unified APIs, recommended)

| Method | Path | Description |
| :----- | :--- | :---------- |
| GET | `/model/version` | Get all tagged versions of the migration models |
| GET | `/model` | Get migration user models of all categories (filters: `modelStage`, `category`, lineage) |
| GET / POST | `/infra-model` | List / create infra models (`modelType=onprem\|cloud`, `modelStage`) |
| GET / PUT / DELETE | `/infra-model/{id}` | Get / update / delete an infra model |
| GET / POST | `/software-model` | List / create software models (`modelStage`) |
| GET / PUT / DELETE | `/software-model/{id}` | Get / update / delete a software model |

### Legacy APIs
The following APIs are kept for compatibility. They distinguish source and target models by `isTargetModel` only, and do not support the `planning` stage, model version selection or lineage. Use the unified APIs above for those features.

| Path | Description |
| :--- | :---------- |
| `/onpremmodel`, `/onpremmodel/{id}` | On-premise migration user models |
| `/cloudmodel`, `/cloudmodel/{id}` | Cloud migration user models |
| `/softwaremodel/source`, `/softwaremodel/source/{id}` | Source software migration user models |
| `/softwaremodel/target`, `/softwaremodel/target/{id}` | Target software migration user models |

### System

| Method | Path | Description |
| :----- | :--- | :---------- |
| GET | `/readyz` | Check CM-Damselfly is ready |
| GET | `/httpVersion` | Check HTTP version of incoming request |

### Example : source → planning → target
```bash
# 1. Create a source (on-premise) infra model  ->  returns "id": "<SOURCE_ID>"
curl -u default:default -X POST -H "Content-Type: application/json" \
  "http://localhost:8088/damselfly/infra-model?modelType=onprem&modelStage=source" \
  -d '{ "userModelName": "my-source", "lineage": { "sourceGroupId": "sg-01" }, "onpremiseInfraModel": { ... } }'

# 2. Create a planning (cloud) infra model derived from the source model  ->  returns "id": "<PLANNING_ID>"
curl -u default:default -X POST -H "Content-Type: application/json" \
  "http://localhost:8088/damselfly/infra-model?modelType=cloud&modelStage=planning&cloudModelVersion=v0.1.18" \
  -d '{ "userModelName": "my-plan", "parentId": "<SOURCE_ID>", "cloudInfraModel": { ... } }'

# 3. Create a target (cloud) infra model derived from the planning model
curl -u default:default -X POST -H "Content-Type: application/json" \
  "http://localhost:8088/damselfly/infra-model?modelType=cloud&modelStage=target" \
  -d '{ "userModelName": "my-target", "parentId": "<PLANNING_ID>", "cloudInfraModel": { ... } }'

# 4. Get all the models derived from the source model
curl -u default:default "http://localhost:8088/damselfly/model?sourceModelId=<SOURCE_ID>"
```

## Recommended execution and development environment

- Operating system (OS):
    - Ubuntu 24.04
- Programming Language:
    - Go 1.26.2+ (for building from source code)
- Docker Container & Docker Compose:
    - Latest stable version
- API doc:
    - Swag v1.16.3

## How to run CM-Damselfly

### Configure build environment

1. Install dependencies

```bash
# Ensure that your system is up to date
sudo apt update -y

# Ensure that you have installed the dependencies,
# such as `ca-certificates`, `curl`, and `gnupg` packages.
sudo apt install make gcc git
```
2. Install Go

**_NOTE :_** Install the stable version of Go. For example, Go v1.26.2

```bash
# Set Go version
GO_VERSION=1.26.2

# Get Go archive
wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz

# Remove any previous Go installation and
# Extract the archive into /usr/local/
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz

# Append /usr/local/go/bin to .bashrc
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
echo 'export GOPATH=$HOME/go' >> ~/.bashrc

# Apply the .bashrc changes
source ~/.bashrc

# Verify the installation
echo $GOPATH
go version

```

### Download the source code

```bash
# Clone CM-Damselfly repository
git clone https://github.com/cloud-barista/cm-damselfly.git ${HOME}/cm-damselfly
```

### Build and Run with docker compose
- Open ubuntu firewall TCP 8088 port on the system to access to the API(If need)
```bash
sudo ufw allow 8088/tcp
```

- Ensure that Docker and Docker Compose are installed on your system
```bash
docker --version
docker compose version
```

- Run CM-Damselfly container and related components
```bash
cd [DAMSELFLY_ROOT]
sudo docker compose up
```

- With the `-d` option runs the container in the background (in detached mode)
```bash
cd [DAMSELFLY_ROOT]
sudo docker compose up -d
```

- Build the current Damselfly source code into a container image
```bash
cd [DAMSELFLY_ROOT]
sudo docker compose up -d --build
```

- Stop CM-Damselfly
```bash
sudo docker compose down cm-damselfly
```

- The easiest way to run the container
```bash
cd [DAMSELFLY_ROOT]
sudo make compose-up
```

### Build and Run from source code
```bash
cd [DAMSELFLY_ROOT]

# (Optional) Regenerate the Swagger API docs
make swag

# Build and run the binary (with the environment variables in conf/setup.env)
make run
```
- Run `make help` to see the other targets (`build`, `prod`, `arm`, `lint`, `stop`, `clean`, `compose`, `compose-down`, etc.).

### Configuration
The configuration is set in [conf/config.yaml](conf/config.yaml) and can be overridden by the environment variables (see [conf/setup.env](conf/setup.env) and [docker-compose.yaml](docker-compose.yaml)).

| Environment variable | Default | Description |
| :------------------- | :------ | :---------- |
| `DAMSELFLY_ROOT` | (set at runtime) | Root directory of CM-Damselfly |
| `DAMSELFLY_SELF_ENDPOINT` | `localhost:8088` | Endpoint to access the Swagger API dashboard from outside |
| `DAMSELFLY_API_ALLOW_ORIGINS` | `*` | Allowed CORS origins |
| `DAMSELFLY_API_AUTH_ENABLED` | `true` | Enable basic auth for all routes |
| `DAMSELFLY_API_USERNAME` / `DAMSELFLY_API_PASSWORD` | `default` / `default` | Basic auth credentials |
| `DAMSELFLY_LKVSTORE_PATH` | `./db/damselfly.db` | Path of the K/V DB file |
| `DAMSELFLY_LOGFILE_PATH` | `./log/damselfly.log` | Path of the log file |
| `DAMSELFLY_LOGLEVEL` | `debug` | Log level (trace, debug, info, warn, error, fatal, panic) |
| `DAMSELFLY_LOGWRITER` | `both` | Log writer (file, stdout, both) |
| `DAMSELFLY_NODE_ENV` | `development` | Execution environment (development, production) |
| `GOPROXY` | `https://proxy.golang.org` | Go module proxies used to download the tagged model definitions |

### Default REST API URL and DB/log file path
- Swagger API URL<BR>
  - http://localhost:8088/damselfly/api (username: default / password: default)
  - To see the request body schema of a specific model version, choose `imdl/{version}/doc.json` or `smdl/{version}/doc.json` in **Select a definition** at the top of Swagger UI.

- Swagger web UI URL<BR>
  - https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cm-damselfly/refs/heads/main/api/swagger.yaml

- Default DB file path (The created and updated user migration models are stored to K/V DB as a file in the following location.)
  - When running from source code : ./cm-damselfly/cmd/cm-damselfly/db/damselfly.db
  - When running with docker compose : ./cm-damselfly/container-volume/cb-damselfly-container/db/damselfly.db

- Default log file path
  - When running from source code : ./cm-damselfly/cmd/cm-damselfly/log/damselfly.log
  - When running with docker compose : ./cm-damselfly/container-volume/cb-damselfly-container/log/damselfly.log

### Versions of migration models applied to the released Damselfly
| **cm-damselfly version** | **OnpremInfraModel<BR>/CloudInfraModel** |**SoftwareModel** |
| :----: | :-----: |:-----: |
| v0.2.0 | v0.0.3 | - |
| v0.2.1 | v0.0.3 | - |
| v0.2.2 | v0.0.3 | - |
| v0.3.0 | v0.0.3 | - |
| v0.3.1 | v0.0.10 | - |
| v0.3.2 | v0.0.10 | - |
| v0.3.3 | v0.0.11 | - |
| v0.3.4 | v0.0.12 | - |
| v0.3.5 | v0.0.13 | - |
| v0.3.6 | v0.0.14 | - |
| v0.4.0 | v0.0.14 | - |
| v0.4.1 | v0.0.15 | - |
| v0.5.0 | v0.0.21 | v0.0.21 |
| v0.5.1 | v0.1.6 | v0.1.0 |
| v0.5.2 | v0.1.9 | v0.1.1 |
| v0.5.3 | v0.1.10 | v0.1.1 |
| v0.6.0 | v0.1.10 | v0.1.1 |
| v0.6.1 | v0.1.11 | v0.1.1 |
| v0.6.2 | v0.1.12 | v0.1.3 |
| v0.6.3 | v0.1.15 | v0.1.3 |
| main : v0.6.4 | v0.1.18 | v0.1.4 |

**_NOTE :_** Since the model version management features (after v0.6.3), the versions above are the ones built into CM-Damselfly (used for the default Swagger API definition). Models of any other tagged version can be created by specifying the model version (see [Model version management](#model-version-management)).

### CM-Damselfly REST API user guide
- Discussion link : [How to use and test CM-Damselfly APIs (with test examples)](https://github.com/cloud-barista/cm-damselfly/discussions/25)
