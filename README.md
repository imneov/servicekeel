# ServiceKeel

分布式服务映射与 DNS 劫持架构

## 目录

- [ServiceKeel](#servicekeel)
  - [目录](#目录)
  - [架构目标](#架构目标)
  - [架构总览](#架构总览)
  - [核心组件](#核心组件)
    - [APIServer](#apiserver)
    - [Router](#router)
    - [Syncer](#syncer)
    - [Sidecar](#sidecar)
  - [数据与资源流动](#数据与资源流动)
  - [服务映射示例](#服务映射示例)
  - [CRD 设计](#crd-设计)
  - [安全性与可扩展性建议](#安全性与可扩展性建议)
  - [安装与使用](#安装与使用)
  - [ROADMAP](#roadmap)
  - [参考文档](#参考文档)

## 架构目标

本架构旨在实现跨节点的服务映射与代理机制，通过 Sidecar + Router 机制将跨网络服务本地化，并按服务名进行 DNS 劫持，为边缘或异构网络环境提供稳定、灵活、可编排的服务访问方案。

## 架构总览

每个节点包含如下组件：

- **APIServer**：本地控制平面，使用 SQLite 存储；
- **Router**：基于 FRP 的自定义分布式边缘网关；
- **Syncer**：配置同步器，将其他节点状态拉取至本地；
- **Pod + Sidecar**：服务消费方，负责映射与 DNS 劫持。

[架构文档](docs/architect.md)

## 核心组件

### APIServer

- 监听 `localhost:7443`，提供 CRUD 接口；
- 存储 Router 配置（来自云端控制器）；
- 供本节点 Router、Sidecar、Syncer 读取。

### Router

- 基于 FRP（frps 改造版）；
- 启动时拉取本地 APIServer 上的 Router CR 配置；
- 内部启动 frps 服务，支持 stcp/xtcp 等隧道代理；
- 将所有连接信息（端口、客户端、状态）上报到 CR.status。

### Syncer

- 从配置中读取所有节点 APIServer 地址；
- 拉取其他节点 Router CR，将状态同步（patch）到本地 APIServer；
- 确保 Sidecar 可见所有目标 Router 状态。

### Sidecar

- 根据环境变量 `SIDECAR_MAPPED_SERVICES`、`SIDECAR_IP_RANGE` 和 `SIDECAR_DNS_ADDR` 读取映射与 DNS 劫持配置；
- 从本地 APIServer 读取 Router 状态，选择可用 Router；
- 启动 frpc：
  - **Dial 模式（stcp）**：将远端服务映射到本地 `127.0.66.x:PORT`；
  - **Bind 模式（visitor）**：在 Router 端发布 Pod 内部服务；
- 监听 `127.0.0.2:53/udp`，劫持 DNS 请求并返回本地代理地址。

## 数据与资源流动

1. 云端控制器 → 各节点 APIServer：写入 Router CR 配置
2. Router → 本地 APIServer：拉取 CR 并上报 frps 状态
3. Syncer → 本地 APIServer：拉取其他节点状态并同步
4. Sidecar → 本地 APIServer：读取所有 Router CR 状态
5. Sidecar ↔ Router (frpc)：建立隧道（stcp）并映射服务
6. Sidecar DNS → Pod：截获服务名解析，返回本地映射 IP

## 服务映射示例

以访问远端 MySQL 服务为例：

1. 设置环境变量：
   ```shell
   export SIDECAR_MAPPED_SERVICES=mysql
   export SIDECAR_IP_RANGE=127.0.66.0/24
   export SIDECAR_DNS_ADDR=127.0.0.2:53  # 可选，自定义 DNS 劫持监听地址，默认为 127.0.0.2:53
   ```
2. Sidecar 查询 APIServer，发现远端 Router 提供 MySQL；
3. Sidecar 启动 frpc，将远端 MySQL 映射至本地 `127.0.66.x:3306`；
4. DNS 劫持返回 `127.0.66.x` 给 `mysql.default.svc`；
5. 应用通过本地地址访问 MySQL 数据库。

## CRD 设计

Router CR 定义示例：
```yaml
apiVersion: edge.example.com/v1
kind: Router
metadata:
  name: node-1
spec:
  frps:
    bindPort: 7000
    dashboardPort: 7500
  # 可选：advertisedServices 列表
status:
  activeServices:
    - name: mysql
      localPort: 7001
      targetPort: 3306
      clients:
        - id: pod-abc-mysql
          type: stcp
          localAddr: 127.0.66.1:3306
          serviceName: mysql
```

## 安全性与可扩展性建议

- 建议使用 stcp 模式并启用密钥加密；
- 为 frpc 注册添加 token 认证；
- 支持 Sidecar 热更新服务列表；
- Router 加入负载均衡与断链重连机制；
- 评估 Syncer 同步延迟对可用性的影响。

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
3. 运行 Sidecar：
   ```bash
   ./servicekeel-sidecar \
     --mapped-services=mysql \
     --ip-range=127.0.66.0/24 \
     --dns-addr=127.0.0.2:53
   ```
4. 部署 Sidecar 作为 Kubernetes 容器时，也可在 Pod 规格里设置上述环境变量。

## ROADMAP

可以看 [ROADMAP](ROADMAP.md)

## 参考文档

- docs/architect.md：架构设计与组件说明
