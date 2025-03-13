# Disks

This section contains information about [OS](./os-disk.md) and [data](./data-disks.md) disk configuration in Cluster API Provider Linode

## Disk encryption

By default, clusters are provisioned with disk encryption disabled.

For enabling disk encryption, set `spec.template.spec.diskEncryption=enabled` in your generated LinodeMachineTemplate resources when creating a CAPL cluster.

~~~admonish warning
If you see issues with cluster creation after enabling disk encryption, reach out to customer support. Its possible its disabled for your account and needs to be manually enabled.
~~~
