# Sidecar

## 业务流程

### 服务代理

Sidecar 作为轻量级组件，负责读取 Pod 注解并实现本地服务注册与发现。其业务流程如下：

1. Sidecar 启动与参数解析  
   1.1 通过命令行参数或环境变量读取：  
     - 配置文件默认挂载在 /etc/servicekeel/ 下，包含从 Pod 注解 `edge.io/exported-services` 和 `edge.io/imported-services` 挂载的配置  
     - `SIDECAR_DNS_ADDR`：DNS 劫持服务器监听地址（默认 `127.0.0.2:53`）  
   1.2 校验参数并打印日志。

2. 启动 DNS 劫持服务器  
   - 创建 `dns.Server`，内部解析并保存 CIDR 网段。  
   - 绑定 UDP 端口，使用 miekg/dns 库后台监听。  
   - 提供：  
     - `AddMapping(name)`：给服务名分配一个未使用的 IP 并记录映射  
     - `RemoveMapping(name)`：删除映射并释放 IP  
     - DNS 请求处理：拦截所有 A 记录查询，查映射表并返回对应 IP，否则返回 NXDOMAIN。

3. 读取配置文件  
   - 从 `SIDECAR_CONFIG_PATH` 读取配置文件，解析出 `exportedServices` 和 `importedServices` 列表。
   - 验证服务配置的完整性，包括：
     - 服务名称、命名空间、集群信息
     - 端口配置（name、port、targetPort、protocol）
     - 协议类型（确保为 "TCP" 或 "UDP"）

4. 服务注册与发现  
   - 对于每个 `exportedServices`：  
     - 通过 `NewServiceClient(...)` 构造 stcp 客户端实例，使用 Unix 套接字建立连接 
     - 根据服务配置的协议类型（TCP/UDP）设置相应的代理规则
     - `ServiceClient.Start()` 启动服务客户端，建立到目标服务的连接  
   - 对于每个 `importedServices`：  
     - 调用 `dnsServer.AddMapping(serviceName)` 分配本地 IP，并填入 `ServiceInfo.MappedIP`  
     - 通过 `NewServiceClient(...)` 构造服务客户端实例，使用 Unix 套接字建立连接  
     - 根据服务配置的协议类型（TCP/UDP）设置相应的代理规则
     - `ServiceClient.Start()` 启动服务客户端，建立到目标服务的连接  

5. DNS + 服务联动代理  
   - 已启动的服务客户端将本地分配的 IP（`MappedIP`）映射到远端目标服务  
   - 应用或进程在 Pod/Host 内通过 DNS 解析服务名时，因 DNS 劫持被指向上述本地 IP  
   - 访问本地 IP 时，流量由服务客户端根据配置的协议类型（TCP/UDP）转发到远端服务，实现透明代理

6. 主流程阻塞  
   - Sidecar 在启动后进入 `select {}` 阻塞，持续提供 DNS + 服务代理

—— 以上即 Sidecar 的业务流程：  
• Sidecar 负责参数解析、DNS 劫持  
• 所有配置在 Pod 创建时已定好，无需 Controller  
• 所有 frpc 连接通过 Unix 套接字实现  
• 根据服务配置的协议类型（TCP/UDP）设置相应的代理规则
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


