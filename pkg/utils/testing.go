/*
	This file contains test helper functions to mock linux sysfs directory.
	If a package need to access system sysfs it should call CreateTmpSysFs() before test
	then call RemoveTmpSysFs() once test is done for clean up.
*/

package utils

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
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

// FakeLink is a dummy netlink struct used during testing
type FakeLink struct {
	netlink.LinkAttrs
}

// type FakeLink struct {
// 	linkAtrrs *netlink.LinkAttrs
// }

func (l *FakeLink) Attrs() *netlink.LinkAttrs {
	return &l.LinkAttrs
}

func (l *FakeLink) Type() string {
	return "FakeLink"
}

func MockDevices() {
	fmt.Println("mocking devices")
	//mockedNetLink := &mocks_utils.NetlinkManager{}
	//netLinkLib = mockedNetLink
	//mockedNetLink.On("LinkByName", "enp175s6").Return(enp175s6, nil)
	//mockedNetLink.On("LinkSetDown", enp175s6).RunFn(nil)
	//mockedNetLink.On("LinkSetName", enp175s6).Return(nil)

/*
	enp175s0f1 := &FakeLink{LinkAttrs: netlink.LinkAttrs{
		Index: 1000,
		Name:  "enp175s0f1",
		//RawFlags: atomic.LoadUint32(rawFlagsAtomic),
		Vfs: []netlink.VfInfo{{
			ID:  0,
			Mac: mustParseMAC("ab:cd:ef:ab:cd:ef"),
		}},
	}}

	//mockedNetLink.On("LinkByName", "enp175s0f1").Return(enp175s0f1, nil)

	enp175s6 := &FakeLink{LinkAttrs: netlink.LinkAttrs{
		Index: 1001,
		Name:  "enp175s6",
	}}

	netLinkLib = &fakeLinkLib{
		links: []*FakeLink{
			enp175s0f1, enp175s6,
		},
	}
		*/

	netLinkLib = NewDummyPF()	
}

func mustParseMAC(x string) net.HardwareAddr {
	ret, err := net.ParseMAC(x)
	if err != nil {
		panic(err)
	}
	return ret
}




type fakeLinkLib struct {
	links []*FakeLink
}

func (n *fakeLinkLib) LinkByName(name string) (netlink.Link, error) {
	for _, l := range n.links {
		if l.Name == name {
			return l, nil
		}
	}

	return nil, fmt.Errorf("link %s not found", name)
}

func (n *fakeLinkLib) LinkSetVfVlanQosProto(_ netlink.Link, _ int, _ int, _ int, _ int) error {
	return nil
}

func (n *fakeLinkLib) LinkSetVfHardwareAddr(pfLink netlink.Link, vfIndex int, hwaddr net.HardwareAddr) error {
	pfLink.Attrs().Vfs[vfIndex].Mac = hwaddr
	return nil
}

func (n *fakeLinkLib) LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error {
	link.Attrs().HardwareAddr = hwaddr
	return nil
}

func (n *fakeLinkLib) LinkSetUp(_ netlink.Link) error {
	return nil
}

func (n *fakeLinkLib) LinkSetDown(_ netlink.Link) error {
	return nil
}

func (n *fakeLinkLib) LinkSetNsFd(_ netlink.Link, _ int) error {
	return nil
}

func (n *fakeLinkLib) LinkSetName(link netlink.Link, name string) error {
	link.Attrs().Name = name
	return nil
}

func (n *fakeLinkLib) LinkSetVfRate(pfLink netlink.Link, vfIndex int, minRate int, maxRate int) error {
	pfLink.Attrs().Vfs[vfIndex].MaxTxRate = uint32(maxRate)
	pfLink.Attrs().Vfs[vfIndex].MinTxRate = uint32(minRate)
	return nil
}

func (n *fakeLinkLib) LinkSetVfSpoofchk(pfLink netlink.Link, vfIndex int, spoofChk bool) error {
	pfLink.Attrs().Vfs[vfIndex].Spoofchk = spoofChk
	return nil
}

func (n *fakeLinkLib) LinkSetVfTrust(pfLink netlink.Link, vfIndex int, trust bool) error {
	if trust {
		pfLink.Attrs().Vfs[vfIndex].Trust = 1
	} else {
		pfLink.Attrs().Vfs[vfIndex].Trust = 0
	}
	
	return nil
}

func (n *fakeLinkLib) LinkSetVfState(pfLink netlink.Link, vfIndex int, state uint32) error {
	pfLink.Attrs().Vfs[vfIndex].LinkState = state
	return nil
}

func (n *fakeLinkLib) LinkDelAltName(link netlink.Link, name string) error {
	slices.DeleteFunc(link.Attrs().AltNames, func(s string) bool {
		return s == name
	})
	return nil
}



type dummyLinksLib struct {
	pf netlink.Link
	vfs map[int]netlink.Link
}

func NewDummyPF() *dummyLinksLib {
	ret := &dummyLinksLib{
		pf: &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{
				Index: 1000,
				Name:  "enp175s0f1",
		
				Vfs: []netlink.VfInfo{{
					ID:  0,
					Mac: mustParseMAC("ab:cd:ef:ab:cd:ef"),
				}},
			},
		},
		vfs: map[int]netlink.Link{
			0: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Index: 1001,
					Name:  "enp175s6",
				},
			},
		},
	}
	
	netns, err := ns.GetNS("/run/netns/test_root_ns")
	if err != nil {
		panic(err)
	}
	//defer netns.Close()
	
	err = netns.Set()
	if err != nil {
		panic(err)
	}

	err = netlink.LinkAdd(ret.pf)
	if err != nil {
		panic(err)
	}

	for _, vf := range ret.vfs {
		err = netlink.LinkAdd(vf)
		if err != nil {
			panic(err)
		}
	}
	
	return ret
}

func (n *dummyLinksLib) LinkByName(name string) (netlink.Link, error) {
	if name == n.pf.Attrs().Name {
		return n.pf, nil
	}
	return netlink.LinkByName(name)
}

func (n *dummyLinksLib) LinkSetVfVlanQosProto(link netlink.Link, vfIndex int, vlan int, vlanQos int, vlanProto int) error {
	panic("not implemented")
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
