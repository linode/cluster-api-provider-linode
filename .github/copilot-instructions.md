# Cluster API Provider Linode (CAPL) - AI Agent Instructions

## Project Overview
CAPL is a Kubernetes Cluster API infrastructure provider for Linode/Akamai cloud services. It enables declarative management of Kubernetes clusters on Linode infrastructure using native Kubernetes APIs and follows the Cluster API v1beta1 specification.

## Architecture Patterns

### Controller-Scope-Service Architecture
All infrastructure resources follow a three-layer pattern:
1. **Controllers** (`internal/controller/`) - Handle Kubernetes reconciliation events
2. **Scopes** (`cloud/scope/`) - Encapsulate reconciliation context with both K8s and Linode clients
3. **Services** (`cloud/services/`) - Abstract Linode API interactions (loadbalancers, domains, object storage)

### Standard Controller Structure
```go
func (r *LinodeResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch the resource
    // 2. Create scope with clients: scope.NewResourceScope(ctx, r.LinodeClientConfig, params)
    // 3. Check pause conditions
    // 4. Call reconcile helper with proper defer for status updates
    // 5. Handle errors and events
}
```

### Scope Pattern Usage
Every resource controller creates a scope that manages:
- Kubernetes client for CRD operations
- Linode client for API calls
- Resource references and credentials
- Patch helpers for status updates

Example: `scope.NewClusterScope()` combines `LinodeCluster` + CAPI `Cluster` resources.

## Key Resource Types

### Core Infrastructure
- **LinodeCluster**: Cluster networking, load balancers (NodeBalancer/DNS), VPC references, firewalls
- **LinodeMachine**: Compute instances with placement groups, disk configuration, networking
- **LinodeVPC**: Virtual Private Cloud with IPv4/IPv6 subnets
- **LinodeFirewall**: Cloud firewall rules with AddressSet references
- **LinodePlacementGroup**: Anti-affinity constraints for high availability

### API Design Conventions
- **Dual Reference Pattern**: Support both direct IDs (`vpcID: 123`) and K8s object refs (`vpcRef: {name: "vpc-1"}`)
- **Credential References**: All resources support `credentialsRef` for multi-tenancy
- **Immutable Fields**: Use `+kubebuilder:validation:XValidation:rule="self == oldSelf"` for region, type, etc.
- **Status Structure**: Always include `ready`, `failureReason`, `failureMessage`, `conditions`

## Development Workflows

### Build & Test Commands
- `make generate` - Regenerate CRDs and mocks after API changes
- `make test` - Run unit tests with mocked clients
- `make e2e E2E_SELECTOR=quick` - Run specific E2E tests using Chainsaw
- `make lint` - Run golangci-lint with project-specific rules
- `make build` - Build the controller manager binary

### Adding New Resources
1. Define API types in `api/v1alpha2/` with proper validation markers
2. Implement controller in `internal/controller/` following the standard pattern
3. Add scope in `cloud/scope/` for client management
4. Add validation webhook in `internal/webhook/v1alpha2/`
5. Add cloud services in `cloud/services/` if needed
6. Run `make generate` to update CRDs and mocks
7. Add E2E tests in `e2e/<resource>-controller/`

### Testing Patterns
- Unit tests use GoMock with `mock.MockLinodeClient` and `mock.MockK8sClient`
- Mock expectations pattern: `mockLinodeClient.EXPECT().Method().Return(result, error)`
- E2E tests use Chainsaw YAML manifests in `e2e/` directories organized by controller and flavors
- Service tests mock both success and error scenarios from Linode API
- Test naming uses dynamic identifiers: `(join('-', ['e2e', 'feature', env('GIT_REF')]))`
- Table-driven tests with `name`, `objects`, `expectedError`, `expectedResult`, `expectations` structure

## Linode Platform Integration

### Load Balancer Types
- **NodeBalancer**: Linode's managed load balancer for cluster API endpoints
- **DNS**: Uses Linode or Akamai DNS for API endpoint resolution
- **External**: For existing external load balancers

### Networking Features
- **VPC**: Private networking with configurable IPv4/IPv6 subnets
- **Firewalls**: Cloud firewalls with inbound/outbound rules and AddressSet reuse
- **Placement Groups**: Anti-affinity for spreading instances across failure domains

### Bootstrap Integration
- Supports kubeadm, k3s, and rke2 bootstrap providers
- Uses cloud-init with Linode's metadata service
- Object storage integration for large bootstrap payloads via pre-signed URLs

## Common Patterns

### Standard Reconciliation Structure
```go
func (r *Controller) reconcile(ctx context.Context, scope *ScopeType) (res ctrl.Result, err error) {
    scope.Resource.Status.Ready = false
    scope.Resource.Status.FailureReason = nil
    
    defer func() {
        if err != nil {
            scope.Resource.Status.FailureReason = util.Pointer("ReconcileError")
            scope.Resource.Status.FailureMessage = util.Pointer(err.Error())
        }
        if patchErr := scope.Close(ctx); patchErr != nil {
            err = errors.Join(err, patchErr)
        }
    }()
    
    // Add finalizer, handle deletion, or ensure resource
}
```

