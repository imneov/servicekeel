# ServiceKeel

分布式服务映射与 DNS 劫持架构

## 目录

- [ServiceKeel](#servicekeel)
  - [目录](#目录)
  - [架构目标](#架构目标)
  - [架构总览](#架构总览)
  - [核心组件](#核心组件)
    - [Sidecar](#sidecar)
    - [Router](#router)
    - [Controller (TODO)](#controller-todo)
  - [数据与资源流动](#数据与资源流动)
  - [服务映射示例](#服务映射示例)
    - [`servicekeel.io/imported-services` 对应的配置文件 (手动或由 Controller/自动化工具注入)](#servicekeelioimported-services-对应的配置文件-手动或由-controller自动化工具注入)
  - [安全性与鲁棒性建议](#安全性与鲁棒性建议)
  - [安装与使用](#安装与使用)
  - [参考文档](#参考文档)

## 架构目标

本架构旨在实现边缘环境下轻量级的服务注册与发现机制，通过 **Sidecar + Router** 配合 **声明式服务配置（如 Pod 注解或 ConfigMap）** 的方式，实现服务信息的本地化感知与访问代理，解决边缘或异构网络环境中 Kubernetes 原生服务发现机制受限的问题。

## 架构总览

每个节点（或 Pod）包含如下组件：

- **Sidecar**：监听声明式服务配置，实现本地服务注册、发现、DNS 劫持与代理功能；
- **Router**：边缘网关，负责维护节点间的网络连接和隧道；
- **Controller (TODO)**：监听 Service/Pod，根据 Service selector 自动生成并注入服务配置信息（如写入 Pod 注解或 ConfigMap）。

[架构文档](docs/architect.md)

## 核心组件

### Sidecar

- 读取 `/etc/servicekeel` 目录下挂载的配置文件（例如 `imported-services-config.yaml` 和 `exported-services-config.yaml`），这些文件通常来源于 Pod 注解、ConfigMap 或其他声明式配置源；
- 实现本地 DNS 服务器（127.0.0.2:53），处理服务名称解析，映射到本地 IP (127.0.66.0/24);
- 为导入服务建立本地代理（支持 TCP/UDP）；
- 与本地 Router 通过 Unix Socket 通信。

### Router

- 维护节点间的网络连接和隧道；
- 接受 Sidecar 的代理注册和连接请求；
- 推荐使用 Unix Socket `/tmp/router.sock` 进行通信。

### Controller (TODO)

- 监听 Service 与 Pod 变更；
- 根据 Service selector 自动生成服务导出配置，并注入到 Pod （例如通过注解或 ConfigMap）。

## 数据与资源流动

1. Controller (TODO) 或自动化工具生成服务导出配置并注入到 Pod 声明中（例如作为注解或 ConfigMap 数据）；
2. 用户或 Controller 生成服务导入配置并注入到 Pod 声明中；
3. Kubernetes 将这些配置信息通过 Volume 机制挂载到 Sidecar 容器内的 `/etc/servicekeel` 目录为文件；
4. Sidecar 根据 `/etc/servicekeel` 目录下的配置信息配置本地 DNS 服务 (127.0.0.2:53) 和代理规则；
5. 应用容器通过 Sidecar 进行 DNS 解析和服务访问；
6. Sidecar 与 Router 建立连接（Unix Socket）；
7. Router 负责节点间隧道通信。

## 服务映射示例

以下是一个客户端 Pod 的完整示例 YAML (`examples/simple/client-pod.yaml`)，展示了如何使用注解配置导入服务，并将注解内容通过 `downwardAPI` 卷挂载到 Sidecar 容器的 `/etc/servicekeel/` 目录下，以及 Sidecar 的基本配置和必要的 volume 挂载：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: client
  namespace: default
  annotations:
    servicekeel.io/imported-services: |
      services:
        - cluster: cluster-a.local
          name: simple-server
          namespace: default
          ports:
            - name: tcp
              port: 8080
              protocol: TCP
              targetport: 8080
            - name: ntp
              port: 123
              protocol: UDP
              targetport: 123
        - cluster: cluster-a.local
          name: simple-server2
          namespace: default
          ports:
            - name: tcp
              port: 8080
              protocol: TCP
              targetport: 8080
            - name: ntp
              port: 123
              protocol: UDP
              targetport: 123
spec:
  dnsPolicy: "None"
  dnsConfig:
    nameservers:
      - "127.0.0.2"
    searches:
      - "default.svc.cluster-a.local"
      - "svc.cluster-a.local"
      - "cluster-a.local"
    options:
      - name: ndots
        value: "5"
  containers:
    - name: client
      image: workload:latest
      ports:
        - containerPort: 8080
          protocol: TCP
        - containerPort: 8081
          protocol: UDP
    - name: sidecar
      image: tkeelio/service-keel-sidecar:latest
      env:
        - name: SIDECAR_IP_RANGE
          value: "127.0.66.0/24"
        - name: SIDECAR_DNS_ADDR
          value: "127.0.0.2:53"
        - name: SIDECAR_SERVER_LISTEN
          value: "/tmp/router.sock"
      volumeMounts:
        - name: podinfo
          mountPath: /etc/servicekeel
        - name: router-socket
          mountPath: /tmp/router.sock
  volumes:
    - name: podinfo
      downwardAPI:
        items:
          - path: "exported-services-config.yaml"  # 第一个注解的文件名
            fieldRef:
              fieldPath: metadata.annotations['servicekeel.io/exported-services']
          - path: "imported-services-config.yaml"  # 第二个注解的文件名
            fieldRef:
              fieldPath: metadata.annotations['servicekeel.io/imported-services']
    - name: router-socket
      hostPath:
        path: /tmp/router.sock
        type: Socket 

## 服务配置说明

ServiceKeel 通过读取 Sidecar 容器内 `/etc/servicekeel` 目录下挂载的配置文件来获取服务的注册与发现信息。这些配置文件的数据通常来源于 Pod 相关的声明式配置（如 Pod 注解、ConfigMap 等），Sidecar 主要关注以下两个配置项对应生成的文件：

### `servicekeel.io/exported-services` 对应的配置文件 (由 Controller 或自动化工具注入)

表示该 Pod 提供的服务列表，通常根据 Service selector 自动生成。这些信息被写入到 `/etc/servicekeel/exported-services-config.yaml` 等文件中供 Sidecar 读取。示例格式：

```yaml
services:
  - cluster: cluster-a.local
    name: my-service
    namespace: default
    ports:
      - name: http
        port: 80
        protocol: TCP
        targetport: 8080
```

### `servicekeel.io/imported-services` 对应的配置文件 (手动或由 Controller/自动化工具注入)

表示该 Pod 需要访问的服务列表。这些信息通常来源于 `servicekeel.io/imported-services` 配置项，并被写入到 `/etc/servicekeel/imported-services-config.yaml` 等文件中供 Sidecar 读取，Sidecar 根据此配置建立本地代理和 DNS 条目。示例格式：

```yaml
services:
  - cluster: cluster-b.local
    name: remote-db
    namespace: prod
    ports:
      - name: mysql
        port: 3306
        protocol: TCP
        targetport: 3306
```

配置值均为 YAML 序列化的字符串，包含服务名称、命名空间、集群信息以及端口（含协议 TCP/UDP）等。

## 安全性与鲁棒性建议

- Sidecar 脱离 API Server 依赖，配置信息注入后可在边缘独立工作，增强断网鲁棒性；
- Controller 或自动化工具可在网络可达时批量同步配置，保障断网前信息完整；
- 协议类型 (TCP/UDP) 在配置中明确，确保断网情况下流量的正确处理；
- 建议为 Router 连接添加认证和加密；
- 考虑 Sidecar 热更新服务列表的机制；
- 优化 DNS 解析逻辑，支持更多场景。

## 安装与使用

1. 克隆项目：
   ```bash
   git clone https://github.com/your-repo/servicekeel.git
   cd servicekeel
   ```
2. 构建 Sidecar 二进制或镜像：
   ```bash
   make build-sidecar    # 或者使用 Dockerfile 构建镜像
   ```
3. 部署 Sidecar：
   将 Sidecar 作为 Pod 的一部分部署，并根据需要配置服务导入信息（例如通过 `servicekeel.io/imported-services` 注解或 ConfigMap）和 Sidecar 容器的环境变量（如 `SIDECAR_IP_RANGE`, `SIDECAR_DNS_ADDR`, `SIDECAR_SERVER_LISTEN`）。可以参考 [服务映射示例](#服务映射示例) 中的完整 Pod YAML 配置。

## 参考文档

- docs/architect.md：基于 Pod 注解的服务注册与发现架构设计
