#!/bin/sh
# <UDF name="instancedata" label="instance-data contents(base64 encoded" />
# <UDF name="userdata" label="user-data file contents (base64 encoded)" />

cat > /etc/cloud/cloud.cfg.d/100_none.cfg <<EOF
datasource_list: [ "None"]
datasource:
  None:
    metadata:
      id: $LINODE_ID
$(echo "${INSTANCEDATA}" | base64 -d | sed "s/^/      /")
    userdata_raw: |
$(echo "${USERDATA}" | base64 -d | sed "s/^/      /")

EOF

cloud-init clean
cloud-init -f /etc/cloud/cloud.cfg.d/100_none.cfg init --local
cloud-init -f /etc/cloud/cloud.cfg.d/100_none.cfg init
cloud-init -f /etc/cloud/cloud.cfg.d/100_none.cfg modules --mode=config
cloud-init -f /etc/cloud/cloud.cfg.d/100_none.cfg modules --mode=final
