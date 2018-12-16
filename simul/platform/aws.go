package platform

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ConsenSys/handel/simul/lib"
	"github.com/ConsenSys/handel/simul/platform/aws"
)

type awsPlatform struct {
	aws           aws.Manager
	c             *lib.Config
	regPath       string
	binPath       string
	binMasterPath string
	confPath      string
	targetSystem  string
	targetArch    string
	user          string
	pemBytes      []byte
}

func NewAws(aws aws.Manager, pemFile string) Platform {
	pemBytes, err := ioutil.ReadFile(pemFile)
	if err != nil {
		panic(err)
	}
	return &awsPlatform{aws: aws,
		targetSystem: "linux",
		targetArch:   "amd64",
		user:         "ubuntu",
		pemBytes:     pemBytes,
	}
}

func (a *awsPlatform) pack(path string, c *lib.Config, binPath string) error {
	// Compile binaries
	//GOOS=linux GOARCH=amd64 go build
	//os.Setenv("GOOS", a.targetSystem)
	//os.Setenv("GOARCH", a.targetArch)
	os.Setenv("GOOS", a.targetSystem)
	os.Setenv("GOARCH", a.targetArch)
	cmd := NewCommand("go", "build", "-o", binPath, path)

	if err := cmd.Run(); err != nil {
		fmt.Println("stdout -> " + cmd.ReadAll())
		return err
	}
	return nil
}

func (a *awsPlatform) Configure(c *lib.Config) error {
	a.c = c
	a.regPath = "/tmp/aws.csv"
	a.binPath = "/tmp/nodeAWS"
	a.binMasterPath = "/tmp/masterAWS"
	a.confPath = "/tmp/aws.conf"

	// Compile binaries
	a.pack("github.com/ConsenSys/handel/simul/node", c, a.binPath)
	a.pack("github.com/ConsenSys/handel/simul/master", c, a.binMasterPath)

	// write config
	if err := c.WriteTo(a.confPath); err != nil {
		return err
	}

	if err := a.aws.StartInstances(); err != nil {
		return err
	}
	return nil
}

func (a *awsPlatform) Cleanup() error {
	return nil
}

func (a *awsPlatform) Start(idx int, r *lib.RunConfig) error {
	cons := a.c.NewConstructor()
	parser := lib.NewCSVParser()
	allAwsInstances := a.aws.Instances()
	nbOfInstances := len(allAwsInstances)

	if r.Nodes+1 > nbOfInstances {
		msg := fmt.Sprintf(`Not enough EC2 instances, number of nodes to sart: %d
            , number of avaliable EC2 instances: %d`, r.Nodes+1, nbOfInstances)
		return errors.New(msg)
	}

	masterInstance, slaveInstances, err := makeMasterAndSlaves(allAwsInstances)
	if err != nil {
		return nil
	}
	masterAddr := aws.GenRemoteAddress(*masterInstance.PublicIP, 5000)

	fmt.Println("[+] Master Instances")
	fmt.Println("	 [-] Instance ", *masterInstance.ID, *masterInstance.State, masterAddr)
	fmt.Println()
	fmt.Println("[+] Avaliable Slave Instances:")
	for i, inst := range slaveInstances {
		fmt.Println("	 [-] Instance ", i, *inst.ID, *inst.State, *inst.PublicIP)
	}

	// ++++++++++Copy master binary to master instance

	// a) Copy all files to master
	// b) Setup NFS on master
	// c) Start master
	// d) Setup NFS on slaves
	// e) copy files on slaves
	// f) start slaves

	log := "log.txt"
	masterCommand := "nohup " + a.binMasterPath + " -masterAddr " + masterAddr + " -nbOfNodes " + strconv.Itoa(len(slaveInstances))
	fullCMD := "chmod 777 " + a.binMasterPath + " && " + masterCommand + " > " + log

	fmt.Println("[+] >>>>>>> Master Command", fullCMD)
	if err := a.runNode(masterAddr, fullCMD, a.binMasterPath); err != nil {
		return err
	}

	// 1. Generate & write the registry file
	addresses, syncs := aws.GenRemoteAddresses(slaveInstances)
	nodes := lib.GenerateNodes(cons, addresses)
	lib.WriteAll(nodes, parser, a.regPath)
	fmt.Println("[+] Registry file written to local storage(", r.Nodes, " nodes)")
	var wg sync.WaitGroup

	sameCmd := []string{
		"chmod 777 " + a.binPath,
		"&&",
		a.binPath, "-config", a.confPath, "-registry", a.regPath, "-master", masterAddr}

	for _, n := range nodes {
		wg.Add(1)
		go func(node *lib.Node, cmd []string) {
			nodeID := int(node.ID())
			cmd = append(cmd, []string{
				"-id", strconv.Itoa(nodeID),
				"-sync", syncs[nodeID],
				"-run", strconv.Itoa(idx),
				">", log}...)

			cmdStr := cmdToString(cmd)
			fmt.Println("Slave CMD:  ", cmdStr)
			a.runNode(node.Address(), cmdStr, a.regPath, a.confPath, a.binPath)
			//	a.runNode(node.Address(), "pwd", a.regPath, a.confPath)
			wg.Done()
		}(n, sameCmd)
	}
	wg.Wait()
	return nil
}

func (a *awsPlatform) runNode(addr, cmd string, files ...string) error {
	awsClient, err := aws.NewSSHClient(a.pemBytes, addr, a.user)
	if err != nil {
		return err
	}
	awsClient.CopyFiles(files)
	awsClient.Start(cmd)
	awsClient.Close()
	return nil
}

func cmdToString(cmd []string) string {
	return strings.Join(cmd[:], " ")
}

func makeMasterAndSlaves(allAwsInstances []aws.Instance) (*aws.Instance, []aws.Instance, error) {
	var masterInstance aws.Instance
	var slaveInstances []aws.Instance
	nbOfMasterIns := 0
	for _, inst := range allAwsInstances {
		if inst.Tag == aws.RnDMasterTag {
			if nbOfMasterIns > 1 {
				return nil, nil, errors.New("More than one Master instance avaliable")
			}
			masterInstance = inst
			nbOfMasterIns++
		} else {
			slaveInstances = append(slaveInstances, inst)
		}
	}
	return &masterInstance, slaveInstances, nil
}
