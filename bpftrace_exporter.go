// based on haproxy_exporter
// https://github.com/prometheus/haproxy_exporter
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/andreasgerstmayr/bpftrace_exporter/pkg/exporter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
)

func main() {
	var (
		addr         = flag.String("listen-address", ":9928", "The address to listen on for HTTP requests.")
		bpftracePath = flag.String("bpftrace", "bpftrace", "Path to the bpftrace executable.")
		scriptPath   = flag.String("script", "", "Path to the bpftrace script.")
		vars         = flag.String("vars", "", "bpftrace variables to export")
	)
	flag.Parse()

	exporter, err := exporter.NewExporter(*bpftracePath, *scriptPath, *vars)
	if err != nil {
		log.Fatalln("Error creating an exporter", err)
		os.Exit(1)
	}

	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("bpftrace_exporter"))

	http.Handle("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
			Timeout:           5 * time.Second,
		},
	))
	log.Fatal(http.ListenAndServe(*addr, nil))
}
