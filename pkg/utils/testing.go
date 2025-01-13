/*
	This file contains test helper functions to mock linux sysfs directory.
	If a package need to access system sysfs it should call CreateTmpSysFs() before test
	then call RemoveTmpSysFs() once test is done for clean up.
*/

package utils

import (
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/vishvananda/netlink"
)

type tmpSysFs struct {
	dirRoot      string
	dirList      []string
	fileList     map[string][]byte
	netSymlinks  map[string]string
	devSymlinks  map[string]string
	vfSymlinks   map[string]string
	originalRoot *os.File
}

var ts = tmpSysFs{
	dirList: []string{
		"sys/class/net",
		"sys/bus/pci/devices",
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1/net/enp175s0f1",
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0/net/enp175s6",
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1/net/enp175s7",
		"sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0/net/ens1",
		"sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0/net/ens1d1",
	},
	fileList: map[string][]byte{
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1/sriov_numvfs": []byte("2"),
		"sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0/sriov_numvfs": []byte("0"),
	},
	netSymlinks: map[string]string{
		"sys/class/net/enp175s0f1": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1/net/enp175s0f1",
		"sys/class/net/enp175s6":   "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0/net/enp175s6",
		"sys/class/net/enp175s7":   "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1/net/enp175s7",
		"sys/class/net/ens1":       "sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0/net/ens1",
		"sys/class/net/ens1d1":     "sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0/net/ens1d1",
	},
	devSymlinks: map[string]string{
		"sys/class/net/enp175s0f1/device": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1",
		"sys/class/net/enp175s6/device":   "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0",
		"sys/class/net/enp175s7/device":   "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1",
		"sys/class/net/ens1/device":       "sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0",
		"sys/class/net/ens1d1/device":     "sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0",

		"sys/bus/pci/devices/0000:af:00.1": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1",
		"sys/bus/pci/devices/0000:af:06.0": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0",
		"sys/bus/pci/devices/0000:af:06.1": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1",
		"sys/bus/pci/devices/0000:05:00.0": "sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0",
	},
	vfSymlinks: map[string]string{
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1/virtfn0": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0",
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0/physfn":  "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1",

		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1/virtfn1": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1",
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1/physfn":  "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1",
	},
}

// CreateTmpSysFs create mock sysfs for testing
func CreateTmpSysFs() error {
	originalRoot, _ := os.Open("/")
	ts.originalRoot = originalRoot

	tmpdir, err := os.MkdirTemp("/tmp", "sriovplugin-testfiles-")
	if err != nil {
		return err
	}

	ts.dirRoot = tmpdir
	//syscall.Chroot(ts.dirRoot)

	for _, dir := range ts.dirList {
		if err := os.MkdirAll(filepath.Join(ts.dirRoot, dir), 0755); err != nil {
			return err
		}
	}

	for filename, body := range ts.fileList {
		if err := os.WriteFile(filepath.Join(ts.dirRoot, filename), body, 0600); err != nil {
			return err
		}
	}

	for link, target := range ts.netSymlinks {
		if err := createSymlinks(filepath.Join(ts.dirRoot, link), filepath.Join(ts.dirRoot, target)); err != nil {
			return err
		}
	}

	for link, target := range ts.devSymlinks {
		if err := createSymlinks(filepath.Join(ts.dirRoot, link), filepath.Join(ts.dirRoot, target)); err != nil {
			return err
		}
	}

	for link, target := range ts.vfSymlinks {
		if err := createSymlinks(filepath.Join(ts.dirRoot, link), filepath.Join(ts.dirRoot, target)); err != nil {
			return err
		}
	}

	SysBusPci = filepath.Join(ts.dirRoot, SysBusPci)
	NetDirectory = filepath.Join(ts.dirRoot, NetDirectory)
	return nil
}

func createSymlinks(link, target string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	return os.Symlink(target, link)
}

// RemoveTmpSysFs removes mocked sysfs
func RemoveTmpSysFs() error {
	err := ts.originalRoot.Chdir()
	if err != nil {
		return err
	}
	if err = syscall.Chroot("."); err != nil {
		return err
	}
	if err = ts.originalRoot.Close(); err != nil {
		return err
	}

	return os.RemoveAll(ts.dirRoot)
}

