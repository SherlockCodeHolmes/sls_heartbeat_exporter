package main

import (
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"sls_heartbeat_exporter/collector"
)

var (
	exporterConfig = flag.String("config", "./sls_heartbeat_exporter.yml", "Path to the cloud configuration file")
)

func handler(w http.ResponseWriter, r *http.Request) {
	registry := prometheus.NewRegistry()
	exporter := collector.NewSlsExporter(*exporterConfig)
	registry.MustRegister(exporter)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func main() {
	flag.Parse()
	config, err := collector.InitExporterConf(*exporterConfig)
	if err != nil {
		log.Printf("New Cloud Config From File error: %s", err.Error())
		return
	}
	http.HandleFunc("/metrics", handler)

	log.Printf("Start server on port %s", config.Global.Port)

	if err := http.ListenAndServe(config.Global.Port, nil); err != nil {
		log.Printf("Error occur when start server: %s", err.Error())
		os.Exit(1)
	}
}
