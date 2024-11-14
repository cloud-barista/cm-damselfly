# Computing Infrastructure Migration Model

This repository provides computing infrastructure migration features.
This is a sub-system of [Cloud-Barista platform](https://github.com/cloud-barista/docs), and intended to deploy a multi-cloud infra as a target computing infrastructure.

## Overview

As a Cloud Computing Infrastructure Migration Framework (codename: cm-damselply) is going to support:
- Target cloud computing infra migration model (Defined using the JSON format and Go Structure.)
- Migration model management test codes for target cloud computing infra

## Execution and development environment

- Operating system (OS): 
    - Ubuntu 22.04
- Languages: 
    - Go: 1.23.0

## How to run CM-Damselfly

### Source code based installation and execution

#### Configure build environment

1. Install dependencies

```bash
# Ensure that your system is up to date
sudo apt update -y

# Ensure that you have installed the dependencies, 
# such as `ca-certificates`, `curl`, and `gnupg` packages.
sudo apt install make gcc git
```
2. Install Go

Note - **Install the stable version of Go**.
For example, install Go v1.23.0

```bash
# Set Go version
GO_VERSION=1.23.0

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

#### Download the source code

```bash
# Clone CM-Damselfly repository
git clone https://github.com/cloud-barista/cm-damselfly.git ${HOME}/cm-damselfly
```

#### Build and Run with docker compose
- Open ubuntu firewall TCP 8088 port on the system to access to the API(If need)
```bash
sudo ufw allow 8088/tcp
```

- Ensure that Docker and Docker Compose are installed on your system
```bash
docker --version
docker compose version
```

- Run CM-Damselfly and related components

```bash
cd [DAMSELFLY_ROOT]
sudo docker compose up
```
- With the `-d` option runs the container in the background

```bash
cd [DAMSELFLY_ROOT]
sudo docker compose up -d
```

- Stop CM-Damselfly
```bash
sudo docker compose down cm-damselfly
```

#### Default API URL and File Path
- Swagger API URL<BR>
  - http://localhost:8088/damselfly/api (username: default / password: default)

- Swagger web UI URL<BR>
  - https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cm-damselfly/refs/heads/main/api/swagger.yaml

- Default DB file path (The user migration model is stored to K/V DB as a file in the following location.)
  - ./cloud-barista/cm-damselfly/db/damselfly.db

- Default log file path
  - ./cloud-barista/cm-damselfly/log/damselfly.log

#### Versions of packages applied to the released Damselfly

| cm-damselfly | cm-model<BR>(OnpremInfraModel) | cb-tumblebug<BR>(CloudInfraModel) |
|--------|--------|--------|
| v0.2.0 | v0.0.3 | v0.9.16 |

#### CM-Damselfly APIs user guide
- Discussion link : [How to use and test CM-Damselfly APIs (with test examples)](https://github.com/cloud-barista/cm-damselfly/discussions/25)
