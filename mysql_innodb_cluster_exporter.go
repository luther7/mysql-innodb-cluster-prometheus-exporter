package main

import (
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/tidwall/gjson"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "mysql_innodb_cluster_exporter"
)

var (
	defaultConnectionString = "root:mysql@localhost:3306"
	allMetrics              = map[string]string{
		"default_replica_set_status": "Current health of the default replica set (1 = UP, 0 = DOWN).",
	}
)

type Exporter struct {
	connectionString string
	mutex            sync.RWMutex
	up               prometheus.Gauge
	totalScrapes     prometheus.Counter
	metrics          map[string]prometheus.Gauge
}

func NewExporter(connectionString string, metrics map[string]prometheus.Gauge) (*Exporter, error) {
	return &Exporter{
		connectionString: connectionString,
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
	exporter.mutex.Lock()
	defer exporter.mutex.Unlock()

	exporter.scrape()
	channel <- exporter.up
	channel <- exporter.totalScrapes
	for _, metric := range exporter.metrics {
		metric.Collect(channel)
	}
}

func runCommand(exporter *Exporter) ([]byte, error) {
	command := exec.Command(
		"mysqlsh",
		exporter.connectionString,
		"--interactive",
		"--js",
		"--json=raw",
		"--quiet-start=2",
		"--execute=dba.getCluster().status()",
	)
	return command.Output()
}

func parseCommand(exporter *Exporter, body []byte) (error) {
	if _, ok := exporter.metrics["default_replica_set_status"]; ok {
		upText := gjson.GetBytes(body, "defaultReplicaSet.status").String()
		var up float64 = 0
		if upText == "OK" {
			up = 1
		}
		exporter.metrics["default_replica_set_status"].Set(up)
	}
	return nil
}

func (exporter *Exporter) scrape() {
	exporter.totalScrapes.Inc()
	body, err := runCommand(exporter)
	if err != nil {
		exporter.up.Set(0)
		log.Errorf("Can't scrape MySQL: %v", err)

		return
	}
	exporter.up.Set(1)
	parseCommand(exporter, body)
}

func main() {
	var (
		listenAddress = kingpin.Flag(
			"web.listen-address",
			"Address to listen on for web interface and telemetry.",
		).Default(":9105").String()
		metricPath = kingpin.Flag(
			"web.telemetry-path",
			"Path under which to expose metrics.",
		).Default("/metrics").String()
		connectionString string
		metrics          = map[string]prometheus.Gauge{}
		selectedMetrics  = map[string]*bool{}
	)

	for name, help := range allMetrics {
		selectedMetrics[name] = kingpin.Flag("collect."+name, help).Default("true").Bool()
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
		log.Infoln("MYSQL_CONNECTION_STRING not set, using default", defaultConnectionString)
		connectionString = defaultConnectionString
	}

	for name, enabled := range selectedMetrics {
		if *enabled {
			metrics[name] = prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      name,
				Help:      allMetrics[name],
			})
		}
	}

	exporter, err := NewExporter(connectionString, metrics)
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
<p><a href='` + *metricPath + `'>Metrics</a></p>
</body>
</html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
