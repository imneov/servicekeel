# ServiceKeel Simple Game Test Example

这个示例展示了如何使用 ServiceKeel 在跨集群环境中测试 TCP 和 UDP 服务的连接性。示例包含了一个简单的客户端-服务器架构，用于验证服务发现和网络连接功能。

## 架构说明

示例包含以下组件：

1. **服务器端 (server-pod.yaml)**
   - TCP 服务器：运行 nginx 提供 HTTP 服务
   - UDP 服务器：运行 chronyd 提供 NTP 服务
   - ServiceKeel sidecar：处理服务发现和网络连接

2. **客户端 (client-pod.yaml)**
   - 测试客户端：定期测试与服务器的 TCP 和 UDP 连接
   - ServiceKeel sidecar：处理服务发现和网络连接

3. **Nginx 配置 (nginx-config.yaml)**
   - 提供两个 HTTP 端点：
     - `/`：返回欢迎消息和状态信息
     - `/health`：健康检查端点

## 功能特点

- **TCP 服务测试**
  - HTTP 请求测试
  - JSON 响应验证
  - 健康检查端点

- **UDP 服务测试**
  - NTP 时间同步测试
  - 时间偏差测量
  - 网络延迟测试

## 部署步骤

1. 创建 Nginx 配置：
```bash
kubectl apply -f nginx-config.yaml
```

2. 创建 Chrony 配置：
```bash
kubectl apply -f chrony-config.yaml
```

3. 部署服务器：
```bash
kubectl apply -f server-pod.yaml
```

4. 部署客户端：
```bash
kubectl apply -f client-pod.yaml
```

## 验证测试

部署完成后，客户端会每 5 秒执行一次测试，包括：

1. TCP 连接测试：
   - 访问主端点 `/`
   - 访问健康检查端点 `/health`

2. UDP 连接测试：
   - 使用 ntpdate 查询 NTP 服务器
   - 测量时间偏差和网络延迟

## 预期输出

客户端会输出类似以下内容：

```
=========================
Testing simple-server...
TCP Test:
{"status": "success", "message": "Hello from ServiceKeel!", "timestamp": "..."}

Health Check:
{"status": "healthy", "timestamp": "..."}

UDP Test (NTP):
server simple-server.default, stratum 3, offset 0.000123, delay 0.001234
=========================
```

这个输出会每 5 秒重复一次，分别测试 simple-server 和 simple-server2 两个服务。

## 配置说明

### 服务发现配置

服务器和客户端都通过 ServiceKeel 注解配置服务发现：

```yaml
annotations:
  servicekeel.io/exported-services: |
    services:
      - cluster: cluster-a.local
        name: simple-server
        namespace: default
        ports:
          - name: tcp
            port: 8080
            protocol: TCP
          - name: udp
            port: 8081
            protocol: UDP
```

### Sidecar 配置

ServiceKeel sidecar 配置包括：

- IP 范围配置
- DNS 服务器地址
- FRP 服务器监听地址

## 注意事项

1. 确保 ServiceKeel 控制平面已正确部署
2. 检查集群间网络连接是否正常
3. 验证 DNS 配置是否正确
4. 确保所需的端口未被其他服务占用 