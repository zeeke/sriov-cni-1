#!/bin/bash

test_image="docker.io/library/busybox:1.36"
here="$(dirname "$(readlink --canonicalize "${BASH_SOURCE[0]}")")"
export CNI_PATH=${here}

setup() {
  ip netns del test_root_ns || true
  ip netns add test_root_ns

  ip netns exec test_root_ns ip link add enp175s0f1 type dummy
  ip netns exec test_root_ns ip link add enp175s6 type dummy
  ip netns exec test_root_ns ip link add enp175s7 type dummy

  export DEFAULT_CNI_DIR=`mktemp -d`
}


test_macaddress() {
  
  make_container "container_1"
  
  export CNI_CONTAINERID=container_1
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
  
  export CNI_CONTAINERID=container_1
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

# This test simulates a heavy load on the IPAM plugin, which takes a long time to finish. In this case, 
# the Kubelet can decide to remove the container with its network namespace while a CNI DEL command is still running.
test_long_running_ipam() {

  make_container "container_1"
  export CNI_CONTAINERID=container_1
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
    "vlan": 0,
    "logLevel": "debug",
    "logFile": "${DEFAULT_CNI_DIR}/sriov.log"
  }
EOM

  export CNI_COMMAND=ADD
  assert 'echo "$CNI_INPUT" | invoke_sriov_cni'

  # Start a long live CNI delete
  echo "$CNI_INPUT" | IPAM_MOCK_SLEEP=3 CNI_COMMAND=DEL invoke_sriov_cni &

  # Simulate the kubelet deleting the container and the network namespace after a timeout
  # The VF goes back to the root network namespace
  sleep 1
  ip netns exec container_1_netns ip link set net1 netns test_root_ns name enp175s6
  delete_container container_1

  # Spawn a new container that tries to use the same device
  make_container "container_2"
  export CNI_CONTAINERID=container_2
  export CNI_NETNS=/run/netns/container_2_netns
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
    "logLevel": "debug",
    "logFile": "${DEFAULT_CNI_DIR}/sriov.log"
  }
EOM

  export CNI_COMMAND=ADD
  assert 'echo "$CNI_INPUT" | invoke_sriov_cni'
  assert_file_contains "${DEFAULT_CNI_DIR}/enp175s0f1.calls" "LinkSetVfVlanQosProto enp175s0f1 0 1234 0 33024"


  wait
  expected_vlan_set_calls=$(cat <<EOM
LinkSetVfVlanQosProto enp175s0f1 0 0 0 33024
LinkSetVfVlanQosProto enp175s0f1 0 0 0 33024
LinkSetVfVlanQosProto enp175s0f1 0 1234 0 33024
EOM
)

  assert_equals "$expected_vlan_set_calls" "`grep LinkSetVfVlanQosProto ${DEFAULT_CNI_DIR}/enp175s0f1.calls`"
}

invoke_sriov_cni() {
  ip netns exec test_root_ns go run ${here}/../../cmd/mocked-sriov/main.go
}

# Create a container and its related network namespace. The first parameter is
# the name of the container, and as a convention, the netns name is `<container_name>_netns`
make_container() {
  container_name=$1
  delete_container $container_name

  ip netns add ${container_name}_netns
  assert "podman run -d --network ns:/run/netns/test_container_ns --name ${container_name} ${test_image} sleep inf"
}

delete_container() {
  container_name=$1
  ip netns del ${container_name}_netns 2>/dev/null

  podman kill ${container_name} >/dev/null 2>/dev/null
  podman rm -f ${container_name} >/dev/null 2>/dev/null
}

assert_file_contains() {
  file=$1
  substr=$2
  if ! grep -q "$substr" $file; then
    fail "File [$file] does not contains [$substr], contents: \n `cat $file`"
  fi
}


# {"MAC":"","MTU":1500,"Master":"enp175s0f1",
#     "OrigVfState":{"AdminMAC":"ab:cd:ef:ab:cd:ef","EffectiveMAC":"c2:c3:9b:3f:e9:31","HostIFName":"enp175s6","LinkState":0,"MaxTxRate":0,"MinTxRate":0,"SpoofChk":false,"Trust":false,"Vlan":0,"VlanProto":0,"VlanQoS":0},
#     "VFID":0,"cniVersion":"0.3.1","deviceID":"0000:af:06.0","ipam":{"type":"test-ipam-cni"},"logFile":"/tmp/tmp.KJ8JEEi4Uu/test_long_running_ipam.log","logLevel":"debug","max_tx_rate":null,"min_tx_rate":null,"name":"sriov-network","runtimeConfig":{},"type":"sriov",
#     "vlan":null,"vlanProto":null,"vlanQoS":null}