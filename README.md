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
  - [注解说明](#注解说明)
    - [`servicekeel.io/exported-services` (由 Controller 自动添加)](#servicekeelioexported-services-由-controller-自动添加)
    - [`servicekeel.io/imported-services` (手动或由 Controller 添加)](#servicekeelioimported-services-手动或由-controller-添加)
  - [安全性与鲁棒性建议](#安全性与鲁棒性建议)
  - [安装与使用](#安装与使用)
  - [ROADMAP](#roadmap)
  - [参考文档](#参考文档)

## 架构目标

本架构旨在实现边缘环境下轻量级的服务注册与发现机制，通过 **Sidecar + Router + Pod 注解** 的方式，实现服务信息的本地化感知与访问代理，解决边缘或异构网络环境中 Kubernetes 原生服务发现机制受限的问题。

## 架构总览

每个节点（或 Pod）包含如下组件：

- **Sidecar**：监听 Pod 注解，实现本地服务注册、发现、DNS 劫持与代理功能；
- **Router**：边缘网关，负责维护节点间的网络连接和隧道；
- **Controller (TODO)**：监听 Service/Pod，自动为 Pod 添加服务导出注解。

[架构文档](docs/architect.md)

## 核心组件

### Sidecar

- 监听 Pod 注解 `servicekeel.io/exported-services` 和 `servicekeel.io/imported-services`；
- 实现本地 DNS 服务器（127.0.0.2:53），处理服务名称解析，映射到本地 IP (127.0.66.0/24);
- 为导入服务建立本地代理（支持 TCP/UDP）；
- 与本地 Router 通过 Unix Socket 通信。

### Router

- 维护节点间的网络连接和隧道；
- 接受 Sidecar 的代理注册和连接请求；
- 推荐使用 Unix Socket `/tmp/router.sock` 进行通信。

### Controller (TODO)

- 监听 Service 与 Pod 变更；
- 根据 Service selector 自动为 Pod 添加 `servicekeel.io/exported-services` 注解。

## 数据与资源流动

1. Controller (TODO) 监听 Service 与 Pod，为 Pod 添加 `servicekeel.io/exported-services` 注解；
2. 用户或 Controller 为 Pod 添加 `servicekeel.io/imported-services` 注解；
3. Sidecar 监听自身 Pod 的注解变化；
4. Sidecar 根据注解信息配置本地 DNS 服务 (127.0.0.2:53) 和代理规则；
5. 应用容器通过 Sidecar 进行 DNS 解析和服务访问；
6. Sidecar 与 Router 建立连接（Unix Socket）；
7. Router 负责节点间隧道通信。

## 服务映射示例

以访问远端 simple-server 服务为例，在客户端 Pod 中添加 `servicekeel.io/imported-services` 注解：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: client
  namespace: default
  annotations:
    servicekeel.io/imported-services: | # 声明需要访问的服务列表
      services:
        - cluster: cluster-a.local # 可选，服务所在集群
          name: simple-server    # 必填，服务名称
          namespace: default     # 可选，服务所在命名空间
          ports:
            - name: tcp
              port: 8080
              protocol: TCP # 支持 TCP/UDP
              targetport: 8080 # 容器内部目标端口
spec:
  containers:
    - name: client
      image: your-app-image # 你的应用容器
    - name: sidecar
      image: tkeelio/service-keel-sidecar:latest # Sidecar 镜像
      env:
        - name: SIDECAR_IP_RANGE
          value: "127.0.66.0/24" # Sidecar 分配的本地 IP 段
        - name: SIDECAR_DNS_ADDR
          value: "127.0.0.2:53" # Sidecar DNS 监听地址
        - name: SIDECAR_SERVER_LISTEN
          value: "unix:///tmp/router.sock" # Sidecar 与 Router 通信方式及地址
```

Sidecar 读取注解后：

1. 将 `simple-server.default.cluster-a.local` 映射到本地 IP (e.g., `127.0.66.1`)；
2. 在 `127.0.66.1:8080` 启动本地代理，通过 Router 隧道连接到远端服务；
3. 应用容器发起对 `simple-server.default` 的 DNS 请求时，Sidecar DNS 服务返回 `127.0.66.1`。
4. 应用连接 `127.0.66.1:8080`，流量通过 Sidecar 代理和 Router 隧道到达远端 simple-server。

## 注解说明

ServiceKeel 通过 Pod 注解 `servicekeel.io/exported-services` 和 `servicekeel.io/imported-services` 来配置服务的注册与发现信息。

### `servicekeel.io/exported-services` (由 Controller 自动添加)

表示该 Pod 提供的服务列表，通常根据 Service selector 自动生成。示例格式：

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

### `servicekeel.io/imported-services` (手动或由 Controller 添加)

表示该 Pod 需要访问的服务列表。Sidecar 根据此注解建立本地代理和 DNS 条目。示例格式：

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

注解值均为 YAML 序列化的字符串，包含服务名称、命名空间、集群信息以及端口（含协议 TCP/UDP）等。

## 安全性与鲁棒性建议

- Sidecar 脱离 API Server 依赖，注解信息写入后可在边缘独立工作，增强断网鲁棒性；
- Controller 可在网络可达时批量同步注解，保障断网前信息完整；
- 协议类型 (TCP/UDP) 在注解中明确，确保断网情况下流量的正确处理；
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
   将 Sidecar 作为 Pod 的一部分部署，并根据需要配置 `servicekeel.io/imported-services` 注解和 Sidecar 容器的环境变量（如 `SIDECAR_IP_RANGE`, `SIDECAR_DNS_ADDR`, `SIDECAR_SERVER_LISTEN`）。参考 [服务映射示例](#服务映射示例) 和 `examples/simple/client-pod.yaml`。

## ROADMAP

可以看 [ROADMAP](ROADMAP.md)

## 参考文档

- docs/architect.md：基于 Pod 注解的服务注册与发现架构设计
