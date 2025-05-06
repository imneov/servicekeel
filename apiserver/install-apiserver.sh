#!/usr/bin/env bash
set -euo pipefail

# 默认 API Server 地址，可通过环境变量 APISERVER_IP 覆盖
APISERVER_IP="${APISERVER_IP:-172.31.19.105}"

# 下载并安装 K3s，只保留 API Server 和 kine，监听 7443 端口
echo "==> Downloading K3s install script"
curl -sfL https://get.k3s.io -o install-k3s.sh
chmod +x install-k3s.sh

echo "==> Installing K3s (API Server + kine)"
sudo ./install-k3s.sh \
  --disable-agent \
  --disable scheduler,coredns,servicelb,traefik,local-storage,metrics-server \
  --https-listen-port 7443

# 启用并启动服务
echo "==> Enabling and starting k3s service"
sudo systemctl enable k3s
sudo systemctl start k3s

# 等待服务就绪
echo "==> Waiting for K3s to be ready"
sleep 5

# 配置 kubeconfig
echo "==> Configuring kubeconfig"
mkdir -p "$HOME/.kube"
sudo cp /etc/rancher/k3s/k3s.yaml "$HOME/.kube/config"
sudo chown $(id -u):$(id -g) "$HOME/.kube/config"

# 替换 server 地址为外部 IP
echo "==> Updating kubeconfig server address to https://${APISERVER_IP}:7443"
sed -i "s|https://127.0.0.1:7443|https://${APISERVER_IP}:7443|" "$HOME/.kube/config"

# 验证安装
echo "==> Testing cluster access"
kubectl get namespaces

echo "==> Install complete!" 