### Error Handling
```go
// Ignore specific HTTP errors
if util.IgnoreLinodeAPIError(err, http.StatusNotFound) != nil {
    return fmt.Errorf("failed to get resource: %w", err)
}

// Handle retryable vs terminal errors
if linodego.ErrHasStatus(err, http.StatusBadRequest) {
    // Terminal error - set failure reason, don't requeue
    return ctrl.Result{}, fmt.Errorf("terminal error: %w", err)
}
// Retryable error - requeue with backoff
return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
```

### Finalizer Management
Add finalizers early in reconciliation, remove during deletion after cleanup.

### Credential Resolution
Controllers resolve credentials from `credentialsRef` Secret or default to cluster-wide token.

### Template System
Cluster templates in `templates/flavors/` define different configurations (VPC vs vpcless, dual-stack networking, etc.).

## Environment Variables

### Core Authentication & API
- `LINODE_TOKEN`: Primary Linode API authentication token (required)
- `LINODE_DNS_TOKEN`: Separate token for DNS operations (optional, defaults to LINODE_TOKEN)
- `LINODE_URL`: Custom Linode API endpoint (optional, for testing/dev environments)
- `LINODE_DNS_URL`: Custom DNS API endpoint (optional)
- `LINODE_DNS_CA`: Custom CA certificate for DNS API (optional)
- `LINODE_CA_BASE64`: Base64-encoded CA certificate for Linode API (optional)

### Akamai Integration
- `AKAMAI_HOST`: Akamai EdgeRC API hostname
- `AKAMAI_CLIENT_TOKEN`: Akamai EdgeRC client token  
- `AKAMAI_CLIENT_SECRET`: Akamai EdgeRC client secret
- `AKAMAI_ACCESS_TOKEN`: Akamai EdgeRC access token

### Development & Debugging
- `CAPL_DEBUG`: Enable debug logging and OpenTelemetry tracing (`true`/`false`)
- `CAPL_MONITORING`: Enable Prometheus metrics and Grafana dashboards (`true`/`false`)
- `ENABLE_WEBHOOKS`: Enable/disable admission webhooks (`true`/`false`)
- `GZIP_COMPRESSION_ENABLED`: Enable gzip compression for metadata (`true`/`false`)
- `SKIP_DOCKER_BUILD`: Skip Docker build in Tilt development (`true`/`false`)
- `VERSION`: Build version override

### Provider Installation (Tilt Development)
- `INSTALL_KUBEADM_PROVIDER`: Install kubeadm bootstrap/control-plane providers (`true`/`false`, default: `true`)
- `INSTALL_HELM_PROVIDER`: Install Cluster API Addon Provider Helm (`true`/`false`, default: `true`)
- `INSTALL_K3S_PROVIDER`: Install K3s bootstrap/control-plane providers (`true`/`false`, default: `false`)
- `INSTALL_RKE2_PROVIDER`: Install RKE2 bootstrap/control-plane providers (`true`/`false`, default: `false`)

### Cluster Configuration (Templates)
- `CLUSTER_NAME`: Name for generated clusters
- `LINODE_REGION`: Default Linode region (e.g., `us-ord`, `us-sea`)
- `LINODE_CONTROL_PLANE_MACHINE_TYPE`: Instance type for control plane nodes (e.g., `g6-standard-2`)
- `LINODE_MACHINE_TYPE`: Instance type for worker nodes (e.g., `g6-standard-2`)
- `LINODE_SSH_PUBKEY`: SSH public key for cluster node access

### DNS LoadBalancer Configuration
- `DNS_ROOT_DOMAIN`: Root domain for DNS-based load balancing (e.g., `example.com`)
- `DNS_UNIQUE_ID`: Unique identifier for DNS records (e.g., `abc123`)

### Backup & Storage
- `OBJ_BUCKET_REGION`: Object storage region for etcd backups (e.g., `us-ord`)
- `ETCDBR_IMAGE`: Custom etcd backup/restore controller image
- `SSE_KEY`: Server-side encryption key for object storage

### E2E Testing
- `E2E_SELECTOR`: Chainsaw test selector (`quick`, `all`, `flavors`, `linodecluster`, etc.)
- `E2E_FLAGS`: Additional flags passed to Chainsaw (e.g., `--assert-timeout 10m0s`)
- `CLUSTER_AUTOSCALER_VERSION`: Version for cluster autoscaler tests (e.g., `v1.29.0`)

### OpenTelemetry Tracing
Standard OpenTelemetry environment variables are supported via `autoexport` package:
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP endpoint URL
- `OTEL_EXPORTER_JAEGER_ENDPOINT`: Jaeger endpoint URL  
- `OTEL_SERVICE_NAME`: Service name for traces
- `OTEL_RESOURCE_ATTRIBUTES`: Additional resource attributes

## Debugging Tips
- Check controller logs for reconciliation errors and API failures
- Verify Linode API permissions and regional capabilities  
- Use `kubectl describe` on resources to see status conditions and events
- E2E test failures often indicate webhook validation or API compatibility issues
- Enable debug logging with `CAPL_DEBUG=true` for detailed tracing
- Validate CRDs are current with `make generate` after API changes
- Generate local-release whenever making changes to the /templates directory. Use command `make local-release` to generate the release files.
