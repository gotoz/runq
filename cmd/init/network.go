package main

import (
	"fmt"
	"log"
	"net"

	"github.com/gotoz/runq/pkg/vm"

	"github.com/vishvananda/netlink"
)

func setupNetwork(networks []vm.Network) error {
	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("netlink.LinkList failed: %w", err)
	}

	if len(networks) != len(links)-1 {
		return fmt.Errorf("mismatch vmdata <-> links")
	}

	// There is no guarantee that the order of the VM eth interfaces
	// is the same as the veth endpoints in the original namespace.
	// Therefore rename all interfaces to temporary names
	// and change back to the original names later on.
	if len(networks) > 1 {
		for i, link := range links {
			if link.Attrs().Name == "lo" {
				continue
			}
			name := fmt.Sprintf("tmp-%02d", i)
			if err := netlink.LinkSetName(link, name); err != nil {
				return fmt.Errorf("netlink.LinkSetName failed, name:%s : %w", name, err)
			}
		}
	}

	for _, link := range links {
		var err error
		attr := link.Attrs()

		if attr.Name == "lo" {
			if err = netlink.LinkSetUp(link); err != nil {
				return fmt.Errorf("netlink.LinkSetUp failed, link:%s : %w", attr.Name, err)
			}
			continue
		}

		// Get config data.
		var nw vm.Network
		for _, n := range networks {
			if n.MacAddress == attr.HardwareAddr.String() {
				nw = n
			}
		}
		if nw.Name == "" {
			return fmt.Errorf("no vm data for link %s", attr.Name)
		}

		// Rename links back to ethX names.
		if len(networks) > 1 {
			if err = netlink.LinkSetName(link, nw.Name); err != nil {
				return fmt.Errorf("netlink LinkSetName %s failed: %w", nw.Name, err)
			}
		}

		// Add IP addresses.
		for _, addr := range nw.Addrs {
			addr := addr
			addr.Label = ""
			err = netlink.AddrAdd(link, &addr)
			if err != nil {
				return fmt.Errorf("netlink.AddrAdd failed, link:%v addr:%v : %w", attr.Name, addr, err)
			}
		}

		err = netlink.LinkSetMTU(link, nw.MTU)
		if err != nil {
			return fmt.Errorf("netlink.LinkSetMTU failed, link:%s : %w", attr.Name, err)
		}

		err = netlink.LinkSetUp(link)
		if err != nil {
			return fmt.Errorf("netlink.LinkSetUp failed, link:%s : %w", attr.Name, err)
		}

		// Add default gateway.
		// TODO: need to handle other routing entries?
		if nw.Gateway != nil {
			route := netlink.Route{
				Src: nil,
				Gw:  nw.Gateway,
			}
			if err := netlink.RouteAdd(&route); err != nil {
				return fmt.Errorf("netlink.RouteAdd failed, route:%v : %w", route, err)
			}

			// Send an arp request to trigger bridge setup.
			if conn, err := net.Dial("udp", nw.Gateway.String()+":0"); err != nil {
				log.Println(err)
			} else {
				conn.Write([]byte{})
				conn.Close()
			}
		}
	}
	return nil
}
