load("ext://k8s_attach", "k8s_attach")

docker_build(
    "docker.io/linode/cluster-api-provider-linode",
    context = ".",
    only=("Dockerfile", "Makefile", "vendor","go.mod", "go.sum", "./api", "./cloud","./cmd", "./controller", "./util", "./version"),
    build_args={'VERSION': os.getenv("VERSION","")},
)

local_resource(
    'capi-controller-manager',
    cmd='EXP_CLUSTER_RESOURCE_SET=true clusterctl init --addon helm',
)

manager_yaml = decode_yaml_stream(kustomize("config/default"))
for resource in manager_yaml:
    if resource["metadata"]["name"] == "capl-manager-credentials":
        resource["stringData"]["apiToken"] = os.getenv('LINODE_TOKEN')
k8s_yaml(encode_yaml_stream(manager_yaml))

k8s_resource(
    workload="capl-controller-manager",
    objects=[
       "capl-system:namespace",
       "linodeclusters.infrastructure.cluster.x-k8s.io:customresourcedefinition",
       "linodemachines.infrastructure.cluster.x-k8s.io:customresourcedefinition",
       "linodeclustertemplates.infrastructure.cluster.x-k8s.io:customresourcedefinition",
       "linodemachinetemplates.infrastructure.cluster.x-k8s.io:customresourcedefinition",
       "linodevpcs.infrastructure.cluster.x-k8s.io:customresourcedefinition",
       "capl-controller-manager:serviceaccount",
       "capl-leader-election-role:role",
       "capl-manager-role:clusterrole",
       "capl-metrics-reader:clusterrole",
       "capl-proxy-role:clusterrole",
       "capl-leader-election-rolebinding:rolebinding",
       "capl-manager-rolebinding:clusterrolebinding",
       "capl-proxy-rolebinding:clusterrolebinding",
       "capl-manager-credentials:secret",
   ]
)
