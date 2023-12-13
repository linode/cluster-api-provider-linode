docker_build("controller", ".")

local_resource(
	'capi-controller-manager',
	cmd='clusterctl init',
)

k8s_yaml(kustomize('config/default'))
