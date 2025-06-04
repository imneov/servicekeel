## 路线图 (Roadmap)

### v0.0.1: Sidecar
- 环境变量解析：支持 `SIDECAR_MAPPED_SERVICES`、`SIDECAR_IP_RANGE`。
- 实现 APIServer 客户端：拉取 Router CR 状态列表。
- 构建 frpc 注册模块：
  - Dial 模式（stcp）：将远端服务映射至本地。
  - Bind 模式（visitor）：发布 Pod 内服务供外部访问。
- DNS 劫持模块：监听 `127.0.0.2:53/udp`，劫持服务名解析并返回本地代理地址。
- 本地缓存与状态上报：记录映射端口和连接状态。
- 单元测试与集成测试：验证映射与 DNS 劫持功能。
  - 在外部节点上运行 APISERVER 以及 FRPS
  - 部署 Pod 到这个节点实现相关流程 

### v0.0.2: 

### v0.1.x: MVP
- 实现基础 APIServer、Router、Syncer、Sidecar 组件
- 支持基本的 stcp 映射和 DNS 劫持功能
- 本地环境验证和 e2e 测试

### v0.2.x: 稳定性与安全
- stcp 密钥加密、token 认证支持
- Router 端负载均衡与自动断链重连
- Sidecar 热更新服务列表功能
- 优化 Syncer 同步延迟并支持增量更新

### v0.3.x: 扩展能力
- 支持 xtcp 等更多隧道协议
- 多区域/多集群策略与流量划分
- Dashboard 可视化管理界面
- 支持更多编排系统（Kubernetes CRD 官方控制器）

### v1.0.0: GA 发布
- 完善文档、示例场景
- 性能测试与压力测试报告
- 社区版本发布，支持插件扩展
