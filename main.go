package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/gitzhang10/BFT/config"
	"github.com/gitzhang10/BFT/gradeddag"
)

var conf *config.Config
var err error

func init() {
	conf, err = config.LoadConfig("", "config")
	if err != nil {
		panic(err)
	}
}

func main() {
	if conf.Protocol == "gradeddag" {
		startGradedDAG()
	} else {
		panic(errors.New("the protocol is unknown"))
	}
}

func startGradedDAG() {
	node := gradeddag.NewNode(conf)
	if err = node.StartP2PListen(); err != nil {
		panic(err)
	}
	// wait for each node to start
	time.Sleep(time.Second * 5)
	if err = node.EstablishP2PConns(); err != nil {
		panic(err)
	}
	node.InitCBC(conf)
	fmt.Println("node starts the GradedDAG!")
	go node.RunLoop()
	go node.HandleMsgLoop()
	go node.CBCOutputBlockLoop()
	node.DoneOutputLoop()

}
