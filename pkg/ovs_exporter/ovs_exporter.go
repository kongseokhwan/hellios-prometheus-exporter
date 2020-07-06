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
	"os/exec"
	"strings"
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
	InterfaceRxDroppedDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "interface", "receive_drop_total"),
		"Number of packets dropped on a network interface, in counts.",
		[]string{"port"},
		nil)
	InterfaceRxErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "interface", "receive_errors_total"),
		"Number of packets errored on a network interface, in counts.",
		[]string{"port"},
		nil)
	InterfaceRxCRCDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "interface", "receive_crc_total"),
		"Number of CRC on a network interface, in counts.",
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
	InterfaceTxDroppedDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "interface", "transmit_drop_total"),
		"Number of packets dropped on a network interface, in counts.",
		[]string{"port"},
		nil)
	InterfaceTxErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "interface", "transmit_errors_total"),
		"Number of packets errored on a network interface, in counts.",
		[]string{"port"},
		nil)
	InterfaceTxCollisionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "interface", "transmit_collisions_total"),
		"Number of Collisions on a network interface, in counts.",
		[]string{"port"},
		nil)
	// TODO : Flow case, key : switchName, flowInformations
	FlowPktsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "flow", "flow_packtes_total"),
		"Number of packets on a flow, in counts.",
		[]string{"bridge", "flow"},
		nil)
	FlowBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ovs", "flow", "flow_bytes_total"),
		"Number of bytes on a flow, in counts.",
		[]string{"bridge", "flow"},
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

type SwitchFlowStats struct {
	BrigeName string
	Flow      *ovs.Flow
	FlowStats *ovs.FlowStats
}

type Options struct {
	Timeout int
}

