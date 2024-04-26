load("ext://k8s_attach", "k8s_attach")
load("ext://helm_resource", "helm_resource", "helm_repo")
load("ext://namespace", "namespace_create")
update_settings(k8s_upsert_timeout_secs=60)

helm_repo("capi-operator-repo", "https://kubernetes-sigs.github.io/cluster-api-operator",labels=["helm-repos"])
helm_repo("jetstack-repo", "https://charts.jetstack.io", labels=["helm-repos"])
helm_resource(
    "cert-manager",
    "jetstack-repo/cert-manager",
    namespace="cert-manager",
    resource_deps=["jetstack-repo"],
    flags=[
        "--create-namespace",
        "--set=installCRDs=true",
        "--set=global.leaderElection.namespace=cert-manager",
    ],
    labels=["cert-manager"],
)

helm_resource(
    "capi-operator",
    "capi-operator-repo/cluster-api-operator",
    namespace="capi-operator-system",
    flags=["--create-namespace", "--wait"],
    resource_deps=["capi-operator-repo", "cert-manager"],
    labels=["CAPI"],
)
namespace_create("capi-system")
k8s_yaml("./hack/manifests/core.yaml")
k8s_resource(
    new_name="capi-controller-manager",
    objects=["capi-system:namespace", "cluster-api:coreprovider"],
    resource_deps=["capi-operator"],
    labels=["CAPI"],
)
if os.getenv("INSTALL_KUBEADM_PROVIDER", "true") == "true":
    namespace_create("kubeadm-control-plane-system")
    namespace_create("kubeadm-bootstrap-system")
    k8s_yaml("./hack/manifests/kubeadm.yaml")
    k8s_resource(
        new_name="kubeadm-controller-manager",
        objects=[
            "kubeadm-bootstrap-system:namespace",
            "kubeadm-control-plane-system:namespace",
            "kubeadm:bootstrapprovider",
            "kubeadm:controlplaneprovider",
        ],
        resource_deps=["capi-controller-manager"],
        labels=["CAPI"],
    )

if os.getenv("INSTALL_HELM_PROVIDER", "true") == "true":
    namespace_create("caaph-system")
    k8s_yaml("./hack/manifests/helm.yaml")
    k8s_resource(
        new_name="helm-controller-manager",
        objects=["caaph-system:namespace", "helm:addonprovider"],
        resource_deps=["capi-controller-manager"],
        labels=["CAPI"],
    )

if os.getenv("INSTALL_K3S_PROVIDER", "false") == "true":
    namespace_create("capi-k3s-control-plane-system")
    namespace_create("capi-k3s-bootstrap-system")
    k8s_yaml("./hack/manifests/k3s.yaml")
    k8s_resource(
        new_name="k3s-controller-manager",
        objects=[
            "capi-k3s-bootstrap-system:namespace",
            "capi-k3s-control-plane-system:namespace",
            "k3s:bootstrapprovider",
            "k3s:controlplaneprovider",
        ],
        resource_deps=["capi-controller-manager"],
        labels=["CAPI"],
    )

if os.getenv("INSTALL_RKE2_PROVIDER", "false") == "true":
    namespace_create("rke2-control-plane-system")
    namespace_create("rke2-bootstrap-system")
    k8s_yaml("./hack/manifests/rke2.yaml")
    k8s_resource(
        new_name="capi-rke2-controller-manager",
        objects=[
            "rke2-bootstrap-system:namespace",
            "rke2-control-plane-system:namespace",
            "rke2:bootstrapprovider",
            "rke2:controlplaneprovider",
        ],
        resource_deps=["capi-controller-manager"],
        labels=["CAPI"],
    )

manager_yaml = decode_yaml_stream(kustomize("config/default"))
for resource in manager_yaml:
    if resource["metadata"]["name"] == "capl-manager-credentials":
        resource["stringData"]["apiToken"] = os.getenv("LINODE_TOKEN")
    if resource["kind"] == "CustomResourceDefinition" and resource["spec"]["group"] == "infrastructure.cluster.x-k8s.io":
        resource["metadata"]["labels"]["clusterctl.cluster.x-k8s.io"] = ""
k8s_yaml(encode_yaml_stream(manager_yaml))

if os.getenv("SKIP_DOCKER_BUILD", "false") != "true":
    docker_build(
        "docker.io/linode/cluster-api-provider-linode",
        context=".",
        only=("Dockerfile", "Makefile", "vendor", "go.mod", "go.sum",
        "./api", "./cloud", "./cmd", "./controller", "./util", "./version",),
        build_args={"VERSION": os.getenv("VERSION", "")},
    )

k8s_resource(
    workload="capl-controller-manager",
    objects=[
        "capl-system:namespace",
        "linodeclusters.infrastructure.cluster.x-k8s.io:customresourcedefinition",
        "linodemachines.infrastructure.cluster.x-k8s.io:customresourcedefinition",
        "linodeclustertemplates.infrastructure.cluster.x-k8s.io:customresourcedefinition",
        "linodemachinetemplates.infrastructure.cluster.x-k8s.io:customresourcedefinition",
        "linodevpcs.infrastructure.cluster.x-k8s.io:customresourcedefinition",
        "linodeobjectstoragebuckets.infrastructure.cluster.x-k8s.io:customresourcedefinition",
        "capl-controller-manager:serviceaccount",
        "capl-leader-election-role:role",
        "capl-manager-role:clusterrole",
        "capl-metrics-reader:clusterrole",
        "capl-proxy-role:clusterrole",
        "capl-leader-election-rolebinding:rolebinding",
        "capl-manager-rolebinding:clusterrolebinding",
        "capl-proxy-rolebinding:clusterrolebinding",
        "capl-manager-credentials:secret",
    ],
    resource_deps=["capi-controller-manager"],
    labels=["CAPL"],
)
