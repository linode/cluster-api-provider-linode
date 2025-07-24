#!/bin/bash
set -euo pipefail

DEFAULT_CONTAINERD_VERSION=1.7.24
DEFAULT_CNI_PLUGIN_VERSIONS=1.6.2
CONTAINERD_VERSION="${CONTAINERD_VERSION:=$DEFAULT_CONTAINERD_VERSION}"
CNI_PLUGIN_VERSIONS="${CNI_PLUGIN_VERSIONS:=$DEFAULT_CNI_PLUGIN_VERSIONS}"
PATCH_VERSION=${1#[v]}
VERSION=${PATCH_VERSION%.*}

# setup containerd config
if ! mkdir -p /etc/containerd ; then
    echo "Error: Failed to create directory /etc/containerd" >&2
    exit 1
fi
chmod 0755 /etc/containerd

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
  [plugins."io.containerd.grpc.v1.cri".registry]
     config_path = "/etc/containerd/certs.d"
  [plugins."io.containerd.grpc.v1.cri".containerd]
     discard_unpacked_layers = false
EOF

chmod 644 /etc/containerd/config.toml

if ! mkdir -p /etc/modules-load.d ; then
    echo "Error: Failed to create directory /etc/modules-load.d" >&2
    exit 1
fi
chmod 0755 /etc/modules-load.d

cat > /etc/modules-load.d/k8s.conf << EOF
overlay
br_netfilter
EOF

chmod 644 /etc/modules-load.d/k8s.conf

if ! mkdir -p /etc/sysctl.d ; then
    echo "Error: Failed to create directory /etc/sysctl.d" >&2
    exit 1
fi
chmod 0755 /etc/sysctl.d

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

if ! mkdir -p /etc/systemd/system.conf.d ; then
    echo "Error: Failed to create directory /etc/systemd/system.conf.d" >&2
    exit 1
fi
chmod 0755 /etc/systemd/system.conf.d

cat > /etc/systemd/system.conf.d/override.conf << EOF
[Manager]
# Set sane defaults for the NOFILE limits to support high-performance workloads:
# - Soft limit (65535): Suitable for most containerized applications.
# - Hard limit (1048576): Allows scaling for high-demand scenarios.
DefaultLimitNOFILE=65535:1048576
EOF

# containerd service
cat > /usr/lib/systemd/system/containerd.service << EOF
[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/containerd

Type=notify
Delegate=yes
KillMode=process
Restart=always
RestartSec=5

# Having non-zero Limit*s causes performance problems due to accounting overhead
# in the kernel. We recommend using cgroups to do container-local accounting.
LimitNPROC=infinity
LimitCORE=infinity
LimitNOFILE=infinity

# Comment TasksMax if your systemd version does not supports it.
# Only systemd 226 and above support this version.
TasksMax=infinity
OOMScoreAdjust=-999

[Install]
WantedBy=multi-user.target
EOF

# kubelet service
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

if ! mkdir -p /usr/lib/systemd/system/kubelet.service.d ; then
    echo "Error: Failed to create directory /usr/lib/systemd/system/kubelet.service.d" >&2
    exit 1
fi
chmod 0755 /usr/lib/systemd/system/kubelet.service.d

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
REQUIRED_TOOLS=(runc socat conntrack ethtool iptables)
INSTALL_TOOLS=()
for tool in "${REQUIRED_TOOLS[@]}"; do
    echo "checking for ${tool}"
    if [ ! -x "$(command -v "${tool}")" ]; then
        echo "${tool} is missing"
        INSTALL_TOOLS+=("${tool}")
    fi
done
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
if [ "${#INSTALL_TOOLS[@]}" -gt 0 ]; then
    apt-get install -y "${INSTALL_TOOLS[@]}"
fi

# install containerd
curl -L "https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-amd64.tar.gz" | tar -C /usr/local -xz

# install cni plugins
if ! mkdir -p /opt/cni/bin ; then
    echo "Error: Failed to create directory /opt/cni/bin" >&2
    exit 1
fi

curl -L "https://github.com/containernetworking/plugins/releases/download/v${CNI_PLUGIN_VERSIONS}/cni-plugins-linux-amd64-v${CNI_PLUGIN_VERSIONS}.tgz" | tar -C /opt/cni/bin -xz
chown -R root:root /opt/cni

# install crictl
curl -L "https://github.com/kubernetes-sigs/cri-tools/releases/download/v${VERSION}.0/crictl-v${VERSION}.0-linux-amd64.tar.gz" | tar -C /usr/local/bin -xz

# install kubeadm,kubelet,kubectl
cd /usr/local/bin
curl -L --remote-name-all "https://dl.k8s.io/release/$1/bin/linux/amd64/{kubeadm,kubelet}"
curl -LO "https://dl.k8s.io/release/v${VERSION}.0/bin/linux/amd64/kubectl"
chmod +x {kubeadm,kubelet,kubectl}

# reload systemd to pick up containerd & kubelet settings
systemctl daemon-reload
systemctl enable --now containerd kubelet
