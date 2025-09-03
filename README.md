# Computing Infrastructure Migration Model

This repository provides computing infrastructure migration features.
This is a sub-system of [Cloud-Barista platform](https://cloud-barista.github.io/technology/), and intended to deploy a multi-cloud infra as a target computing infrastructure.

## Overview

As a Cloud Computing Infrastructure Migration Framework (codename: cm-damselply) is going to support:
- Target cloud computing infra migration model (Defined using the JSON format and Go Structure.)
- Migration model management test codes for target cloud computing infra

## Execution and development environment

- Operating system (OS): 
    - Ubuntu 22.04
- Languages: 
    - Go 1.23.x

## How to run CM-Damselfly

### Configure build environment

1. Install dependencies

```bash
# Ensure that your system is up to date
sudo apt update -y

# Ensure that you have installed the dependencies, 
# such as `ca-certificates`, `curl`, and `gnupg` packages.
sudo apt install make gcc git
```
2. Install Go

**_NOTE :_** Install the stable version of Go. For example, Go v1.23.0

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

### Default REST API URL and DB/log file path
- Swagger API URL<BR>
  - http://localhost:8088/damselfly/api (username: default / password: default)

- Swagger web UI URL<BR>
  - https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cm-damselfly/refs/heads/main/api/swagger.yaml

- Default DB file path (The created and updated user migration models are stored to K/V DB as a file in the following location.)
  - ./cm-damselfly/cmd/cm-damselfly/db/damselfly.db

- Default log file path
  - ./cm-damselfly/cmd/cm-damselfly/log/damselfly.log

### Versions of packages applied to the released Damselfly

| cm-damselfly | cm-model<BR>(OnpremInfraModel<BR>/SoftwareModel) | cb-tumblebug<BR>(CloudInfraModel) |
|--------|--------|--------|
| v0.2.0 | v0.0.3 | v0.9.16 |
| v0.2.1 | v0.0.3 | v0.10.0 |
| v0.2.2 | v0.0.3 | v0.10.0 |
| v0.3.0 | v0.0.3 | v0.10.3 |
| v0.3.1 | v0.0.10 | v0.11.2 |
| v0.3.2 | v0.0.10 | v0.11.2 |
| v0.3.3 | v0.0.11 | v0.11.3 |
| v0.3.4 | v0.0.12 | v0.11.3 |
| v0.3.5 | v0.0.13 | v0.11.9 |

### CM-Damselfly REST API user guide
- Discussion link : [How to use and test CM-Damselfly APIs (with test examples)](https://github.com/cloud-barista/cm-damselfly/discussions/25)
