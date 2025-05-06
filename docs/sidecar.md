# Sidecar



## 业务流程

### 服务代理

下面结合 `cmd/sidecar`（Sidecar 入口）和 `internal/controller`（FRPServer 控制器）的实现，梳理整个“服务代理”（Sidecar + Controller）的业务流程：

1. Sidecar 启动与参数解析  
   1.1 通过命令行参数或环境变量读取：  
     - `SIDECAR_MAPPED_SERVICES`：要代理的服务列表（逗号分隔）  
     - `SIDECAR_IP_RANGE`：分配给各服务的本地 IP 段（CIDR）  
     - `SIDECAR_DNS_ADDR`：DNS 劫持服务器监听地址（默认 `127.0.0.2:53`）  
   1.2 校验参数（服务数量 ≤100，IP 段非空），并打印日志。

2. 启动 DNS 劫持服务器  
   - 创建 `dns.Server`，内部解析并保存 CIDR 网段。  
   - 绑定 UDP 端口，使用 miekg/dns 库后台监听。  
   - 提供：  
     - `AddMapping(name)`：给服务名分配一个未使用的 IP 并记录映射  
     - `RemoveMapping(name)`：删除映射并释放 IP  
     - DNS 请求处理：拦截所有 A 记录查询，查映射表并返回对应 IP，否则返回 NXDOMAIN。

3. 初始化 FRPServerController  
   - 调用 `controller.NewFRPSServer(mappedServices, dnsServer)`：  
     - 构建 Kubernetes client（controller-runtime）  
     - 保存 `mappedServices` 列表和 DNS 服务实例  
     - 设置周期（默认 10s）和内部状态容器。

4. 周期性 Reconcile 循环  
   - 以固定间隔（10s）异步调用 `Reconcile`：  
     4.1 从 Kubernetes 中列出所有 `FRPServer` 自定义资源（CR）。  
     4.2 遍历每条 CR 的 `status.activeConnections`：  
       - 取出 `ProxyName`（格式含服务名、端口、协议等）和 `SecretKey`  
       - 解析出服务名、协议、端口，若不在 `mappedServices` 中则跳过  
       - 构造内部 `EndpointInfo`：记录 FRP Server 地址/端口、SecretKey、服务详情  
     4.3 将新抓取到的 `endpoints` 与上轮保存的列表对比，计算：  
       - `newEndpoints`：新增未管理的服务隧道  
       - `deleteEndpoints`：失效需关闭的服务隧道  

5. 新增/删除 Endpoint 处理  
   - 对于每个 `newEndpoints`：  
     - 调用 `dnsServer.AddMapping(serviceName)` 分配本地 IP，并填入 `EndpointInfo.MappedIP`  
     - 通过 `controller.NewFRPClient(...)` 构造 FRP 客户端（frpc）实例  
     - `FRPClient.Start()` 启动 frpc 子进程，建立到 FRP Server 的隧道  
   - 对于每个 `deleteEndpoints`：  
     - 停掉对应的 `FRPClient.Stop()`，释放 frpc 进程  
     - 调用 `dnsServer.RemoveMapping(serviceName)` 回收 IP  

6. DNS + FRP 联动代理  
   - 已启动的 frpc 隧道将本地分配的 IP（`MappedIP`）映射到远端目标服务  
   - 应用或进程在 Pod/Host 内通过 DNS 解析服务名时，因 DNS 劫持被指向上述本地 IP  
   - 访问本地 IP 时，流量由 frpc 隧道转发到远端服务，实现透明代理

7. 主流程阻塞  
   - Sidecar 在启动 Controller 后进入 `select {}` 阻塞，持续提供 DNS + 隧道服务

—— 以上即 Sidecar + Controller 两部分协同完成的服务代理全流程：  
• Sidecar 负责参数解析、DNS 劫持  
• Controller 负责监控 FRPServer CR、管理隧道（frpc）& DNS 映射  
• 最终在本地 IP 与远端服务之间建立动态的透明代理链路。






## 部署架构

已经有一个 router 运行在节点 172.31.19.105 上，监听 17000 端口

有一个 Client 连接到 router 并且注册了 mysql.abc 这个 name，这些数据都可以在 CR 中看到

现在需要给 sidecar 增加 docs/architect.md 中的内容，监听这个 CR，然后创建对应的 DNS 条目




CR 
```
root@i-0jpjme2g:~/workspace/frp# kubectl get frpservers.tunnel.kubeinfra.io frpserver-sample -oyaml
apiVersion: tunnel.kubeinfra.io/v1alpha1
kind: FRPServer
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"tunnel.kubeinfra.io/v1alpha1","kind":"FRPServer","metadata":{"annotations":{},"name":"frpserver-sample"},"spec":{"allowPorts":[{"end":2000,"start":2000}],"bindAddr":"0.0.0.0","bindPort":17000,"kcpBindPort":17000,"proxyBindAddr":"0.0.0.0","version":"v1","webServer":{"addr":"0.0.0.0","password":"admin","port":7500,"user":"admin"}}}
  creationTimestamp: "2025-05-04T14:09:04Z"
  generation: 1
  name: frpserver-sample
  resourceVersion: "689594"
  uid: 61fcd8f5-a5e4-47af-90e3-b5645b026c5a
spec:
  allowPorts:
  - end: 2000
    start: 2000
  bindAddr: 0.0.0.0
  bindPort: 17000
  detailedErrorsToClient: true
  kcpBindPort: 17000
  natholeAnalysisDataReserveHours: 168
  proxyBindAddr: 0.0.0.0
  udpPacketSize: 1500
  userConnTimeout: 10
  version: v1
  vhostHTTPTimeout: 60
  webServer:
    addr: 0.0.0.0
    password: admin
    port: 7500
    user: admin
status:
  activeConnections:
  - bytesIn: 0
    bytesOut: 0
    clientName: ""
    clientVersion: 0.61.2
    currentConnections: 0
    localAddr: ""
    proxyConfig:
      healthCheck: {}
      loadBalancer: {}
      localIP: 127.0.0.1
      name: mysql.abc
      remotePort: 2000
      transport:
        bandwidthLimitMode: client
      type: tcp
    proxyName: mysql.abc
    proxyType: tcp
    remoteAddr: ""
    startTime: "0000-05-04T23:10:04Z"
    status: online
    todayTrafficIn: 4029
    todayTrafficOut: 4969
  conditions:
  - lastTransitionTime: "2025-05-04T15:11:47Z"
    message: frp server is running
    observedGeneration: 1
    reason: Running
    status: "True"
    type: Ready
  serviceStatus:
    totalBytesIn: 0
    totalBytesOut: 0
    totalClients: 0
    totalConnections: 0
    totalProxies: 0
    uptime: 0s
```