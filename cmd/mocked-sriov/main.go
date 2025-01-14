package main

import (
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/cnicommands"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/config"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
)

func main() {

	customCNIDir, ok :=os.LookupEnv("DEFAULT_CNI_DIR")
	if ok {
		config.DefaultCNIDir = customCNIDir
	}

	err := utils.CreateTmpSysFs()
	if err != nil {
		panic(err)
	}

	defer func() {
		err := utils.RemoveTmpSysFs()
		if err != nil {
			panic(err)
		}	
	}()

	cancel, err := utils.MockNetlinkLib(config.DefaultCNIDir)
	if err != nil {
		panic(err)
	}
	defer cancel()


	cniFuncs := skel.CNIFuncs{
		Add:   cnicommands.CmdAdd,
		Del:   cnicommands.CmdDel,
		Check: cnicommands.CmdCheck,
	}
	skel.PluginMainFuncs(cniFuncs, version.All, "")
}
