#!/bin/bash

set -e
set -x

test_image="quay.io/openshift-kni/cnf-tests:4.18"
here="$(dirname "$(readlink --canonicalize "${BASH_SOURCE[0]}")")"


function cleanup {
  ip link delete enp175s0f1
}
trap cleanup EXIT


ip netns del test_container_ns || true
ip netns add test_container_ns


ip netns del test_root_ns || true
ip netns add test_root_ns

#ip link add enp175s0f1 type dummy
#ip link set enp175s0f1 multicast on
#ip addr add 10.10.0.1/24 dev dummy0
#ip link set enp175s0f1 up

# podman pull ${test_image}



podman kill sriov-cni-test || true
podman rm -f sriov-cni-test
container_id=`podman run -d --network ns:/run/netns/test_container_ns --name sriov-cni-test ${test_image} sleep inf`
#netns=`podman inspect --format "{{.NetworkSettings.SandboxKey}}" $container_id`
#netns=${netns#/run/user/1000/netns/}
netns=/run/netns/test_container_ns

export SRIOV_MOCK_ENVIRONMENT=TRUE
export CNI_CONTAINERID=${container_id}
export CNI_COMMAND=ADD
export CNI_PATH=${here}
export CNI_NETNS=${netns}
export CNI_IFNAME=net1

#sleep 1


cat <<EOF | go run cmd/sriov/main.go
{
  "type": "sriov",
  "cniVersion": "0.3.1",
  "name": "sriov-network",
  "ipam": {
    "type": "test-ipam-cni"
  },
  "deviceID": "0000:af:06.0",
  "logLevel": "debug"
}
EOF



#cat <<EOF | go run cmd/sriov/main.go
#{
#  "type": "sriov",
#  "cniVersion": "0.3.1",
#  "name": "sriov-network",
#  "ipam": {
#    "type": "host-local",
#    "subnet": "10.56.217.0/24",
#    "routes": [{
#      "dst": "0.0.0.0/0"
#    }],
#    "gateway": "10.56.217.1"
#  },
#  "deviceID": "0000:af:06.0",
#  "logLevel": "debug"
#}
#EOF
