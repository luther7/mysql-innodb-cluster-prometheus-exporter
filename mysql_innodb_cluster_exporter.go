// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "mysql_innodb_cluster_exporter"
)

var (
	allMetrics = map[string]string{
		"server_up": "Current health of the server (1 = UP, 0 = DOWN).",
	}
)

type Exporter struct {
	connectionString string
	mutex            sync.RWMutex
	fetch            func() (io.ReadCloser, error)
	up               prometheus.Gauge
	totalScrapes     prometheus.Counter
	metrics          []prometheus.Gauge
}

func NewExporter(connectionString string, metrics []prometheus.Gauge) (*Exporter, error) {
	return &Exporter{
		connectionString: connectionString,
		fetch:            fetcher,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Was the last scrape of MySQL successful.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_total_scrapes",
			Help:      "Current total MySQL scrapes.",
		}),
		metrics: metrics,
	}, nil
}

func (exporter *Exporter) Describe(channel chan<- *prometheus.Desc) {
	for _, metric := range exporter.metrics {
		metric.Describe(channel)
	}
	channel <- exporter.up.Desc()
	channel <- exporter.totalScrapes.Desc()
}

func (exporter *Exporter) Collect(channel chan<- prometheus.Metric) {
	exporter.mutex.Lock() // To protect metrics from concurrent collects.
	defer exporter.mutex.Unlock()

	exporter.scrape()
	channel <- exporter.up
	channel <- exporter.totalScrapes
	for _, metric := range exporter.metrics {
		metric.Collect(channel)
	}
}

func fetcher(connectionString string, timeout time.Duration) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		//TODO
	}
}

func (exporter *Exporter) scrape() {
	exporter.totalScrapes.Inc()

	body, err := exporter.fetch()
	if err != nil {
		exporter.up.Set(0)
		log.Errorf("Can't scrape MySQL: %v", err)
		return
	}
	defer body.Close()
	exporter.up.Set(1)

	// TODO
}

func main() {
	var (
		listenAddress = kingpin.Flag(
			"web.listen-address",
			"Address to listen on for web interface and telemetry.",
		).Default(":9104").String()
		metricPath = kingpin.Flag(
			"web.telemetry-path",
			"Path under which to expose metrics.",
		).Default("/metrics").String()
		connectionString string
		metrics          = []prometheus.Gauge{}
	)

	for name, help := range allMetrics {
		enabled := kingpin.Flag("collect."+name, help).Default("true").Bool()
		if *enabled {
			metric := prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      name,
				Help:      help,
			})
			metrics = append(metrics, metric)
		}
	}

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("mysqld_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting mysql_innodb_cluster_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	connectionString = os.Getenv("MYSQL_CONNECTION_STRING")
	if len(connectionString) == 0 {
		// TODO also allow reading the data source from a file
		log.Fatal("MYSQL_CONNECTION_STRING not set")
	}

	exporter, err := NewExporter(connectionString, selectedMetrics)
	if err != nil {
		log.Fatal(err)
	}
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("mysqld_innodb_cluster_exporter"))

	log.Infoln("Listening on", *listenAddress)
	http.Handle(*metricPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>MySQL InnoDB Cluster Exporter</title></head>
             <body>
             <h1>MySQL InnoDB Cluster Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
