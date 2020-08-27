package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	ovs "github.com/kongseokhwan/hellios-prometheus-exporter/pkg/ovs_exporter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
)

func main() {
	var listenAddress string
	var metricsPath string
	var pollTimeout int
	var pollInterval int
	var isShowVersion bool
	var logLevel string
	var systemRunDir string
	var databaseVswitchName string
	var databaseVswitchSocketRemote string
	var databaseVswitchFileDataPath string
	var databaseVswitchFileLogPath string
	var databaseVswitchFilePidPath string
	var databaseVswitchFileSystemIDPath string
	var serviceVswitchdFileLogPath string
	var serviceVswitchdFilePidPath string
	var bridgeAddr string

	flag.StringVar(&listenAddress, "web.listen-address", ":9476", "Address to listen on for web interface and telemetry.")
	flag.StringVar(&metricsPath, "web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	flag.IntVar(&pollTimeout, "ovs.timeout", 2, "Timeout on gRPC requests to ovs.")
	flag.IntVar(&pollInterval, "ovs.poll-interval", 15, "The minimum interval (in seconds) between collections from ovs server.")
	flag.BoolVar(&isShowVersion, "version", false, "version information")
	flag.StringVar(&logLevel, "log.level", "info", "logging severity level")

	flag.StringVar(&systemRunDir, "system.run.dir", "/var/run/openvswitch", "OVS default run directory.")

	flag.StringVar(&databaseVswitchName, "database.vswitch.name", "Open_vSwitch", "The name of OVS db.")
	flag.StringVar(&databaseVswitchSocketRemote, "database.vswitch.socket.remote", "unix:/var/run/openvswitch/db.sock", "JSON-RPC unix socket to OVS db.")
	flag.StringVar(&databaseVswitchFileDataPath, "database.vswitch.file.data.path", "/etc/openvswitch/conf.db", "OVS db file.")
	flag.StringVar(&databaseVswitchFileLogPath, "database.vswitch.file.log.path", "/var/log/openvswitch/ovsdb-server.log", "OVS db log file.")
	flag.StringVar(&databaseVswitchFilePidPath, "database.vswitch.file.pid.path", "/var/run/openvswitch/ovsdb-server.pid", "OVS db process id file.")
	flag.StringVar(&databaseVswitchFileSystemIDPath, "database.vswitch.file.system.id.path", "/etc/openvswitch/system-id.conf", "OVS system id file.")

	flag.StringVar(&serviceVswitchdFileLogPath, "service.vswitchd.file.log.path", "/var/log/openvswitch/ovs-vswitchd.log", "OVS vswitchd daemon log file.")
	flag.StringVar(&serviceVswitchdFilePidPath, "service.vswitchd.file.pid.path", "/var/run/openvswitch/ovs-vswitchd.pid", "OVS vswitchd daemon process id file.")

	flag.StringVar(&bridgeAddr, "bridge.addr", "tcp:10.1.100.154:6633", "The address of Helios Bridge")

	var usageHelp = func() {
		fmt.Fprintf(os.Stderr, "\n%s - Prometheus Exporter for Open VSwitvh (ovs)\n\n", ovs.GetExporterName())
		fmt.Fprintf(os.Stderr, "Usage: %s [arguments]\n\n", ovs.GetExporterName())
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nDocumentation: https://github.com/kongseokhwan/hellios-prometheus-exporter/\n\n")
	}
	flag.Usage = usageHelp
	flag.Parse()

	opts := ovs.Options{
		Timeout: pollTimeout,
	}

	if err := log.Base().SetLevel(logLevel); err != nil {
		log.Errorf(err.Error())
		os.Exit(1)
	}

	if isShowVersion {
		fmt.Fprintf(os.Stdout, "%s %s", ovs.GetExporterName(), ovs.GetVersion())
		if ovs.GetRevision() != "" {
			fmt.Fprintf(os.Stdout, ", commit: %s\n", ovs.GetRevision())
		} else {
			fmt.Fprint(os.Stdout, "\n")
		}
		os.Exit(0)
	}

	log.Infof("Starting %s %s", ovs.GetExporterName(), ovs.GetVersionInfo())
	log.Infof("Build context %s", ovs.GetVersionBuildContext())

	exporter, err := ovs.NewExporter(opts, bridgeAddr)
	if err != nil {
		log.Errorf("%s failed to init properly: %s", ovs.GetExporterName(), err)
	}

	exporter.SetPollInterval(int64(pollInterval))
	prometheus.MustRegister(exporter)

	http.Handle(metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>ovs Exporter</title></head>
             <body>
             <h1>ovs Exporter</h1>
             <p><a href='` + metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	log.Infoln("Listening on", listenAddress)
	log.Fatal(http.ListenAndServe(listenAddress, nil))
}
