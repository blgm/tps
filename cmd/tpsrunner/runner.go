package tpsrunner

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"

	"code.cloudfoundry.org/tps/config"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/ifrit/ginkgomon"
)

func NewListener(bin, listenAddr, bbsAddress, trafficControllerURL, consulCluster string) *ginkgomon.Runner {
	configFile, err := ioutil.TempFile("", "listener_config")
	Expect(err).NotTo(HaveOccurred())

	listenerConfig := config.DefaultListenerConfig()
	listenerConfig.BBSAddress = bbsAddress
	listenerConfig.ListenAddress = listenAddr
	listenerConfig.LagerConfig.LogLevel = "debug"
	listenerConfig.ConsulCluster = consulCluster
	listenerConfig.TrafficControllerURL = trafficControllerURL

	listenerJSON, err := json.Marshal(listenerConfig)
	Expect(err).NotTo(HaveOccurred())
	err = ioutil.WriteFile(configFile.Name(), listenerJSON, 0644)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:       "tps-listener",
		StartCheck: "tps-listener.started",
		Command: exec.Command(
			bin,
			"-configPath", configFile.Name(),
		),
	})
}

func NewWatcher(bin, bbsAddress, ccBaseURL, consulCluster string) *ginkgomon.Runner {
	configFile, err := ioutil.TempFile("", "listener_config")
	Expect(err).NotTo(HaveOccurred())

	watcherConfig := config.DefaultWatcherConfig()
	watcherConfig.BBSAddress = bbsAddress
	watcherConfig.ConsulCluster = consulCluster
	watcherConfig.CCBaseUrl = ccBaseURL
	watcherConfig.LagerConfig.LogLevel = "debug"
	watcherConfig.CCClientCert = "../../fixtures/watcher_cc_client.crt"
	watcherConfig.CCClientKey = "../../fixtures/watcher_cc_client.key"
	watcherConfig.CCCACert = "../../fixtures/watcher_cc_ca.crt"

	watcherJSON, err := json.Marshal(watcherConfig)
	Expect(err).NotTo(HaveOccurred())
	err = ioutil.WriteFile(configFile.Name(), watcherJSON, 0644)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name: "tps-watcher",
		Command: exec.Command(
			bin,
			"-configPath", configFile.Name(),
		),
		StartCheck: "tps-watcher.started",
	})
}