func portFilter(ports []*ovs.PortStats, f func(*ovs.PortStats) bool) []*ovs.PortStats {
	portStats := make([]*ovs.PortStats, 0)

	for _, v := range ports {
		if f(v) {
			portStats = append(portStats, v)
		}
	}
	return portStats
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
	ch <- InterfaceRxCRCDesc
	ch <- InterfaceRxDroppedDesc
	ch <- InterfaceRxErrorsDesc
	ch <- InterfaceTxBytesDesc
	ch <- InterfaceTxPacketsDesc
	ch <- InterfaceTxCollisionsDesc
	ch <- InterfaceTxDroppedDesc
	ch <- InterfaceTxErrorsDesc
	ch <- FlowBytesDesc
	ch <- FlowPktsDesc
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
	log.Debug("Collect() calls RLock()")
	e.RLock()
	defer e.RUnlock()
	if len(e.metrics) == 0 {
		log.Debug("Collect() no metrics found")
		return
	}

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

	var ovsBridges []string
	var portStats []*ovs.PortStats
	var flows []*ovs.Flow
	var flowStats []SwitchFlowStats
	var tables []*ovs.Table

	// 1. get whole switches use go net link library
	// construct `go version` command
	cmd := exec.Command("sudo", "ovs-vsctl", "list-br")

	// run command
	if bridges, errC := cmd.Output(); errC != nil {
		log.Debugf("Error: %n", errC)
	} else {
		// parse & translate & string & push to
		log.Debugf("Otuput: %s\n", bridges)
		tmpBridges := string(bridges)
		ovsBridges = strings.Fields(tmpBridges)
	}

	for _, br := range ovsBridges {
		log.Debugf("Bridge Name : %s\n", br)
		brPorts, err := e.Client.OpenFlow.DumpPorts(br)
		if err != nil {
			log.Error(err)
		} else {
			filteredPortStats := portFilter(brPorts, func(val *ovs.PortStats) bool {
				return val.PortID > 0
			})
			//portStats = append(portStats, brPorts...)
			portStats = append(portStats, filteredPortStats...)
		}

		// 2. Create SwithFlowStats Slice with Flow & BridgeName
		brFlows, err := e.Client.OpenFlow.DumpFlows(br)
		if err != nil {
			log.Error(err)
		} else {
			for _, flow := range brFlows {
				var tmpFlowStat SwitchFlowStats
				tmpFlowStat.Flow = flow
				tmpFlowStat.BrigeName = br
				flowStats = append(flowStats, tmpFlowStat)
			}
			flows = append(flows, brFlows...)
		}

		brTables, err := e.Client.OpenFlow.DumpTables(br)
		if err != nil {
			log.Error(err)
		} else {
			tables = append(tables, brTables...)
		}
	}

	// 2. get whole port statistics
	for _, i := range portStats {
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
		log.Debugf("%v: GatherMetrics() completed GetInterfaceRxBytes", i.PortID)

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
		log.Debugf("%v: GatherMetrics() completed GetInterfaceRxPackets", i.PortID)

		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on  a network interface, in bytes.",
			// []string{"port"},
			// nil)
			InterfaceRxCRCDesc,
			prometheus.CounterValue,
			float64(i.Received.CRC),
			fmt.Sprintf("%v", i.PortID),
		))
		log.Debugf("%v: GatherMetrics() completed GetInterfaceRxCRC", i.PortID)

		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on  a network interface, in bytes.",
			// []string{"port"},
			// nil)
			InterfaceRxDroppedDesc,
			prometheus.CounterValue,
			float64(i.Received.Dropped),
			fmt.Sprintf("%v", i.PortID),
		))
		log.Debugf("%v: GatherMetrics() completed GetInterfaceRxDropped", i.PortID)

		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on  a network interface, in bytes.",
			// []string{"port"},
			// nil)
			InterfaceRxErrorsDesc,
			prometheus.CounterValue,
			float64(i.Received.Errors),
			fmt.Sprintf("%v", i.PortID),
		))
		log.Debugf("%v: GatherMetrics() completed GetInterfaceRxErrors", i.PortID)

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
		log.Debugf("%v: GatherMetrics() completed GetInterfaceTxBytes", i.PortID)

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
		log.Debugf("%v: GatherMetrics() completed GetInterfaceTxPackets", i.PortID)

		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on  a network interface, in bytes.",
			// []string{"port"},
			// nil)
			InterfaceTxCollisionsDesc,
			prometheus.CounterValue,
			float64(i.Transmitted.Collisions),
			fmt.Sprintf("%v", i.PortID),
		))
		log.Debugf("%v: GatherMetrics() completed GetInterfaceTxCollisions", i.PortID)

		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on  a network interface, in bytes.",
			// []string{"port"},
			// nil)
			InterfaceTxDroppedDesc,
			prometheus.CounterValue,
			float64(i.Transmitted.Dropped),
			fmt.Sprintf("%v", i.PortID),
		))
		log.Debugf("%v: GatherMetrics() completed GetInterfaceTxDropped", i.PortID)

		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on  a network interface, in bytes.",
			// []string{"port"},
			// nil)
			InterfaceTxErrorsDesc,
			prometheus.CounterValue,
			float64(i.Transmitted.Errors),
			fmt.Sprintf("%v", i.PortID),
		))
		log.Debugf("%v: GatherMetrics() completed GetInterfaceTxErrors", i.PortID)
	}

	//3. Make FlowStats
	for _, fl := range flowStats {
		stats, _ := e.Client.OpenFlow.DumpAggregate(fl.BrigeName, fl.Flow.MatchFlow())
		fl.FlowStats = stats

		flowText, _ := fl.Flow.MarshalText()

		//MustNewConstMetric(desc *Desc, valueType ValueType, value float64, labelValues ...string) Metric {
		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on a network interface, in bytes.",
			// []string{"port"},
			// nil)
			FlowBytesDesc,
			prometheus.CounterValue,
			float64(fl.FlowStats.ByteCount),
			fmt.Sprintf("%s", fl.BrigeName),
			fmt.Sprintf("%s", flowText),
		))
		log.Debugf("%s: GatherMetrics() completed GetInterfaceRxBytes", flowText)

		e.metrics = append(e.metrics, prometheus.MustNewConstMetric(
			// prometheus.BuildFQName("ovs", "interface", "receive_bytes_total"),
			// "Number of bytes received on  a network interface, in bytes.",
			// []string{"port"},
			// nil)
			FlowPktsDesc,
			prometheus.CounterValue,
			float64(fl.FlowStats.PacketCount),
			fmt.Sprintf("%s", fl.BrigeName),
			fmt.Sprintf("%s", flowText),
		))
		log.Debugf("%s: GatherMetrics() completed GetInterfaceRxPackets", flowText)
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
