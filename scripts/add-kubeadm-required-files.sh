#!/bin/bash
set -euo pipefail

mkdir -p -m 755 /etc/containerd
cat > /etc/containerd/config.toml << EOF
version = 2
imports = ["/etc/containerd/conf.d/*.toml"]
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "registry.k8s.io/pause:3.9"
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
    runtime_type = "io.containerd.runc.v2"
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
    SystemdCgroup = true
EOF

chmod 644 /etc/containerd/config.toml

mkdir -p -m 755 /etc/modules-load.d
cat > /etc/modules-load.d/k8s.conf << EOF
overlay
br_netfilter
EOF

chmod 644 /etc/modules-load.d/k8s.conf

mkdir -p -m 755 /etc/sysctl.d
cat > /etc/sysctl.d/k8s.conf << EOF
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
net.ipv6.conf.all.forwarding        = 1
EOF

chmod 644 /etc/sysctl.d/k8s.conf

modprobe overlay
modprobe br_netfilter
sysctl --system
