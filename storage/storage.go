package storage

import (
	"context"
	"flag"
	"log"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	serverUrl = flag.String("influxdb-url", "http://localhost:8086", "InfluxDB server url")
	token     = flag.String("influxdb-token", "", "InfluxDB authentication token")
	org       = flag.String("influxdb-org", "", "InfluxDB organization")
	bucket    = flag.String("influxdb-bucket", "arpwatch", "InfluxDB bucket")

	writeErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "arpwatch_influxdb_write_errors_total",
		Help: "The total number of Influxdb write errors",
	})

	client   influxdb2.Client
	writeAPI api.WriteAPI
)

func Connect() error {
	client = influxdb2.NewClient(*serverUrl, *token)

	_, err := client.Health(context.Background())
	if err != nil {
		client.Close()
		return err
	}

	writeAPI = client.WriteAPI(*org, *bucket)

	errorsCh := writeAPI.Errors()
	go func() {
		// TODO: fail entire application if write errors persist for X minutes.
		for err := range errorsCh {
			log.Printf("Influxdb write error: %v", err)
			writeErrors.Inc()
		}
	}()

	return nil
}

func Close() {
	writeAPI.Flush()
	client.Close()
}

func WritePoint(ip string, mac string, online bool) {
	p := influxdb2.NewPoint(
		"host",
		map[string]string{
			"ip":  ip,
			"mac": mac,
		},
		map[string]interface{}{
			"online": online,
		},
		time.Now(),
	)

	// Writes are asynchronous.
	// By default batched per 5000 and flushed every 1s.
	writeAPI.WritePoint(p)
}
