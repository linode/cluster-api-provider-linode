load("ext://k8s_attach", "k8s_attach")

docker_build("controller", ".", only=("Dockerfile", "Makefile", "vendor","go.mod", "go.sum", "./api", "./cloud","./cmd", "./controller", "./util", "./version"), build_args={'VERSION': os.getenv("VERSION","")})

local_resource(
    'capi-controller-manager',
    cmd='EXP_CLUSTER_RESOURCE_SET=true clusterctl init --addon helm',
)

k8s_yaml(kustomize('config/default'))

# get generated secret name so we can categorize it
token_secret_name = str(local('kustomize build config/default | grep -m1 "name: cluster-api-provider-linode-token-"', quiet=True, echo_off=True)).split()[1]

k8s_resource(
    workload="cluster-api-provider-linode-controller-manager",
    objects=[
       "cluster-api-provider-linode-system:namespace",
       "linodeclusters.infrastructure.cluster.x-k8s.io:customresourcedefinition",
       "linodemachines.infrastructure.cluster.x-k8s.io:customresourcedefinition",
       "linodeclustertemplates.infrastructure.cluster.x-k8s.io:customresourcedefinition",
       "linodemachinetemplates.infrastructure.cluster.x-k8s.io:customresourcedefinition",
       "linodevpcs.infrastructure.cluster.x-k8s.io:customresourcedefinition",
       "cluster-api-provider-linode-controller-manager:serviceaccount",
       "cluster-api-provider-linode-leader-election-role:role",
       "cluster-api-provider-linode-manager-role:clusterrole",
       "cluster-api-provider-linode-metrics-reader:clusterrole",
       "cluster-api-provider-linode-proxy-role:clusterrole",
       "cluster-api-provider-linode-leader-election-rolebinding:rolebinding",
       "cluster-api-provider-linode-manager-rolebinding:clusterrolebinding",
       "cluster-api-provider-linode-proxy-rolebinding:clusterrolebinding",
       "%s:secret" % token_secret_name
   ]
)
