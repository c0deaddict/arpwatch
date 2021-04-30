package watcher

import (
	"bytes"
	"net"

	"github.com/c0deaddict/arpwatch/reporter"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

type Watcher struct {
	iface   net.Interface
	network net.IPNet
	report  chan<- reporter.Ping
}

func New(iface net.Interface, network net.IPNet, report chan<- reporter.Ping) *Watcher {
	return &Watcher{iface, network, report}
}

func (w *Watcher) Start(stop <-chan struct{}) error {
	// Open up a pcap handle for packet reads/writes.
	handle, err := pcap.OpenLive(w.iface.Name, 65536, true, pcap.BlockForever)
	if err != nil {
		return err
	}

	// We're only interested in ARP packets.
	err = handle.SetBPFFilter("arp")
	if err != nil {
		handle.Close()
		return err
	}

	go func() {
		defer handle.Close()
		w.loop(handle, stop)
	}()

	return nil
}

func (w *Watcher) loop(handle *pcap.Handle, stop <-chan struct{}) {
	src := gopacket.NewPacketSource(handle, layers.LayerTypeEthernet)
	in := src.Packets()
	for {
		var packet gopacket.Packet
		select {
		case <-stop:
			return
		case packet = <-in:
			arpLayer := packet.Layer(layers.LayerTypeARP)
			if arpLayer == nil {
				continue
			}
			arp := arpLayer.(*layers.ARP)
			if arp.Operation == layers.ARPRequest && bytes.Equal([]byte(w.iface.HardwareAddr), arp.SourceHwAddress) {
				// This is a packet I sent.
				continue
			}

			w.report <- reporter.Ping{
				IP:  net.IP(arp.SourceProtAddress),
				MAC: net.HardwareAddr(arp.SourceHwAddress),
			}
		}
	}
}
