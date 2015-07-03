package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
)

var discoveryURL = flag.String("discovery-url", "", "The etcd discovery url")
var pollInterval = flag.Duration("poll-interval", 5*time.Minute,
	"How often to check etcd for updates")

var ports = []int{2380, 2379, 4001}

const configPath = "/haproxy.cfg"

func main() {
	flag.Parse()
	if err := Main(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

type DiscoveryResponse struct {
	Node Node `json:"node"`
}

type Node struct {
	Nodes []Node `json:"nodes"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Dir   bool   `json:"dir"`
}

func getHosts() ([]string, error) {
	res, err := http.Get(*discoveryURL)
	if err != nil {
		return nil, err
	}

	discoveryResponse := DiscoveryResponse{}
	if err := json.NewDecoder(res.Body).Decode(&discoveryResponse); err != nil {
		return nil, err
	}

	hosts := []string{}
	for _, node := range discoveryResponse.Node.Nodes {
		host := node.Value
		host = strings.TrimPrefix(host, "http://")
		host = strings.TrimSuffix(host, ":7001")
		hosts = append(hosts, host)
	}
	return hosts, err
}

func getConfig(hosts []string) []byte {
	configBuf := bytes.NewBuffer(nil)
	fmt.Fprintf(configBuf, `
global
  log 127.0.0.1 local0
  log 127.0.0.1 local1 notice
  maxconn 4096
  chroot /var/lib/haproxy
  user haproxy
  group haproxy

defaults
  log global
  option dontlognull
  timeout connect 5000
  timeout client 50000
  timeout server 50000
`)
	for _, port := range ports {
		fmt.Fprintf(configBuf, `
listen port%d :%d
  mode tcp
  option tcplog
  balance leastconn
`, port, port)
		for index, host := range hosts {
			fmt.Fprintf(configBuf, "  server server-%d %s:%d\n",
				index, host, port)
		}
	}

	return configBuf.Bytes()
}

func Configure() (bool, error) {
	hosts, err := getHosts()
	if err != nil {
		return false, err
	}

	newConfig := getConfig(hosts)

	// Make sure the hosts are in the same order (so that we don't generate)
	// spurious changes.
	sort.Strings(hosts)

	// Abort if the config hasn't changed.
	oldConfig, err := ioutil.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if bytes.Equal(newConfig, oldConfig) {
		return false, nil
	}

	// Write the new config.
	if err := ioutil.WriteFile(configPath, newConfig, 0644); err != nil {
		return false, err
	}
	return true, nil
}

func Main() error {
	if _, err := Configure(); err != nil {
		return err
	}

	cmd := exec.Command("haproxy", "-f", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}

	// ticker emits an event whenever we should re-poll the configuration
	// service
	ticker := time.NewTicker(*pollInterval)

	// if we get SIGINT or SIGKILL stop
	gotShutdownSignal := false
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func() {
		_ = <-signalCh
		gotShutdownSignal = true
		fmt.Printf("shutting down\n")
		if p := cmd.Process; p != nil {
			p.Kill()
			os.Exit(0)
		}
	}()

	// if the command stops, stop the ticker
	go func() {
		cmd.Wait()
		if !gotShutdownSignal {
			fmt.Printf("haproxy exited\n")
			os.Exit(1)
		}
	}()

	for _ = range ticker.C {
		changed, err := Configure()
		if err != nil {
			return err
		}
		if changed {
			fmt.Printf("reconfiguring haproxy\n")
			if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
				return err
			}
		}
	}

	return nil // not reached
}
