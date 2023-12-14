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
    - Go: 1.19

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
For example, install Go v1.19 or v1.21.x

```bash
# Set Go version
GO_VERSION=1.21.4

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

Clone CM-Damselfly repository

```bash
git clone https://github.com/cloud-barista/cm-damselfly.git ${HOME}/cm-damselfly
```

#### Run Test code

Run Migration Model Management Test code

```bash
cd ${HOME}/cm-damselfly/Test

go run model-test.go
or
./model-test.sh
```
