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
    - Go: 1.23

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


#### Build and Run

Open Ubuntu Firewall TCP 8088 port
```
sudo ufw allow 8088/tcp

make run
```

Swagger API docs 생성
./cloud-barista/cm-damselfly]# 에서 (import한 structure를 위해 아래와 같이 실행)
```
swag init --parseDependency --parseInternal
```

Build 및 API server 구동
```
make run
```

Swagger API URL : http://localhost:8088/damselfly/api (username: default / password: default)


사용자 모델은 아래의 위치에 파일 DB로 저장됨.
```
./cloud-barista/cm-damselfly/.damselfly/lkvstore.db
```

(참고) 테스트용 Source 사용자 모델 생성 request body
```
# 아래의 첫 괄호{}는 swagger request body form에 있는거 활용해야...
{ 

 "name": "MyModel-241007-1",
 "description": "MyModel-241007-1",
 "version": "SourceModel-v0.01",
 "network": {
    "cidr": "192.168.1.0/24",
    "gateway": "192.168.1.1",
    "dns": "8.8.8.8"
  },
  "servers": [
    {
      "hostname": "server01",
      "cpu": {
        "cores": 4,
        "model": "Intel Xeon"
      },
      "memory": {
        "totalMB": 16384,
        "type": "DDR4"
      },
      "rootDisk": {
        "sizeGB": 500,
        "mountPoint": "/"
      },
      "dataDisks": [
        {
          "sizeGB": 1000,
          "mountPoint": "/data1"
        }
      ],
      "interfaces": [
        {
          "name": "eth0",
          "ipAddress": "192.168.1.100",
          "macAddress": "00:0a:95:9d:68:16"
        }
      ],
      "routingTable": [
        {
          "destination": "0.0.0.0/0",
          "gateway": "192.168.1.1"
        }
      ],
      "os": {
        "name": "Ubuntu",
        "version": "20.04"
      }
    }
  ]

}
```