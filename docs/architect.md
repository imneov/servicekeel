# 分布式服务映射与 DNS 劫持架构文档

## 1. 架构目标

本架构旨在实现一种跨节点的服务映射与代理机制，通过 Sidecar + Router（基于 FRP 改造）机制将跨网络服务本地化，并实现按服务名 DNS 劫持，从而在边缘或异构网络中提供一种稳定、灵活、可编排的服务访问方案。

---

## 2. 架构总览

每个节点包含如下组件：

* **APIServer**（本地控制面，使用 SQLite 存储）
* **Router**（修改过的 FRP，支持动态配置与状态上报）
* **Syncer**（配置同步器）
* **Pod + Sidecar**（服务消费方）

---

## 3. 核心组件设计

### 3.1 APIServer（本地控制器）

* 每个节点运行一个本地 APIServer，监听 `localhost:7443`。
* 使用 SQLite 本地存储，支持 CRUD 操作。
* 提供本节点 Router 配置（CustomResource）给本地其他组件（Router、Sidecar、Syncer）。
* 所有 Router 的配置实际来自云端，由云端控制器写入。

> 资源路径示例：
> `GET /apis/edge.example.com/v1/routers/{nodeName}`

---

### 3.2 Router（frps 改造版）

* Router 是基于 FRP 的自定义版本（等价于"分布式边缘网关"）。
* 启动时会从本地 APIServer 拉取与当前节点名称一致的 Router 配置。
* Router 内部启动 `frps` 服务，支持 `stcp`/`xtcp` 等隧道代理。
* 所有连接信息（包括 frpc 连接、端口、状态等）上报到 `Router` CR 的 `status` 字段。

```yaml
apiVersion: edge.example.com/v1
kind: Router
metadata:
  name: node-1
spec:
  frps:
    bindPort: 7000
    dashboardPort: 7500
status:
  clients:
  - name: pod-abc
    address: 127.0.66.1:3306
```

---

### 3.3 Syncer（控制平面同步器）

* 每个节点运行一个 Syncer，职责如下：

  * 从静态配置（或环境变量）读取所有节点的 APIServer 地址列表，不支持动态变更
  * 拉取所有节点的 Router CR，将其状态同步（或 patch）至本地 APIServer，用于 Sidecar 查阅

> 目的：即便 Pod 与 Router 不在同一节点，Sidecar 也能"看到"目标 Router 状态。

---

### 3.4 Sidecar（服务劫持 + 注册器）

* 每个 Pod 启动时会自动附带一个 Sidecar。
* Sidecar 执行以下流程：

#### 环境变量驱动

从环境变量中获取需要映射的服务名称和 IP 段，例如：

```env
SIDECAR_MAPPED_SERVICES=mysql,redis
SIDECAR_IP_RANGE=127.0.66.0/24
```

##### 可配置项示例

```yaml
env:
  - name: SIDECAR_MAPPED_SERVICES
    value: "mysql,redis"
  - name: SIDECAR_IP_RANGE
    value: "127.0.66.0/24"
```

#### frpc 注册器

* Sidecar 会读取本地 APIServer 中所有 Router 状态；
* 找到支持所需服务（如 `mysql`）的 Router；
* 为每个需要映射或发布的服务执行两种 frpc 动作：
  1. Dial（stcp）模式：在 Pod 本地创建 frpc 进程，将远端服务映射到 `127.0.66.x:PORT` 本地端口，使应用客户端能够访问；
  2. Bind（visitor）模式：在 Router 端以 visitor 方式注册并发布 Pod 内部服务，使其他节点的流量能够被路由至该服务；
* 将所有映射和发布的结果（端口、连接状态等）写入本地缓存，以供后续 DNS 劫持和状态监控使用。

#### DNS 劫持器

* 在 Pod 内监听 `127.0.0.2:53/udp`，劫持 `mysql.default.svc` 等服务名称；
* 返回 `127.0.66.x` 范围内的本地代理地址（由 `SIDECAR_IP_RANGE` 指定），达到劫持服务访问的目的；
* 结合 `dnsPolicy: None + dnsConfig.nameservers=[127.0.0.2]` 生效。

---

## 4. 数据与资源流动

```text
1. 云端控制器 -> 所有节点 APIServer
   写入 Router 配置（每个节点一个 CR）

2. 节点 Router -> 本地 APIServer
   拉取 Router 配置，运行 frps，状态上报到 CR.status

3. 节点 Syncer -> 本地 APIServer
   拉取其他节点 APIServer 的 Router 状态，同步至本节点

4. Pod Sidecar -> 本地 APIServer
   Watch 所有 Router 状态，根据服务需求选择 Router

5. Sidecar -> Router (frps)
   以 frpc 方式连接，建立 stcp 映射，并在本地监听

6. Sidecar DNS Server -> Pod 内服务
   拦截服务名请求，返回 frpc 本地监听地址（127.0.66.1）
```

---

## 5. 服务映射示例流程

> Pod 需访问远端 `mysql` 数据库服务

1. 用户设置环境变量 `SIDECAR_MAPPED_SERVICES=mysql` 和 `SIDECAR_IP_RANGE=127.0.66.0/24`；
2. Sidecar 查询 APIServer，发现某个 Router 提供了 `mysql`；
3. Sidecar 启动 frpc，映射连接至本地 `127.0.66.x:3306`；
4. Sidecar DNS 返回 `127.0.66.x` 给服务名称 `mysql.default.svc`；
5. 应用访问 `mysql.default.svc:3306` 实际连接本地映射端口。

---

## 6. CRD 设计（简略）

### Router CR

```yaml
apiVersion: edge.example.com/v1
kind: Router
metadata:
  name: node-1
spec:
  frps:
    bindPort: 7000
    dashboardPort: 7500
  # 可选：明确声明此 Router 节点能代理的服务列表
  # advertisedServices:
  #   - name: mysql
  #     port: 3306
  #   - name: redis
  #     port: 6379
status:
  # 报告此 Router (frps) 当前实际代理的服务端口
  # 这些通常由连接上来的 frpc (Sidecar) 动态注册确定
  activeServices:
    - name: mysql # 服务名，应与 frpc 注册时提供的元数据关联
      localPort: 7001 # frps 监听的端口 (用于 stcp)
      targetPort: 3306 # 原始目标服务端口 (供参考)
  # 报告当前连接到此 Router (frps) 的客户端 (Sidecar/frpc)
      clients:
    - id: pod-abc-mysql # 唯一客户端标识 (e.g., podName-serviceName)
      type: stcp # 连接类型
      remoteAddr: <frpc_client_ip>:<frpc_client_port> # frpc 客户端地址
      localAddr: 127.0.66.1:3306 # frpc 在 Pod 内监听的地址和端口
      serviceName: mysql # 映射的服务名
      # 可以添加更多状态，如连接时间、流量等
```

---

## 7. 安全性与可扩展性建议

* 使用 `stcp` 通信模式，启用密钥加密通信；
* 为 `frpc` 注册添加服务名与 token 认证；
* 支持 Sidecar 的热更新服务列表；
* Router 可支持负载均衡策略和断链重连；
* **评估 Syncer 同步延迟对服务可用性的影响**；

---
