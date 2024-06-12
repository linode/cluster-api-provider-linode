# Distributed Tracing

Distributed tracing is a method used to monitor and track requests as they propagate through various services within a distributed system. In the context of Cluster API Provider Linode (CAPL), distributed tracing can help in identifying performance bottlenecks, debugging issues, and gaining insights into the behavior of the CAPL manager.

## What is Distributed Tracing?

Distributed tracing involves capturing trace data as requests flow through different components of a system. Each trace contains information about the request, including the start and end time, as well as any interactions with other services. This trace data is invaluable for understanding the system's performance and diagnosing issues.

## Why Instrument CAPL with OpenTelemetry?

OpenTelemetry is a collection of tools, APIs, and SDKs used to instrument, generate, collect, and export telemetry data (such as traces, metrics, and logs) to help understand software performance and behavior. By instrumenting the CAPL manager with OpenTelemetry, you can achieve the following benefits:

- **Enhanced Observability**: Gain detailed insights into the internal workings of the CAPL manager.
- **Performance Monitoring**: Identify and address performance bottlenecks within your Kubernetes clusters.
- **Troubleshooting**: Easier debugging of issues by tracing requests through the entire lifecycle.
- **Integration**: Seamlessly integrate with various observability backends that support OpenTelemetry.

## How to Instrument CAPL with OpenTelemetry

To instrument the CAPL manager with OpenTelemetry, follow these steps:

1. **Install Tracing Backend**: Deploy a supported OpenTelemetry backend to collect, process, and export telemetry data. This can be [Zipkin](https://zipkin.io/), [Jaeger](https://www.jaegertracing.io/), or any other collector supporting OpenTelemetry format. Refer to the respective documentation for setup instructions and configuration options.

2. **Configure OpenTelemetry**: Set the necessary environment variables to configure OpenTelemetry for the CAPL manager. The primary environment variable is `OTEL_TRACES_EXPORTER`, which specifies the exporter for trace data.

### Default Configuration

By default, the `OTEL_TRACES_EXPORTER` is set to `none`. This means no trace data will be exported. To enable tracing, you need to set this variable to an appropriate exporter, such as `otlp` for OpenTelemetry Protocol.

To customize the CAPL manager deployment and add extra environment variables, you can use Kustomize. Kustomize is a configuration management tool that allows you to customize Kubernetes resources declaratively.

1. **Create a Kustomization File**: Create a `kustomization.yaml` file in your project directory.

<!-- TODO: write correct kustomization example -->

```yaml
# kustomization.yaml
resources:
  - manager.yaml

patchesStrategicMerge:
  - add-otel-env-vars.yaml
```

2. **Create a Patch File**: Create a patch file, `add-otel-env-vars.yaml`, to add the necessary environment variables.

```yaml
# add-otel-env-vars.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
spec:
  template:
    spec:
      containers:
      - name: manager
        env:
        - name: OTEL_TRACES_EXPORTER
          value: "otlp"
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "http://localhost:4317"
```

3. **Apply the Customization**: Use Kustomize to apply the changes to your CAPL manager deployment.

```sh
kubectl apply -k .
```

## Conclusion

Instrumenting the CAPL manager with OpenTelemetry provides powerful insights into CAPI system's performance and behavior. By setting up distributed tracing, you can enhance observability, improve performance monitoring, and streamline troubleshooting. Using Kustomize, you can easily customize the deployment configuration to include necessary environment variables for OpenTelemetry.

For more detailed configuration options and examples, refer to the [OpenTelemetry documentation](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/).
