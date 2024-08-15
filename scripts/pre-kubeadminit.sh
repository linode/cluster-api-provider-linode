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

cat > /usr/lib/systemd/system/kubelet.service << EOF
[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=https://kubernetes.io/docs/
Wants=network-online.target
After=network-online.target

[Service]
ExecStart=/usr/local/bin/kubelet
Restart=always
StartLimitInterval=0
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

mkdir -p /usr/lib/systemd/system/kubelet.service.d
cat > /usr/lib/systemd/system/kubelet.service.d/10-kubeadm.conf << EOF
# Note: This dropin only works with kubeadm and kubelet v1.11+
[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml"
# This is a file that "kubeadm init" and "kubeadm join" generates at runtime, populating the KUBELET_KUBEADM_ARGS variable dynamically
EnvironmentFile=-/var/lib/kubelet/kubeadm-flags.env
# This is a file that the user can use for overrides of the kubelet args as a last resort. Preferably, the user should use
# the .NodeRegistration.KubeletExtraArgs object in the configuration files instead. KUBELET_EXTRA_ARGS should be sourced from this file.
EnvironmentFile=-/etc/sysconfig/kubelet
ExecStart=
ExecStart=/usr/local/bin/kubelet \$KUBELET_KUBECONFIG_ARGS \$KUBELET_CONFIG_ARGS \$KUBELET_KUBEADM_ARGS \$KUBELET_EXTRA_ARGS
EOF

sed -i '/swap/d' /etc/fstab
swapoff -a
# check for required tools and only install missing tools
REQUIRED_TOOLS=(containerd socat conntrack iptables)
INSTALL_TOOLS=()
for tool in ${REQUIRED_TOOLS[*]}; do
    echo "checking for ${tool}"
    if [ ! -x "$(command -v ${tool})" ]; then
        echo "${tool} is missing"
        INSTALL_TOOLS+=(${tool})
    fi
done
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
# use containerd files we write instead of package defaults
apt-get install -o Dpkg::Options::="--force-confold" -y "${INSTALL_TOOLS[*]}"

PATCH_VERSION=${1#[v]}
VERSION=${PATCH_VERSION%.*}
curl -L "https://github.com/kubernetes-sigs/cri-tools/releases/download/v${VERSION}.0/crictl-v${VERSION}.0-linux-amd64.tar.gz" | tar -C /usr/local/bin -xz
cd /usr/local/bin
curl -L --remote-name-all https://dl.k8s.io/release/$1/bin/linux/amd64/{kubeadm,kubelet}
curl -LO "https://dl.k8s.io/release/v${VERSION}.0/bin/linux/amd64/kubectl"
chmod +x {kubeadm,kubelet,kubectl}
