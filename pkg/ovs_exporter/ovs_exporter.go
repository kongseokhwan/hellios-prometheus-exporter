// Copyright 2018 Paul Greenberg (greenpau@outlook.com)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ovs_exporter

import (
	//"github.com/davecgh/go-spew/spew"
	"fmt"
	_ "net/http/pprof"
	"sync"
	"sync/atomic"
	"time"

	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

const (
	namespace = "ovs"
)

var (
	appName    = "ovs-exporter"
	appVersion = "[untracked]"
	gitBranch  string
	gitCommit  string
	buildUser  string // whoami
	buildDate  string // date -u
)

var (
	InterfaceRxBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
		"Number of bytes received on a network interface, in bytes.",
		[]string{"port"},
		nil)
	InterfaceRxPacketsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "interface", "receive_packets_total"),
		"Number of packets received on a network interface, in bytes.",
		[]string{"port"},
		nil)
	InterfaceTxBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "interface", "transmit_bytes_total"),
		"Number of bytes sent on a network interface, in bytes.",
		[]string{"port"},
		nil)
	InterfaceTxPacketsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "interface", "transmit_packets_total"),
		"Number of packets sent on a network interface, in bytes.",
		[]string{"port"},
		nil)
)

// Exporter collects ovs data from the given server and exports them using
// the prometheus metrics package.
type Exporter struct {
	sync.RWMutex
	Client               *ovs.Client
	timeout              int
	pollInterval         int64
	errors               int64
	errorsLocker         sync.RWMutex
	nextCollectionTicker int64
	metrics              []prometheus.Metric
}

type Options struct {
	Timeout int
}

// NewExporter returns an initialized Exporter.
func NewExporter(opts Options) (*Exporter, error) {
	version.Version = appVersion
	version.Revision = gitCommit
	version.Branch = gitBranch
	version.BuildUser = buildUser
	version.BuildDate = buildDate
	e := Exporter{
		timeout: opts.Timeout,
	}
	// TODO : Change to Hellios Client
	client := ovs.New()
	e.Client = client
	bridges, err := e.Client.VSwitch.ListBridges()
	if err != nil {
		log.Error(err)
	}
	log.Debugf("%s: NewExporter() calls ListBridges()", bridges)
	log.Debug("NewExporter() initialized successfully")
	return &e, nil
}

// Describe describes all the metrics ever exported by the ovs exporter. It
// implements prometheus.Collector.
// TODO #2: Mapping Structure to prometheus channel
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- InterfaceRxBytesDesc
	ch <- InterfaceRxPacketsDesc
	ch <- InterfaceTxBytesDesc
	ch <- InterfaceTxPacketsDesc
}

// IncrementErrorCounter increases the counter of failed queries
func (e *Exporter) IncrementErrorCounter() {
	e.errorsLocker.Lock()
	defer e.errorsLocker.Unlock()
	atomic.AddInt64(&e.errors, 1)
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.GatherMetrics()
	log.Debugf("%s: Collect() calls RLock()", e.Client.System.ID)
	e.RLock()
	defer e.RUnlock()
	if len(e.metrics) == 0 {
		log.Debugf("%s: Collect() no metrics found", e.Client.System.ID)
		ch <- prometheus.MustNewConstMetric(
			up,
			prometheus.GaugeValue,
			0,
		)
		ch <- prometheus.MustNewConstMetric(
			info,
			prometheus.GaugeValue,
			1,
			e.Client.System.ID, e.Client.System.RunDir, e.Client.System.Hostname,
			e.Client.System.Type, e.Client.System.Version,
			e.Client.Database.Vswitch.Version, e.Client.Database.Vswitch.Schema.Version,
		)
		ch <- prometheus.MustNewConstMetric(
			requestErrors,
			prometheus.CounterValue,
			float64(e.errors),
			e.Client.System.ID,
		)
		ch <- prometheus.MustNewConstMetric(
			nextPoll,
			prometheus.CounterValue,
			float64(e.nextCollectionTicker),
			e.Client.System.ID,
		)
		return
	}
	log.Debugf("%s: Collect() sends %d metrics to a shared channel", e.Client.System.ID, len(e.metrics))
	for _, m := range e.metrics {
		ch <- m
	}
}

// GatherMetrics collect data from ovs server and stores them
// as Prometheus metrics.
func (e *Exporter) GatherMetrics() {
	log.Debug("GatherMetrics() called")
	if time.Now().Unix() < e.nextCollectionTicker {
		return
	}
	e.Lock()
	log.Debug("GatherMetrics() locked")
	defer e.Unlock()
	if len(e.metrics) > 0 {
		e.metrics = e.metrics[:0]
		log.Debug("GatherMetrics() cleared metrics")
	}
	upValue := 1
	isClusterEnabled := false

	var err error

	ports, err := c.Client.OpenFlow.DumpPorts("br0")
	if err != nil {
		log.Error(err)
	}

	for _, i := range ports {
		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on a network interface, in bytes.",
			// []string{"port"},
			// nil)
			InterfaceRxBytesDesc,
			prometheus.CounterValue,
			float64(i.Received.Bytes),
			// []string{"port"},
			fmt.Sprintf("%v", i.PortID),
		))
		log.Debug("%s: GatherMetrics() completed GetInterfaceRxBytes")

		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on  a network interface, in bytes.",
			// []string{"port"},
			// nil)
			InterfaceRxPacketsDesc,
			prometheus.CounterValue,
			float64(i.Received.Packets),
			fmt.Sprintf("%v", i.PortID),
		))
		log.Debug("%s: GatherMetrics() completed GetInterfaceRxPackets")

		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on  a network interface, in bytes.",
			// []string{"port"},
			// nil)
			InterfaceTxBytesDesc,
			prometheus.CounterValue,
			float64(i.Transmitted.Bytes),
			fmt.Sprintf("%v", i.PortID),
		))
		log.Debug("%s: GatherMetrics() completed GetInterfaceTxBytes")

		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on  a network interface, in bytes.",
			// []string{"port"},
			// nil)
			InterfaceTxPacketsDesc,
			prometheus.CounterValue,
			float64(i.Transmitted.Packets),
			fmt.Sprintf("%v", i.PortID),
		))
		log.Debug("%s: GatherMetrics() completed GetInterfaceTxPackets")
	}

	e.nextCollectionTicker = time.Now().Add(time.Duration(e.pollInterval) * time.Second).Unix()

	log.Debug("GatherMetrics() returns")
	return
}

func init() {
	prometheus.MustRegister(version.NewCollector(namespace + "_exporter"))
}

// GetVersionInfo returns exporter info.
func GetVersionInfo() string {
	return version.Info()
}

// GetVersionBuildContext returns exporter build context.
func GetVersionBuildContext() string {
	return version.BuildContext()
}

// GetVersion returns exporter version.
func GetVersion() string {
	return version.Version
}

// GetRevision returns exporter revision.
func GetRevision() string {
	return version.Revision
}

// GetExporterName returns exporter name.
func GetExporterName() string {
	return appName
}

// SetPollInterval sets exporter's polling interval.
func (e *Exporter) SetPollInterval(i int64) {
	e.pollInterval = i
}