func MockNetlinkLib() (func(), error) {
	var err error 
	oldnetlinkLib := netLinkLib 
	netLinkLib, err = NewDummyPF("enp175s0f1", []string{"enp175s6", "enp175s7"})

	return func() {
		netLinkLib = oldnetlinkLib
	}, err
}

// dummyLinksLib creates dummy interfaces for Physical and Virtual function, interceptin calls to netlink library
type dummyLinksLib struct {
	pf netlink.Link
	vfs map[int]*netlink.Dummy
}

func NewDummyPF(pfName string, vfNames []string) (*dummyLinksLib, error) {
	ret := &dummyLinksLib{
		pf: &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{
				Name:  pfName,
		
				Vfs: []netlink.VfInfo{{
					ID:  0,
					Mac: mustParseMAC("ab:cd:ef:ab:cd:ef"),
				}},
			},
		},
		vfs: map[int]*netlink.Dummy{},
	}

	for i, vfName := range vfNames {
		ret.vfs[i] = &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name:  vfName,
				},
			}

	}
	
	return ret, nil
}

func (n *dummyLinksLib) LinkByName(name string) (netlink.Link, error) {
	if name == n.pf.Attrs().Name {
		return n.pf, nil
	}
	return netlink.LinkByName(name)
}

func (n *dummyLinksLib) LinkSetVfVlanQosProto(link netlink.Link, vfIndex int, vlan int, vlanQos int, vlanProto int) error {
	//panic("not implemented")
	//n.vfs[vfIndex].VlanId = vlan
	//n.vfs[vfIndex].VlanProtocol = netlink.VlanProtocol(vlanProto)
	return netlink.LinkModify(n.vfs[vfIndex])
}

func (n *dummyLinksLib) LinkSetVfHardwareAddr(pfLink netlink.Link, vfIndex int, hwaddr net.HardwareAddr) error {
	pfLink.Attrs().Vfs[vfIndex].Mac = hwaddr
	return nil
}

func (n *dummyLinksLib) LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error {
	return netlink.LinkSetHardwareAddr(link, hwaddr)
}

func (n *dummyLinksLib) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

func (n *dummyLinksLib) LinkSetDown(link netlink.Link) error {
	return netlink.LinkSetDown(link)
}

func (n *dummyLinksLib) LinkSetNsFd(link netlink.Link, nsFd int) error {
	return netlink.LinkSetNsFd(link, nsFd)
}

func (n *dummyLinksLib) LinkSetName(link netlink.Link, name string) error {
	return netlink.LinkSetName(link, name)
}

func (n *dummyLinksLib) LinkSetVfRate(pfLink netlink.Link, vfIndex int, minRate int, maxRate int) error {
	pfLink.Attrs().Vfs[vfIndex].MaxTxRate = uint32(maxRate)
	pfLink.Attrs().Vfs[vfIndex].MinTxRate = uint32(minRate)
	return nil
}

func (n *dummyLinksLib) LinkSetVfSpoofchk(pfLink netlink.Link, vfIndex int, spoofChk bool) error {
	pfLink.Attrs().Vfs[vfIndex].Spoofchk = spoofChk
	return nil
}

func (n *dummyLinksLib) LinkSetVfTrust(pfLink netlink.Link, vfIndex int, trust bool) error {
	if trust {
		pfLink.Attrs().Vfs[vfIndex].Trust = 1
	} else {
		pfLink.Attrs().Vfs[vfIndex].Trust = 0
	}
	
	return nil
}

func (n *dummyLinksLib) LinkSetVfState(pfLink netlink.Link, vfIndex int, state uint32) error {
	pfLink.Attrs().Vfs[vfIndex].LinkState = state
	return nil
}

func (n *dummyLinksLib) LinkDelAltName(link netlink.Link, name string) error {
	return netlink.LinkDelAltName(link, name)
}

func mustParseMAC(x string) net.HardwareAddr {
	ret, err := net.ParseMAC(x)
	if err != nil {
		panic(err)
	}
	return ret
}
