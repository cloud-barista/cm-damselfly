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

#### Build and Run
```bash

# Open ubuntu firewall TCP 8088 port to access to the API(If need)
sudo ufw allow 8088/tcp

pwd
/home/(user)/go/src/github.com/cloud-barista/cm-damselfly

# Build and Create Swagger API docs
make
# included : swag init --parseDependency --parseInternal

# Run Damselfly and API server
make run
```
- Swagger API URL<BR>
  - http://localhost:8088/damselfly/api (username: default / password: default)

- Swagger web UI URL<BR>
  - https://cloud-barista.github.io/api/?url=https://raw.githubusercontent.com/cloud-barista/cm-damselfly/refs/heads/main/api/swagger.yaml

- Default DB to store path (The user migration model is stored to K/V DB as a file in the following location.)
  - ./cloud-barista/cm-damselfly/.damselfly/lkvstore.db

- Default log file path
  - ./cloud-barista/cm-damselfly/cmd/cm-damselfly/log/damselfly.log  
