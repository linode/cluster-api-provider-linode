# Observability

This is a collection of observability tools for cluster-api-provider-linode. This will help us observe and improve cluster-api-provider-linode performance.

There are two observability tools:
- [Prometheus](https://github.com/prometheus-operator/prometheus-operator)
- [Grafana](https://grafana.com)

## Usage

In order to install the observability tools, please run the following commands:

```shell
export CAPL_MONITORING=true
make local-deploy
```
*Note: By setting the environment variable `CAPL_MONITORING=true`, Tilt will deploy the monitoring stack*

Once the monitoring stack is up and running, you can access the Grafana dashboard and prometheus metrics by port-forwarding the prometheus and grafana services:

```shell
kubectl port-forward -n monitoring svc/prometheus-prometheus 9090:9090
kubectl port-forward -n monitoring svc/grafana 8080:80
```

### Grafana

After port-forwarding, you can access the Grafana dashboard by navigating to http://localhost:8080. The default username and password are `admin` and `capl-operator` respectively.