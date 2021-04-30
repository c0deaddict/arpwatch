package pinger

import (
	"errors"
	"log"
	"net"
	"os"
	"time"

	"github.com/c0deaddict/arpwatch/reporter"
	"github.com/c0deaddict/arpwatch/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var (
	pingerSendDuration = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "arpwatch_pinger_send_duration_seconds",
		Help:       "The number of seconds sending out pings to the network took",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"iface"})
)

type PingerOpts struct {
	Iface    net.Interface
	Network  net.IPNet
	Report   chan<- reporter.Ping
	Interval time.Duration
}

type Pinger struct {
	opts PingerOpts
}

func New(opts PingerOpts) *Pinger {
	return &Pinger{opts}
}

func (p *Pinger) Run(stop <-chan struct{}) error {
	c, err := icmp.ListenPacket("ip4:icmp", p.opts.Network.IP.String())
	if err != nil {
		return err
	}

	log.Printf("Pinger listening for ICMP on %v", p.opts.Network.IP)

	done := make(chan error)

	go func() {
		done <- p.sender(c, stop)
	}()

	if err := p.receiver(c); err != nil {
		// Stop sender by closing connection.
		c.Close()
		// Wait for sender to stop.
		<-done
		return err
	}

	return <-done
}

func (p *Pinger) sender(c *icmp.PacketConn, stop <-chan struct{}) error {
	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte(""),
		},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		c.Close()
		return err
	}

	for {
		start := time.Now()
		for _, addr := range utils.EnumerateIPs(&p.opts.Network) {
			if _, err := c.WriteTo(wb, &net.IPAddr{IP: addr}); err != nil {
				return err
			}
		}
		elapsed := time.Since(start)

		pingerSendDuration.WithLabelValues(p.opts.Iface.Name).Observe(elapsed.Seconds())

		sleep := 1*p.opts.Interval - elapsed
		if sleep < 0 {
			log.Printf("Warning: Pinger is sending slower than interval")
			sleep = 100 * time.Millisecond
		}

		select {
		case <-stop:
			c.Close()
			return nil
		case <-time.After(sleep):
		}
	}
}

func (p *Pinger) receiver(c *icmp.PacketConn) error {
	rb := make([]byte, 1500)

	for {
		n, peer, err := c.ReadFrom(rb)
		if errors.Is(err, net.ErrClosed) {
			// This is expected if we c.Close() from sender()
			return nil
		} else if err != nil {
			return err
		}

		rm, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), rb[:n])
		if err != nil {
			log.Printf("Failed to parse icmp message: %v", err)
		}
		switch rm.Type {
		case ipv4.ICMPTypeEchoReply:
			ip := peer.(*net.IPAddr).IP
			p.opts.Report <- reporter.Ping{IP: ip}
		default:
			// Ignore.
		}
	}
}
