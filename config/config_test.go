package config

import (
	"fmt"
	"testing"
)

func TestConfigRead(t *testing.T) {
	config, err := LoadConfig("./config", "config_test")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("name:", config.Name)
	fmt.Println("clusterPort:", config.ClusterPort)
	fmt.Println("clusterAddr:", config.ClusterAddr)
	fmt.Println("clusterAddrWithPorts:", config.ClusterAddrWithPorts)
	fmt.Println("max_pool:", config.MaxPool)
	fmt.Println("log_level:", config.LogLevel)
	fmt.Println("is_faulty:", config.IsFaulty)
	fmt.Println("batch_size:", config.BatchSize)
	fmt.Println("round:", config.Round)
	fmt.Println("protocol:", config.Protocol)
}
