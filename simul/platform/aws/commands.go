package aws

import (
	"strconv"
	"strings"
)

type Commands struct {
	masterBinPath string
	slaveBinPath  string
	confPath      string
	regPath       string
	sharedDir     string
}

func NewCommands(masterBinPath, slaveBinPath, confPath, regPath string) Commands {
	sharedDir := "$HOME/sharedDir"
	return Commands{
		masterBinPath: masterBinPath,
		slaveBinPath:  slaveBinPath,
		confPath:      confPath,
		regPath:       regPath,
		sharedDir:     sharedDir,
	}
}

func (c Commands) ConfifureMaster() map[int]string {
	cmds := make(map[int]string)
	cmds[0] = "sudo apt-get install nfs-kernel-server"
	cmds[1] = "sudo service nfs-kernel-server start"
	cmds[2] = "mkdir -p " + c.sharedDir
	cmds[3] = "sudo chmod 777 /etc/exports"
	cmds[4] = "cat /etc/exports" // *(rw,no_subtree_check,no_root_squash,sync,insecure) > /etc/exports"
	cmds[5] = "cp " + c.masterBinPath + " " + c.sharedDir
	cmds[6] = "cp " + c.slaveBinPath + " " + c.sharedDir
	cmds[7] = "cp " + c.confPath + " " + c.sharedDir
	cmds[8] = "sudo service nfs-kernel-server reload"
	return cmds
}

func (c Commands) CpyToSharedDir() map[int]string {
	cmds := make(map[int]string)
	cmds[0] = "cp " + c.regPath + " " + c.sharedDir
	cmds[1] = "chmod 777 " + c.masterBinPath
	return cmds
}

func (c Commands) MsterStart(masterAddr string, nbOfNodes int) string {
	return "nohup " + c.masterBinPath + " -masterAddr " + masterAddr + " -nbOfNodes " + strconv.Itoa(nbOfNodes) + " > log.txt"
}

/*
func MasterConfig(masterBin, slaveBin, confPath string) map[int]string {
	cmds := make(map[int]string)
	sharedDir := "$HOME/sharedDir"
	cmds[0] = "sudo apt-get install nfs-kernel-server"
	cmds[1] = "sudo service nfs-kernel-server start"
	cmds[2] = "mkdir -p " + sharedDir
	cmds[3] = "sudo chmod 777 /etc/exports"
	cmds[4] = "cat /etc/exports" // *(rw,no_subtree_check,no_root_squash,sync,insecure) > /etc/exports"
	cmds[5] = "cp " + masterBin + " " + sharedDir
	cmds[6] = "cp " + slaveBin + " " + sharedDir
	cmds[7] = "cp " + confPath + " " + sharedDir
	cmds[8] = "sudo service nfs-kernel-server reload"
	return cmds
}

func MsterRun(masterBin, regPath, masterAddr string, nbOfNodes int) map[int]string {
	sharedDir := "$HOME/sharedDir"
	cmds := make(map[int]string)
	cmds[0] = "cp " + regPath + " " + sharedDir
	cmds[1] = "chmod 777 " + masterBin
	return cmds
}

func MsterStart(masterBin, regPath, masterAddr string, nbOfNodes int) string {
	return "nohup " + masterBin + " -masterAddr " + masterAddr + " -nbOfNodes " + strconv.Itoa(nbOfNodes) + " > log.txt"

}*/

func SlaveConfig(masterAddr, masterBin, slaveBin, confPath string) map[int]string {
	cmds := make(map[int]string)
	sharedDir := "$HOME/sharedDir"
	cmds[0] = "mkdir -p " + sharedDir
	cmds[1] = "sudo apt-get -y install nfs-common"
	cmds[2] = "sudo mount -t nfs " + masterAddr + ":" + sharedDir + " " + sharedDir
	cmds[3] = "cp -r " + sharedDir + "/* " + "/tmp"
	return cmds
}

func SlaveRun(slaveBin, conf, reg, masterAddr, sync, log string, id, run int) map[int]string {
	sharedDir := "$HOME/sharedDir"
	cmds := make(map[int]string)

	cmds[0] = "cp " + sharedDir + "/aws.csv" + " /tmp"
	cmds[1] = "chmod 777 " + slaveBin
	//cmds[2] = "nohup " + slaveBin + " -config " + conf + " -registry " + reg + " -master " + masterAddr + " -id " + strconv.Itoa(id) + " -sync " + sync + " -run " + strconv.Itoa(run) + " > " + log
	return cmds
}

func SlaveStart(slaveBin, conf, reg, masterAddr, sync, log string, id, run int) string {
	return "nohup " + slaveBin + " -config " + conf + " -registry " + reg + " -master " + masterAddr + " -id " + strconv.Itoa(id) + " -sync " + sync + " -run " + strconv.Itoa(run) + " > " + log
}

func CMDMapToString(cmds map[int]string) string {
	c := make([]string, 0, len(cmds))
	for idx := 0; idx < len(cmds); idx++ {
		c = append(c, cmds[idx])
	}
	return strings.Join(c, " && ")
}
