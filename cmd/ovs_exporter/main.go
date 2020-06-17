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
	var databaseNorthboundName string
	var databaseNorthboundSocketRemote string
	var databaseNorthboundSocketControl string
	var databaseNorthboundFileDataPath string
	var databaseNorthboundFileLogPath string
	var databaseNorthboundFilePidPath string
	var databaseNorthboundPortDefault int
	var databaseNorthboundPortSsl int
	var databaseNorthboundPortRaft int
	var databaseSouthboundName string
	var databaseSouthboundSocketRemote string
	var databaseSouthboundSocketControl string
	var databaseSouthboundFileDataPath string
	var databaseSouthboundFileLogPath string
	var databaseSouthboundFilePidPath string
	var databaseSouthboundPortDefault int
	var databaseSouthboundPortSsl int
	var databaseSouthboundPortRaft int
	var serviceVswitchdFileLogPath string
	var serviceVswitchdFilePidPath string
	var serviceNorthdFileLogPath string
	var serviceNorthdFilePidPath string

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

	flag.StringVar(&databaseNorthboundName, "database.northbound.name", "ovs_Northbound", "The name of ovs NB (northbound) db.")
	flag.StringVar(&databaseNorthboundSocketRemote, "database.northbound.socket.remote", "unix:/run/openvswitch/ovsnb_db.sock", "JSON-RPC unix socket to ovs NB db.")
	flag.StringVar(&databaseNorthboundSocketControl, "database.northbound.socket.control", "unix:/run/openvswitch/ovsnb_db.ctl", "JSON-RPC unix socket to ovs NB app.")
	flag.StringVar(&databaseNorthboundFileDataPath, "database.northbound.file.data.path", "/var/lib/openvswitch/ovsnb_db.db", "ovs NB db file.")
	flag.StringVar(&databaseNorthboundFileLogPath, "database.northbound.file.log.path", "/var/log/openvswitch/ovsdb-server-nb.log", "ovs NB db log file.")
	flag.StringVar(&databaseNorthboundFilePidPath, "database.northbound.file.pid.path", "/run/openvswitch/ovsnb_db.pid", "ovs NB db process id file.")
	flag.IntVar(&databaseNorthboundPortDefault, "database.northbound.port.default", 6641, "ovs NB db network socket port.")
	flag.IntVar(&databaseNorthboundPortSsl, "database.northbound.port.ssl", 6631, "ovs NB db network socket secure port.")
	flag.IntVar(&databaseNorthboundPortRaft, "database.northbound.port.raft", 6643, "ovs NB db network port for clustering (raft)")

	flag.StringVar(&databaseSouthboundName, "database.southbound.name", "ovs_Southbound", "The name of ovs SB (southbound) db.")
	flag.StringVar(&databaseSouthboundSocketRemote, "database.southbound.socket.remote", "unix:/run/openvswitch/ovssb_db.sock", "JSON-RPC unix socket to ovs SB db.")
	flag.StringVar(&databaseSouthboundSocketControl, "database.southbound.socket.control", "unix:/run/openvswitch/ovssb_db.ctl", "JSON-RPC unix socket to ovs SB app.")
	flag.StringVar(&databaseSouthboundFileDataPath, "database.southbound.file.data.path", "/var/lib/openvswitch/ovssb_db.db", "ovs SB db file.")
	flag.StringVar(&databaseSouthboundFileLogPath, "database.southbound.file.log.path", "/var/log/openvswitch/ovsdb-server-sb.log", "ovs SB db log file.")
	flag.StringVar(&databaseSouthboundFilePidPath, "database.southbound.file.pid.path", "/run/openvswitch/ovssb_db.pid", "ovs SB db process id file.")
	flag.IntVar(&databaseSouthboundPortDefault, "database.southbound.port.default", 6642, "ovs SB db network socket port.")
	flag.IntVar(&databaseSouthboundPortSsl, "database.southbound.port.ssl", 6632, "ovs SB db network socket secure port.")
	flag.IntVar(&databaseSouthboundPortRaft, "database.southbound.port.raft", 6644, "ovs SB db network port for clustering (raft)")

	flag.StringVar(&serviceVswitchdFileLogPath, "service.vswitchd.file.log.path", "/var/log/openvswitch/ovs-vswitchd.log", "OVS vswitchd daemon log file.")
	flag.StringVar(&serviceVswitchdFilePidPath, "service.vswitchd.file.pid.path", "/var/run/openvswitch/ovs-vswitchd.pid", "OVS vswitchd daemon process id file.")

	flag.StringVar(&serviceNorthdFileLogPath, "service.ovs.northd.file.log.path", "/var/log/openvswitch/ovs-northd.log", "ovs northd daemon log file.")
	flag.StringVar(&serviceNorthdFilePidPath, "service.ovs.northd.file.pid.path", "/run/openvswitch/ovs-northd.pid", "ovs northd daemon process id file.")

	var usageHelp = func() {
		fmt.Fprintf(os.Stderr, "\n%s - Prometheus Exporter for Open Virtual Network (ovs)\n\n", ovs.GetExporterName())
		fmt.Fprintf(os.Stderr, "Usage: %s [arguments]\n\n", ovs.GetExporterName())
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nDocumentation: https://github.com/forward53/ovs_exporter/\n\n")
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

	exporter, err := ovs.NewExporter(opts)
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
