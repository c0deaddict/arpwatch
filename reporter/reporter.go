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
	offlineLag = flag.Duration("offline-lag", 30*time.Second, "Consider a host as offline after interval + offline-lag")

	knownHosts = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "arpwatch_known_hosts",
		Help: "The number of known hosts",
	})

	hostUp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "arpwatch_host_up",
		Help: "Indicates if the host was up or not",
	}, []string{"ip", "mac"})
)

type Ping struct {
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

type Reporter struct {
	interval time.Duration
	hosts    map[string]*hostInfo
	ch       <-chan Ping
}

func New(interval time.Duration, ch <-chan Ping) *Reporter {
	return &Reporter{
		interval: interval,
		hosts:    make(map[string]*hostInfo),
		ch:       ch,
	}
}

func (r *Reporter) Run() {
	timer := time.NewTicker(r.interval)
	defer timer.Stop()

	for {
		select {
		case ping, ok := <-r.ch:
			if !ok {
				return
			}
			r.process(&ping)

		case <-timer.C:
			r.report()
		}
	}
}

func (r *Reporter) process(p *Ping) {
	if p.MAC == nil {
		log.Printf("IP %v is alive", p.IP)
	} else {
		log.Printf("IP %v is at %v", p.IP, p.MAC)
	}

	ip := p.IP.String()
	if info, ok := r.hosts[ip]; ok {
		if p.MAC != nil {
			if info.mac != nil && bytes.Compare(p.MAC, info.mac) != 0 {
				log.Printf("%v changed from %v to %v", ip, info.mac, p.MAC)
			}
			info.mac = p.MAC

		}

		info.seen = time.Now()
	} else {
		r.hosts[ip] = &hostInfo{
			mac:   p.MAC,
			seen:  time.Now(),
			state: STATE_NEW,
		}
	}
}

func (r *Reporter) report() {
	now := time.Now()

	knownHosts.Set(float64(len(r.hosts)))

	for ip, info := range r.hosts {
		if info.state == STATE_NEW {
			log.Printf("New host discovered: %v", ip)
			info.state = STATE_ONLINE
		}

		if now.Sub(info.seen) >= r.interval+*offlineLag {
			log.Printf("IP %v (%v) not seen in last %v\n", ip, info.mac, r.interval*2)
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
