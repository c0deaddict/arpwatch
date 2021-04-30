package metrics

import (
	"log"
	"flag"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var exporterListen = flag.String("exporter-listen", ":9999", "Prometheus exporter listen address")

func Run() {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*exporterListen, nil))
}
