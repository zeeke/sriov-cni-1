#!/bin/bash

test_image="docker.io/library/busybox:1.36"
here="$(dirname "$(readlink --canonicalize "${BASH_SOURCE[0]}")")"

setup() {
  ip netns del test_root_ns || true
  ip netns add test_root_ns

  ip netns exec test_root_ns ip link add enp175s0f1 type dummy
  ip netns exec test_root_ns ip link add enp175s6 type dummy
  ip netns exec test_root_ns ip link add enp175s7 type dummy
}


test_macaddress() {
  
  make_container "container_1"
  
  export DEFAULT_CNI_DIR=`mktemp -d`
  export CNI_CONTAINERID=container_1
  export CNI_PATH=${here}
  export CNI_NETNS=/run/netns/container_1_netns
  export CNI_IFNAME=net1
  
  read -r -d '' CNI_INPUT <<- EOM
  {
    "type": "sriov",
    "cniVersion": "0.3.1",
    "name": "sriov-network",
    "ipam": {
      "type": "test-ipam-cni"
    },
    "deviceID": "0000:af:06.0",
    "mac": "60:00:00:00:00:E1",
    "logLevel": "debug"
  }
EOM

  export CNI_COMMAND=ADD
  assert 'echo "$CNI_INPUT" | invoke_sriov_cni'
  assert 'ip netns exec container_1_netns ip link | grep -i 60:00:00:00:00:E1'

  export CNI_COMMAND=DEL
  assert 'echo "$CNI_INPUT" | invoke_sriov_cni'
  assert 'ip netns exec test_root_ns ip link show enp175s6'
}


test_vlan() {
  
  make_container "container_1"
  
  export DEFAULT_CNI_DIR=`mktemp -d`
  export CNI_CONTAINERID=container_1
  export CNI_PATH=${here}
  export CNI_NETNS=/run/netns/container_1_netns
  export CNI_IFNAME=net1
  
  read -r -d '' CNI_INPUT <<- EOM
  {
    "type": "sriov",
    "cniVersion": "0.3.1",
    "name": "sriov-network",
    "vlan": 1234,
    "ipam": {
      "type": "test-ipam-cni"
    },
    "deviceID": "0000:af:06.0",
    "mac": "60:00:00:00:00:E1",
    "logLevel": "debug"
  }
EOM

  export CNI_COMMAND=ADD
  assert 'echo "$CNI_INPUT" | invoke_sriov_cni'
  assert_file_contains ${DEFAULT_CNI_DIR}/enp175s0f1.calls "LinkSetVfVlanQosProto enp175s0f1 0 1234 0 33024"

  export CNI_COMMAND=DEL
  assert 'echo "$CNI_INPUT" | invoke_sriov_cni'
  assert 'ip netns exec test_root_ns ip link show enp175s6'
  assert_file_contains ${DEFAULT_CNI_DIR}/enp175s0f1.calls "LinkSetVfVlanQosProto enp175s0f1 0 0 0 33024"
}

test_multiple_invocations_on_same_vf() {
  make_container "container_1"
  make_container "container_2"
  
  export DEFAULT_CNI_DIR=`mktemp -d`
  
  export CNI_CONTAINERID=container_1
  export CNI_COMMAND=ADD
  export CNI_PATH=${here}
  export CNI_NETNS=/run/netns/container_1_netns
  export CNI_IFNAME=net1
  export IPAM_MOCK_SLEEP=3
  
  read -r -d '' CNI_INPUT <<- EOM
  {
    "type": "sriov",
    "cniVersion": "0.3.1",
    "name": "sriov-network",
    "ipam": {
      "type": "test-ipam-cni"
    },
    "deviceID": "0000:af:06.0",
    "mac": "60:00:00:00:00:E1",
    "logLevel": "debug"
  }
EOM

  assert 'echo "$CNI_INPUT" | invoke_sriov_cni'
  assert 'ip netns exec container_1_netns ip link | grep -i 60:00:00:00:00:E1'
}

invoke_sriov_cni() {
  ip netns exec test_root_ns go run ${here}/../../cmd/mocked-sriov/main.go
}

# Create a container and its relative network namespace. The first parameter is
# the name of the container, and as a convention, the netns name is `<container_name>_netns`
make_container() {
  container_name=$1
  ip netns del ${container_name}_netns || true 2>/dev/null
  ip netns add ${container_name}_netns

  podman kill ${container_name} >/dev/null 2>/dev/null
  podman rm -f ${container_name} >/dev/null 2>/dev/null
  assert "podman run -d --network ns:/run/netns/test_container_ns --name ${container_name} ${test_image} sleep inf"
}

assert_file_contains() {
  file=$1
  substr=$2
  if ! grep -q $substr $file; then
    fail "File [$file] does not contains [$substr], contents: \n `cat $file`"
  fi
}

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
