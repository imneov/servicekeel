#  kube-apiserver



## 安装



```bash
# 在 Docker 中启动 K3s Server
docker run --privileged -d --name k3s-server \
  -p 7443:6443 \
  -v /var/lib/rancher/k3s:/var/lib/rancher/k3s \
  -v /etc/rancher/k3s:/etc/rancher/k3s \
  rancher/k3s:v1.28.15-k3s1 server \
    --disable-agent \
    --disable-controller-manager \
    --disable-scheduler \
    --disable scheduler,coredns,servicelb,traefik,local-storage,metrics-server \
    --tls-san 172.31.19.105 \
    --tls-san '*.edgewize.io'
```
172.31.19.105 为节点 IP


### 3. 以 DaemonSet 方式运行

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: k3s-server
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: k3s-server
  template:
    metadata:
      labels:
        app: k3s-server
    spec:
      hostNetwork: false
      containers:
      - name: k3s
        image: rancher/k3s:v1.28.15-k3s1
        ports:
        - containerPort: 6443
          hostPort: 7443
          protocol: TCP
        volumeMounts:
        - name: data
          mountPath: /var/lib/rancher/k3s
        - name: etc
          mountPath: /etc/rancher/k3s
        args:
        - server
        - --disable-agent
        - --disable-controller-manager
        - --disable-scheduler
        - --disable
        - scheduler,coredns,servicelb,traefik,local-storage,metrics-server
        - --tls-san
        - 172.31.19.105
        - --tls-san
        - '*.edgewize.io'
      volumes:
      - name: data
        hostPath:
          path: /var/lib/rancher/k3s
      - name: etc
        hostPath:
          path: /etc/rancher/k3s
```

### 配置访问

```bash
mkdir -p $HOME/.kube
sudo cp /etc/rancher/k3s/k3s.yaml $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
kubectl get namespaces
```
