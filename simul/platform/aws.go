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
	targetSystem  string
	targetArch    string
	user          string
	pemBytes      []byte
	master        aws.NodeController
	allSlaveNodes []aws.NodeController
	masterCMDS    aws.MasterCommands
	slaveCMDS     aws.SlaveCommands
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

	CMDS := aws.NewCommands("/tmp/masterAWS", "/tmp/nodeAWS", "/tmp/aws.conf", "/tmp/aws.csv")
	a.masterCMDS = aws.MasterCommands{CMDS}
	a.slaveCMDS = aws.SlaveCommands{CMDS}

	// Compile binaries
	a.pack("github.com/ConsenSys/handel/simul/node", c, CMDS.SlaveBinPath)
	a.pack("github.com/ConsenSys/handel/simul/master", c, CMDS.MasterBinPath)

	// write config
	if err := c.WriteTo(CMDS.ConfPath); err != nil {
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

	cons := c.NewConstructor()
	masterAddr := aws.GenRemoteAddress(*masterInstance.PublicIP, 5000)
	node := lib.GenerateNode(cons, -1, masterAddr)
	master, err := aws.NewSSHNodeContlorrer(node, a.pemBytes, a.user, "")

	if err != nil {
		return err
	}
	a.master = master

	for {
		err := master.Init()
		if err != nil {
			fmt.Println("Master Init failed, trying one more time", err, *masterInstance.ID, *masterInstance.PublicIP, *masterInstance.State)
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

	fmt.Println("[+] Transfering files to Master:", CMDS.MasterBinPath, CMDS.SlaveBinPath, CMDS.ConfPath)
	master.CopyFiles(CMDS.MasterBinPath, CMDS.SlaveBinPath, CMDS.ConfPath)
	cmds := a.masterCMDS.Configure()
	//	cmdStr := aws.CMDMapToString(cmds)

	fmt.Println("[+] Configuring Master")
	for idx := 0; idx < len(cmds); idx++ {
		fmt.Println("       Exec:", idx, cmds[idx])
		_, err := master.Run(cmds[idx])
		if err != nil {
			return err
		}
	}

	slaveCmds := a.slaveCMDS.Configure(*masterInstance.PublicIP)
	fmt.Println("")
	fmt.Println("")
	fmt.Println("[+] Configuring Slaves")

	addresses, syncs := aws.GenRemoteAddresses(slaveInstances)
	for i, addr := range addresses {
		node := lib.GenerateNode(cons, i, addr)
		slaveNodeController, err := aws.NewSSHNodeContlorrer(node, a.pemBytes, a.user, syncs[i])

		if err != nil {
			return err
		}
		slaveNodeController.Init()
		for idx := 0; idx < len(slaveCmds); idx++ {
			fmt.Println("     Slave ", i, "cmd: ", idx, slaveCmds[idx])
			_, err = slaveNodeController.Run(slaveCmds[idx])
			if err != nil {
				return err
			}
		}
		slaveNodeController.Close()
		a.allSlaveNodes = append(a.allSlaveNodes, slaveNodeController)
	}
	return nil
}

func (a *awsPlatform) Cleanup() error {
	return nil
}

func (a *awsPlatform) Start(idx int, r *lib.RunConfig) error {

	nbOfInstances := len(a.allSlaveNodes)
	if r.Nodes > nbOfInstances {
		msg := fmt.Sprintf(`Not enough EC2 instances, number of nodes to sart: %d
	               , number of avaliable EC2 instances: %d`, r.Nodes, nbOfInstances)
		return errors.New(msg)
	}
	slaveNodes := a.allSlaveNodes[0:r.Nodes]

	writeRegFile(slaveNodes, a.masterCMDS.RegPath)
	fmt.Println("[+] Registry file written to local storage(", r.Nodes, " nodes)")
	fmt.Println("[+] Transfering registry file to Master")
	a.master.CopyFiles(a.masterCMDS.RegPath)
	shareRegistryFile := a.masterCMDS.ShareRegistryFile()
	fmt.Println("[+] Master handel node:")
	for i := 0; i < len(shareRegistryFile); i++ {
		fmt.Println("       Exec:", i, shareRegistryFile[i])
		_, err := a.master.Run(shareRegistryFile[i])
		if err != nil {
			return err
		}
	}

	masterStart := a.masterCMDS.Start(a.master.Node().Address(), r.Nodes)
	fmt.Println("       Exec:", len(masterStart)+1, masterStart)
	a.master.Start(masterStart)

	var wg sync.WaitGroup
	for _, n := range slaveNodes {
		wg.Add(1)
		go func(slaveNode aws.NodeController) {

			cpyFiles := a.slaveCMDS.CopyRegistryFileFromSharedDirToLocalStorage()
			if err := slaveNode.Init(); err != nil {
				panic(err)
			}

			for i := 0; i < len(cpyFiles); i++ {
				fmt.Println("Running Slave", i, cpyFiles[i])
				_, err := slaveNode.Run(cpyFiles[i])
				if err != nil {
					panic(err)
				}
			}
			nodeID := int(slaveNode.Node().ID())
			startSlave := a.slaveCMDS.Start(a.master.Node().Address(), slaveNode.SyncAddr(), nodeID, idx)
			fmt.Println("Start Slave", startSlave)
			slaveNode.Start(startSlave)
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

func writeRegFile(slaves []aws.NodeController, regPath string) {
	parser := lib.NewCSVParser()
	var nodes []*lib.Node
	for _, slave := range slaves {
		nodes = append(nodes, slave.Node())
	}
	lib.WriteAll(nodes, parser, regPath)
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
