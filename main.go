package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/c0deaddict/arpwatch/metrics"
	"github.com/c0deaddict/arpwatch/pinger"
	"github.com/c0deaddict/arpwatch/reporter"
	"github.com/c0deaddict/arpwatch/storage"
	"github.com/c0deaddict/arpwatch/utils"
	"github.com/c0deaddict/arpwatch/watcher"
)

// presence for mqtt

var (
	ifaceNames = utils.StringSliceFlag("iface", []string{}, "interfaces to watch")
	interval   = flag.Duration("interval", 60*time.Second, "interval")

	ifaces   []net.Interface
	networks []*net.IPNet
)

func main() {
	flag.Parse()

	if err := findInterfaces(); err != nil {
		log.Fatalln(err)
	}

	go metrics.Run()

	if err := storage.Connect(); err != nil {
		log.Fatalln(err)
	}
	defer storage.Close()

	// Set up and start the reporter.
	ch := make(chan reporter.Ping, 10)
	defer close(ch)

	reporter := reporter.New(*interval, ch)
	go reporter.Run()

	stop := make(chan struct{})
	defer close(stop)

	var wg sync.WaitGroup
	for i, iface := range ifaces {
		// Start a Pinger on each interface.
		p := pinger.New(pinger.PingerOpts{
			Iface:    iface,
			Network:  *networks[i],
			Report:   ch,
			Interval: *interval,
		})
		wg.Add(1)
		go func(iface net.Interface) {
			defer wg.Done()
			err := p.Run(stop)
			if err != nil {
				log.Printf("Pinger on interface %v crashed: %v", iface.Name, err)
			}
		}(iface)

		// Start up a watcher on each interface.
		w := watcher.New(iface, *networks[i], ch)
		if err := w.Start(stop); err != nil {
			log.Fatalln(err)
		}
	}

	wg.Wait()
}

func findInterfaces() error {
	if len(*ifaceNames) == 0 {
		return errors.New("At least one interface is required")
	}

	// Get a list of all interfaces.
	allIfaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	// Filter on the requested interfaces.
	for _, iface := range allIfaces {
		found := false
		for i, name := range *ifaceNames {
			if iface.Name == name {
				*ifaceNames = append((*ifaceNames)[:i], (*ifaceNames)[i+1:]...)
				found = true
				break
			}
		}
		if found {
			ifaces = append(ifaces, iface)
		}
	}

	if len(*ifaceNames) != 0 {
		return fmt.Errorf("Interfaces not found: %s", strings.Join(*ifaceNames, ", "))
	}

	networks = make([]*net.IPNet, len(ifaces))
	for i, iface := range ifaces {
		// Determine the IPv4 network of the interface.  This only uses
		// the first address found, an interface could have multiple
		// IPv4 networks.
		network, err := utils.FirstIPv4Network(&iface)
		if err != nil {
			log.Fatalf("Could not determine IPv4 network for interface %v: %v", iface.Name, err)
		} else if network.IP[0] == 127 {
			log.Fatalf("Interface %v has local 127.x.x.x network", iface.Name)
		} else if network.Mask[0] != 0xff || network.Mask[1] != 0xff {
			log.Fatalf("Network %v is too large for interface %v", network, iface.Name)
		}

		log.Printf("Using network range %v for interface %v", network, iface.Name)
		networks[i] = network
	}

	return nil
}
