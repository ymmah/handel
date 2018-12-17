package platform

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

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
	master        *aws.SshCMD
	slaveNodes    []*aws.SshCMD
	addresses     []string
	masterAddr    string
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

	//Start EC2 instances
	if err := a.aws.StartInstances(); err != nil {
		return err
	}

	masterInstance, slaveInstances, err := makeMasterAndSlaves(a.aws.Instances())
	if err != nil {
		fmt.Println(err)
		return err
	}

	masterAddr := aws.GenRemoteAddress(*masterInstance.PublicIP, 5000)
	master, err := aws.NewSSHNodeContlorrer(a.pemBytes, masterAddr, a.user, "")

	if err != nil {
		return err
	}
	a.masterAddr = masterAddr
	a.master = master
	//	time.Sleep(30 * time.Second)
	/*	if err := master.Init(); err != nil {
		fmt.Println("Init failed", err, *masterInstance.ID, *masterInstance.PublicIP, *masterInstance.State)
		return err
	}*/

	for {
		err := master.Init()
		if err != nil {
			fmt.Println("Init failed", err, *masterInstance.ID, *masterInstance.PublicIP, *masterInstance.State)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}

	fmt.Println("[+] Master Instances")
	fmt.Println("	 [-] Instance ", *masterInstance.ID, *masterInstance.State, masterAddr)
	fmt.Println()
	fmt.Println("[+] Avaliable Slave Instances:")

	for i, inst := range slaveInstances {
		fmt.Println("	 [-] Instance ", i, *inst.ID, *inst.State, *inst.PublicIP)
	}

	fmt.Println("[+] Transfering files to Master:", a.binMasterPath, a.binPath, a.confPath)
	master.CopyFiles(a.binMasterPath, a.binPath, a.confPath)
	cmds := aws.MasterConfig(a.binMasterPath, a.binPath, a.confPath)
	//	cmdStr := aws.CMDMapToString(cmds)

	fmt.Println("[+] Configuring Master")
	for idx := 0; idx < len(cmds); idx++ {
		fmt.Println("       Exec:", idx, cmds[idx])
		_, err := master.Run(cmds[idx])
		if err != nil {
			return err
		}
	}

	addresses, syncs := aws.GenRemoteAddresses(slaveInstances)
	a.addresses = addresses
	slaveCmds := aws.SlaveConfig(*masterInstance.PublicIP, a.binMasterPath, a.binPath, a.confPath)
	fmt.Println("")
	fmt.Println("")
	fmt.Println("[+] Configuring Slaves")

	for i, addr := range addresses {
		slaveNodeController, err := aws.NewSSHNodeContlorrer(a.pemBytes, addr, a.user, syncs[i])
		if err != nil {
			return err
		}
		slaveNodeController.Init()
		for idx := 0; idx < len(slaveCmds); idx++ {
			fmt.Println("     Slave ", i, "cmd: ", idx, slaveNodeController.Sync, idx, slaveCmds[idx])
			_, err = slaveNodeController.Run(slaveCmds[idx])
			if err != nil {
				return err
			}
		}
		slaveNodeController.Close()
		a.slaveNodes = append(a.slaveNodes, slaveNodeController)
	}
	return nil
}

func (a *awsPlatform) Cleanup() error {
	return nil
}

func (a *awsPlatform) Start(idx int, r *lib.RunConfig) error {

	/*
	   	if r.Nodes+1 > nbOfInstances {
	   		msg := fmt.Sprintf(`Not enough EC2 instances, number of nodes to sart: %d
	               , number of avaliable EC2 instances: %d`, r.Nodes+1, nbOfInstances)
	   		return errors.New(msg)
	   	}*/

	// ++++++++++Copy master binary to master instance

	cons := a.c.NewConstructor()
	parser := lib.NewCSVParser()
	nodes := lib.GenerateNodes(cons, a.addresses)
	lib.WriteAll(nodes, parser, a.regPath)
	fmt.Println("[+] Registry file written to local storage(", r.Nodes, " nodes)")
	fmt.Println("[+] Transfering registry file to Master")
	a.master.CopyFiles(a.regPath)
	cmds := aws.MsterRun(a.binMasterPath, a.regPath, a.masterAddr, len(nodes))

	fmt.Println("[+] Master starting:")
	for i := 0; i < len(cmds); i++ {
		fmt.Println("       Exec:", i, cmds[i])
		_, err := a.master.Run(cmds[i])
		if err != nil {
			return err
		}
	}

	cmd := aws.MsterStart(a.binMasterPath, a.regPath, a.masterAddr, len(nodes))
	fmt.Println("       Exec:", len(cmds)+1, cmd)
	a.master.Start(cmd)

	var wg sync.WaitGroup
	for _, n := range nodes {
		wg.Add(1)
		go func(node *lib.Node) {
			nodeID := int(node.ID())
			slaveNode := a.slaveNodes[nodeID]

			cmds := aws.SlaveRun(a.binPath, a.confPath, a.regPath, a.masterAddr, slaveNode.Sync, "log.txt", nodeID, idx)

			//		fmt.Println("Slave CMD:  ", aws.CMDMapToString(cmds))
			if err := slaveNode.Init(); err != nil {
				panic(err)
			}

			for i := 0; i < len(cmds); i++ {
				fmt.Println("Running Slave", i, cmds[i])
				_, err := slaveNode.Run(cmds[i])
				if err != nil {
					panic(err)
				}
			}
			cmd := aws.SlaveStart(a.binPath, a.confPath, a.regPath, a.masterAddr, slaveNode.Sync, "log.txt", nodeID, idx)
			fmt.Println("Start Slave", cmd)
			slaveNode.Start(cmd)
			slaveNode.Close()
			wg.Done()
		}(n)
	}
	wg.Wait()
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
