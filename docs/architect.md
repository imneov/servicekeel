# 设计文档：边缘环境下基于 Pod 注解的服务注册与发现机制

## 一、背景与动机

在边缘计算场景中，节点常常处于网络不可达或弱连接状态，导致 Kubernetes 原生的基于 API Server 的服务注册与发现机制无法正常工作。
为实现边缘自治，保障服务间通信，我们需要一种在本地也能运行的轻量级服务注册与发现方案。

我们借鉴 Kubernetes KEP-1645（Multi-Cluster Services API）中服务导出（Export）与导入（Import）的概念，
在边缘环境下采用注解（Annotation）和标签（Label）作为服务注册与发现信息的载体，从而绕过中心化控制面依赖，增强系统的自治能力。

## 二、总体设计思路

本设计提出以下关键机制：

- 服务注册：根据 Pod 被哪些 Service 选中，在其注解中记录应导出的服务名称（包含命名空间、集群信息和协议类型）。
- 服务导入：在 Pod 注解中声明该 Pod 需要访问的服务，可指定 namespace、cluster 和协议类型。
- 边缘代理 Sidecar/Agent：读取注解信息，完成服务注册与发现的本地逻辑，支持不同协议（TCP/UDP）的处理。

## 三、关键定义

- `edge.io/exported-services`：表示该 Pod 所需注册的服务列表，由 Service selector 决定。
- `edge.io/imported-services`：表示该 Pod 需访问的服务列表，由用户或 Controller 指定。

注解值格式统一为 JSON 序列化的字符串，例如：

```json
{
  "services": [
    {
      "name": "service-name-1",
      "namespace": "namespace-1",
      "cluster": "cluster-1",
      "ports": [
        {"name": "http", "port": 8080, "targetPort": 8080, "protocol": "TCP"},
        {"name": "metrics", "port": 9090, "targetPort": 9090, "protocol": "UDP"}
      ]
    },
    {
      "name": "service-name-2",
      "namespace": "namespace-2",
      "cluster": "cluster-1",
      "ports": [
        {"name": "grpc", "port": 9000, "targetPort": 9000, "protocol": "TCP"}
      ]
    }
  ]
}
```

### 3.1 端口配置说明

每个服务的端口配置包含以下字段：
- `name`：端口名称，用于标识用途（如 http、metrics、grpc 等）
- `port`：服务对外暴露的端口
- `targetPort`：容器内部实际监听的端口
- `protocol`：传输协议，支持 "TCP" 或 "UDP"

## 四、设计细节

### 4.1 服务注册流程

1. Controller 监听 Service 与 Pod 变更。
2. 判断 Service 的 selector 是否匹配某个 Pod。
3. 若匹配，将服务信息（包括协议类型）添加到 Pod 的 `edge.io/exported-services` 注解中。

### 4.2 服务导入流程

1. 用户可直接在 Pod spec 中通过 `edge.io/imported-services` 注解声明所需访问的服务，格式同导出。
2. 注解支持省略 namespace 与 cluster（默认为当前）。
3. 边缘 Agent 负责将导入服务建立为本地代理，根据协议类型（TCP/UDP）配置相应的代理规则。

### 4.3 示例 YAML

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app
  namespace: default
  labels:
    app: my-app
  annotations:
    edge.io/exported-services: |
      {
        "services": [
          {
            "name": "my-service",
            "namespace": "default",
            "cluster": "cluster-a",
            "ports": [
              {"name": "http", "port": 80, "targetPort": 8080, "protocol": "TCP"}
            ]
          },
          {
            "name": "metrics",
            "namespace": "default",
            "cluster": "cluster-a",
            "ports": [
              {"name": "metrics", "port": 9090, "targetPort": 9090, "protocol": "UDP"}
            ]
          }
        ]
      }
    edge.io/imported-services: |
      {
        "services": [
          {
            "name": "db-service",
            "namespace": "default",
            "cluster": "cluster-b",
            "ports": [
              {"name": "http", "port": 80, "targetPort": 8080, "protocol": "TCP"}
            ]
          },
          {
            "name": "cache",
            "namespace": "default",
            "cluster": "cluster-b",
            "ports": [
              {"name": "metrics", "port": 9090, "targetPort": 9090, "protocol": "UDP"}
            ]
          }
        ]
      }
spec:
  containers:
    - name: main
      image: app:v1
```

### 4.4 服务名称约定

- `<serviceName>`（必填）
- `<namespace>`（可选，默认当前 Pod 所在 namespace）
- `<cluster>`（可选，默认当前集群）

使用完整标识可确保跨集群唯一性。

## 五、系统组件说明

### 5.1 边缘 Agent / Sidecar

- 监听 Pod 注解 `edge.io/exported-services` 和 `edge.io/imported-services`。
- 向本地注册表（如 Consul、Envoy SDS、自定义注册表）注册导出服务。
- 为导入服务建立代理或 DNS 重写，实现本地访问。
- 根据服务配置的协议类型（TCP/UDP）设置相应的代理规则。

### 5.2 Controller

- 部署于中心或边缘控制面。
- Watch Service 与 Pod，识别匹配关系，为 Pod 自动补充 `edge.io/exported-services` 注解。
- 可通过 CR 模板或 Admission Webhook 为 Pod 注入默认的 `edge.io/imported-services`。

## 六、兼容性与边缘断网处理

- 注解信息一经写入，可被边缘组件脱离 API Server 独立使用。
- Controller 可在网络可达时批量同步注解，断网前保证信息完整。
- 协议类型信息确保在断网情况下仍能正确处理不同类型的服务流量。

## 七、附加功能建议

- 提供将 CR 转注解的转换工具或 Webhook。
- 支持将服务端点信息（如 port、protocol）一起编码进注解。
- 考虑为 StatefulSet 增强 Pod 注解的自动继承机制。
- 添加协议类型验证，确保只支持 TCP 和 UDP。

## 八、总结

通过在 Pod 注解中记录服务注册与导入信息，包括协议类型等详细配置，实现轻量、灵活、无侵入的多集群服务注册与发现，适配边缘场景下的网络不稳定与自治需求。
后续可结合服务网格、服务目录系统进一步扩展，例如与 Envoy SDS、CoreDNS、自定义注册表结合。
