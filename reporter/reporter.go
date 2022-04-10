package reporter

import (
	"bytes"
	"flag"
	"log"
	"net"
	"time"

	"github.com/c0deaddict/arpwatch/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	interval   = flag.Duration("report-interval", 60*time.Second, "Report interval")
	offlineLag = flag.Duration("offline-lag", 30*time.Second, "Consider a host as offline after report-interval + offline-lag")

	knownHosts = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "arpwatch_known_hosts",
		Help: "The number of known hosts",
	})

	hostUp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "arpwatch_host_up",
		Help: "Indicates if the host was up or not",
	}, []string{"ip", "mac"})

	ch    = make(chan ping, 10)
	hosts = make(map[string]*hostInfo)
)

type ping struct {
	MAC net.HardwareAddr
	IP  net.IP
}

type state uint

const (
	STATE_NEW = iota
	STATE_ONLINE
	STATE_OFFLINE
)

// Maybe remove hosts after X days of being OFFLINE?
type hostInfo struct {
	mac   net.HardwareAddr
	seen  time.Time
	state state
}

func Start() {
	go run()
}

func Stop() {
	close(ch)
}

func Ping(mac net.HardwareAddr, ip net.IP) {
	ch <- ping{mac, ip}
}

func run() {
	timer := time.NewTicker(*interval)
	defer timer.Stop()

	for {
		select {
		case ping, ok := <-ch:
			if !ok {
				return
			}
			process(&ping)

		case <-timer.C:
			report()
		}
	}
}

func process(p *ping) {
	if p.MAC == nil {
		log.Printf("IP %v is alive", p.IP)
	} else {
		log.Printf("IP %v is at %v", p.IP, p.MAC)
	}

	ip := p.IP.String()
	if info, ok := hosts[ip]; ok {
		if p.MAC != nil {
			if info.mac != nil && bytes.Compare(p.MAC, info.mac) != 0 {
				log.Printf("%v changed from %v to %v", ip, info.mac, p.MAC)
			}
			info.mac = p.MAC

		}

		info.seen = time.Now()
	} else {
		hosts[ip] = &hostInfo{
			mac:   p.MAC,
			seen:  time.Now(),
			state: STATE_NEW,
		}
	}
}

func report() {
	now := time.Now()

	knownHosts.Set(float64(len(hosts)))

	for ip, info := range hosts {
		if info.state == STATE_NEW {
			log.Printf("New host discovered: %v", ip)
			info.state = STATE_ONLINE
			// presence.NewHost(ip, info.mac)
		}

		if now.Sub(info.seen) >= *interval+*offlineLag {
			log.Printf("IP %v (%v) not seen in last %v\n", ip, info.mac, *interval*2)
			info.state = STATE_OFFLINE
		} else if info.state == STATE_OFFLINE {
			log.Printf("IP %v (%v) is back!\n", ip, info.mac)
			info.state = STATE_ONLINE
		}

		if info.mac != nil {
			up := 0
			if info.state == STATE_ONLINE {
				up = 1
			}
			hostUp.WithLabelValues(ip, info.mac.String()).Set(float64(up))

			storage.WritePoint(ip, info.mac.String(), info.state == STATE_ONLINE)
		}

	}
}
