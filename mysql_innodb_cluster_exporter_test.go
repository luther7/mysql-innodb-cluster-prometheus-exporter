// See: https://github.com/prometheus/mysqld_exporter/blob/master/mysqld_exporter_test.go

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"syscall"
	"testing"
	"time"
	"path"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

const (
	testConnectionString = "root:mysql@localhost:3306"
)

var (
	serverMetrics = map[string]prometheus.Gauge{
		"default_replica_set_status": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "default_replica_set_status",
				Help:      allMetrics["default_replica_set_status"],
			},
		),
	}
)

// bin stores information about path of executable and attached port
type bin struct {
	path string
	port int
}

// TestBin builds, runs and tests binary.
func TestBin(t *testing.T) {
	var err error
	binName := "mysqld_exporter"

	binDir, err := ioutil.TempDir("/tmp", binName+"-test-bindir-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := os.RemoveAll(binDir)
		if err != nil {
			t.Fatal(err)
		}
	}()

	path := binDir + "/" + binName
	cmd := exec.Command(
		"go",
		"build",
		"-o",
		path,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		t.Fatalf("Failed to build: %s", err)
	}

	tests := []func(*testing.T, bin){
		testLandingPage,
	}

	portStart := 56000
	t.Run(binName, func(t *testing.T) {
		for _, f := range tests {
			f := f // capture range variable
			fName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
			portStart++
			data := bin{
				path: path,
				port: portStart,
			}
			t.Run(fName, func(t *testing.T) {
				t.Parallel()
				f(t, data)
			})
		}
	})
}

func testLandingPage(t *testing.T, data bin) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Run exporter.
	cmd := exec.CommandContext(
		ctx,
		data.path,
		"--web.listen-address", fmt.Sprintf(":%d", data.port),
	)
	cmd.Env = append(os.Environ(), "DATA_SOURCE_NAME=127.0.0.1:3306")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()

	// Get the main page.
	urlToGet := fmt.Sprintf("http://127.0.0.1:%d", data.port)
	body, err := waitForBody(urlToGet)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)

	expected := `<html>
<head><title>MySQL InnoDB Cluster Exporter</title></head>
<body>
<h1>MySQL InnoDB Cluster Exporter</h1>
<p><a href='/metrics'>Metrics</a></p>
</body>
</html>`
	if got != expected {
		t.Fatalf("got '%s' but expected '%s'", got, expected)
	}
}

// waitForBody is a helper function which makes http calls until http server is up
// and then returns body of the successful call.
func waitForBody(urlToGet string) (body []byte, err error) {
	tries := 60

	// Get data, but we need to wait a bit for http server.
	for i := 0; i <= tries; i++ {
		// Try to get web page.
		body, err = getBody(urlToGet)
		if err == nil {
			return body, err
		}

		// If there is a syscall.ECONNREFUSED error (web server not available) then retry.
		if urlError, ok := err.(*url.Error); ok {
			if opError, ok := urlError.Err.(*net.OpError); ok {
				if osSyscallError, ok := opError.Err.(*os.SyscallError); ok {
					if osSyscallError.Err == syscall.ECONNREFUSED {
						time.Sleep(1 * time.Second)
						continue
					}
				}
			}
		}

		// There was an error, and it wasn't syscall.ECONNREFUSED.
		return nil, err
	}

	return nil, fmt.Errorf("failed to GET %s after %d tries: %s", urlToGet, tries, err)
}

// getBody is a helper function which retrieves http body from given address.
func getBody(urlToGet string) ([]byte, error) {
	resp, err := http.Get(urlToGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func expectMetrics(t *testing.T, c prometheus.Collector, fixture string) {
	exp, err := os.Open(path.Join("test", fixture))
	if err != nil {
		t.Fatalf("Error opening fixture file %q: %v", fixture, err)
	}
	if err := testutil.CollectAndCompare(c, exp); err != nil {
		t.Fatal("Unexpected metrics returned:", err)
	}
}

func testCommand(t *testing.T, bodyString string, fixture string) {
	body := []byte(bodyString)
	e, _ := NewExporter(testConnectionString, serverMetrics)
	parseCommand(e, body)
	expectMetrics(t, e, fixture)
}

func TestBadCommand(t *testing.T) {
	badString := "FOOBAR"
	testCommand(t, badString, "bad_command.metrics")
}

func TestGoodCommand(t *testing.T) {
	goodString := `
{"clusterName":"test","defaultReplicaSet":{"name":"default","primary":"server-1:3306","ssl":"REQUIRED","status":"OK","statusText":"Cluster is ONLINE and can tolerate up to ONE failure.","topology":{"server-1:3306":{"address":"server-1:3306","mode":"R/W","readReplicas":{},"role":"HA","status":"ONLINE","version":"8.0.12"},"server-2:3306":{"address":"server-2:3306","mode":"R/O","readReplicas":{},"role":"HA","status":"ONLINE","version":"8.0.12"},"server-3:3306":{"address":"server-3:3306","mode":"R/O","readReplicas":{},"role":"HA","status":"ONLINE","version":"8.0.12"}},"topologyMode":"Single-Primary"},"groupInformationSourceMember":"7d998e64a098:3306"}
`
	testCommand(t, goodString, "good_command.metrics")
}
