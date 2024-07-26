/*
Package main in the directory config_gen implements a tool to read configuration from a template,
and generate customized configuration files for each node.
The generated configuration file particularly contains the public/private keys for TS and ED25519.
*/
package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gitzhang10/BFT/sign"
	"github.com/spf13/viper"
)

func judgeWhetherInSlice(i int, b []int) bool {
	for _, v := range b {
		if i == v {
			return true
		}
	}
	return false
}

func generateRandomNumber(nodeNum int, faultyNum int) []int {
	var nums []int
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for len(nums) < faultyNum {
		num := r.Intn(nodeNum)
		// discard duplicates
		if !judgeWhetherInSlice(num, nums) {
			nums = append(nums, num)
		}
	}
	return nums
}

func main() {

	viperRead := viper.New()
	// for environment variables
	viperRead.SetEnvPrefix("")
	viperRead.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viperRead.SetEnvKeyReplacer(replacer)
	viperRead.SetConfigName("config_template")
	viperRead.AddConfigPath("./")
	err := viperRead.ReadInConfig()
	if err != nil {
		panic(err)
	}

	leaderCount := 1  // the number of nodes in first machine
	ProcessCount := 1 // the number of nodes in other machines

	// deal with cluster as a string map
	ClusterMapInterface := viperRead.GetStringMap("IPs")
	clusterMapInterface := make(map[string]string)
	for name, addr := range ClusterMapInterface {
		rs := []rune(name)
		ipIndex, _ := strconv.Atoi(string(rs[4:]))
		if addrAsString, ok := addr.(string); ok {
			for j := 0; j < ProcessCount; j++ {
				if ipIndex == 0 {
					for k := 0; k < leaderCount; k++ {
						suScript := strconv.Itoa(k)
						clusterMapInterface["node"+suScript] = addrAsString
					}
					break
				}
				suScript := strconv.Itoa((ipIndex-1)*ProcessCount + j + leaderCount)
				clusterMapInterface["node"+suScript] = addrAsString
			}
		} else {
			panic("cluster in the config file cannot be decoded correctly")
		}
	}
	nodeNumber := len(ClusterMapInterface)
	clusterMapString := make(map[string]string, nodeNumber)
	clusterName := make([]string, nodeNumber)
	i := 0
	for name, addr := range ClusterMapInterface {
		if addrAsString, ok := addr.(string); ok {
			clusterMapString[name] = addrAsString
			clusterName[i] = name
			i++
		} else {
			panic("cluster in the config file cannot be decoded correctly")
		}
	}
	sort.Strings(clusterName)

	// deal with p2p_listen_port as a string map
	P2pPortMapInterface := viperRead.GetStringMap("peers_p2p_port")
	if nodeNumber != len(P2pPortMapInterface) {
		panic("p2p_listen_port does not match with cluster")
	}
	p2pPortMapInterface := make(map[string]int)
	mapNameToP2PPort := make(map[string]int, nodeNumber)
	for name := range clusterMapString {
		portAsInterface, ok := P2pPortMapInterface[name]
		if !ok {
			panic("p2p_listen_port does not match with cluster")
		}
		if portAsInt, ok := portAsInterface.(int); ok {
			mapNameToP2PPort[name] = portAsInt
			rs := []rune(name)
			ipIndex, _ := strconv.Atoi(string(rs[4:]))
			for j := 0; j < ProcessCount; j++ {
				if ipIndex == 0 {
					for k := 0; k < leaderCount; k++ {
						subScript := strconv.Itoa(k)
						p2pPortMapInterface["node"+subScript] = portAsInt + k*10
					}
					break
				}
				subScript := strconv.Itoa((ipIndex-1)*ProcessCount + j + leaderCount)
				p2pPortMapInterface["node"+subScript] = portAsInt + j*10
			}
		} else {
			panic("p2p_listen_port contains a non-int value")
		}
	}
	// deal with rpc_listen_port as a string map
	rpcPortMapInterface := make(map[string]int)
	for name, port := range p2pPortMapInterface {
		rpcPortMapInterface[name] = port - 2000
	}

	// create the ED25519 keys
	privKeysED25519 := make(map[string]string)
	pubKeysED25519 := make(map[string]string)
	var privKeyED, pubKeyED []byte
	for i := 0; i < nodeNumber; i++ {
		if i == 0 {
			for k := 0; k < leaderCount; k++ {
				privKeyED, pubKeyED = sign.GenED25519Keys()
				subScript := strconv.Itoa(k)
				pubKeysED25519["node"+subScript] = hex.EncodeToString(pubKeyED)
				privKeysED25519["node"+subScript] = hex.EncodeToString(privKeyED)
			}
			continue
		}
		for j := 0; j < ProcessCount; j++ {
			privKeyED, pubKeyED = sign.GenED25519Keys()
			subScript := strconv.Itoa((i-1)*ProcessCount + j + leaderCount)
			pubKeysED25519["node"+subScript] = hex.EncodeToString(pubKeyED)
			privKeysED25519["node"+subScript] = hex.EncodeToString(privKeyED)
		}
	}

	// create the threshold signature keys
	TotalNodeNum := (nodeNumber-1)*ProcessCount + leaderCount
	numT := TotalNodeNum - TotalNodeNum/3
	shares, pubPoly := sign.GenTSKeys(numT, TotalNodeNum)

	// load simple parameter
	maxPool := viperRead.GetInt("max_pool")
	batchSize := viperRead.GetInt("batch_size")
	logLevel := viperRead.GetInt("log_level")
	round := viperRead.GetInt("round")
	protocol := viperRead.GetString("protocol")
	faultyNum := viperRead.GetInt("faulty_number")
	faultyNode := generateRandomNumber(TotalNodeNum, faultyNum)
	fmt.Println("FaultyNodes:", faultyNode)

	// write to configure files
	for _, name := range clusterName {
		viperWrite := viper.New()
		var loopCount int
		rs := []rune(name)
		ipIndex, err := strconv.Atoi(string(rs[4:]))
		if err != nil {
			panic("get replicaId failed")
		}
		if ipIndex == 0 {
			loopCount = leaderCount
		} else {
			loopCount = ProcessCount
		}
		for j := 0; j < loopCount; j++ {
			index := strconv.Itoa(j)
			var replicaId int
			if ipIndex == 0 {
				replicaId = j
			} else {
				replicaId = (ipIndex-1)*ProcessCount + j + leaderCount
			}
			viperWrite.SetConfigFile(fmt.Sprintf("%s_%s.yaml", name, index))
			shareAsBytes, err := sign.EncodeTSPartialKey(shares[replicaId])
			if err != nil {
				panic("fail encode the share")
			}
			tsPubKeyAsBytes, err := sign.EncodeTSPublicKey(pubPoly)
			if err != nil {
				panic("fail encode the TSPublicKey")
			}

			viperWrite.Set("name", "node"+strconv.Itoa(replicaId))
			//viperWrite.Set("address", clusterMapString[name])
			//viperWrite.Set("p2p_listen_port", mapNameToP2PPort[name]+j*10)
			viperWrite.Set("peers_p2p_port", p2pPortMapInterface)
			viperWrite.Set("peers_rpc_port", rpcPortMapInterface)
			viperWrite.Set("max_pool", maxPool)
			viperWrite.Set("batch_size", batchSize)
			viperWrite.Set("PrivKeyED", privKeysED25519["node"+strconv.Itoa(replicaId)])
			viperWrite.Set("cluster_pubkeyed", pubKeysED25519)
			viperWrite.Set("TSShare", hex.EncodeToString(shareAsBytes))
			viperWrite.Set("TSPubKey", hex.EncodeToString(tsPubKeyAsBytes))
			viperWrite.Set("log_level", logLevel)
			viperWrite.Set("cluster_ips", clusterMapInterface)
			viperWrite.Set("round", round)
			viperWrite.Set("protocol", protocol)

			if judgeWhetherInSlice(replicaId, faultyNode) {
				viperWrite.Set("is_faulty", 1)
			} else {
				viperWrite.Set("is_faulty", 0)
			}
			_ = viperWrite.WriteConfig()
		}
	}
}
