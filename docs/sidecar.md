# Sidecar


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