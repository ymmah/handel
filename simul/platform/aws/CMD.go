package aws

type AST interface {
}

type Value struct {
	command string
}

type AND struct {
	left AST
	rigt AST
}

type TransferFilesToMaster struct {
	src string
	dst string
}

type CopyFromNFS struct {
	masterDir string
	slaveDir  string
}

/*
sudo apt-get install nfs-kernel-server
sudo service nfs-kernel-server start

/home/ubuntu *(rw,no_subtree_check,no_root_squash,sync,insecure)
/sudo service nfs-kernel-server reload*/
func X() {
	_ =
		//Transfer
		AND{Value{"sudo apt-get install nfs-kernel-server"},
			AND{Value{"sudo service nfs-kernel-server start"},
				AND{Value{"home/ubuntu *(rw,no_subtree_check,no_root_squash,sync,insecure)"},
					Value{"sudo service nfs-kernel-server reload"}}}}
	//Start
}

type AWSCmd struct {
}

/*
func Y() {
	cmds := make(map[int]string)
	//********Master
	//TransferFile to tmp
	sharedDir = /ubuntu/sharedDir
	cmds[1]:="sudo apt-get install nfs-kernel-server"
	cmds[2]:="sudo service nfs-kernel-server start"
	cmds[3]:=sharedDir+" *(rw,no_subtree_check,no_root_squash,sync,insecure)"
	cmds[4]:="sudo service nfs-kernel-server reload"
	cmds[5]:="chmod 777 " + a.binMasterPath
	cmds[6]:="nohup " + a.binMasterPath + " -masterAddr " + masterAddr + " -nbOfNodes " + strconv.Itoa(len(slaveInstances))
	cpy  tmp => sharedDir

	//*******Slave
  masterAddr:=35.165.223.199
	cmds[1]:="mkdir" sharedDir
	cmds[2]:="sudo apt-get install nfs-common"
	cmds[3]:="sudo mount -t nfs"+ masterAddr+":/"+sharedDir +" "+ sharedDir
	cmds[4]:= cpy sharedDir => tmp
	cmds[5]:=	a.binPath + " -config "+ a.confPath " -registry "+a.regPath+" -master "+masterAddr

}
*/
func interpteretr(ast AST, cmd CMD) {
	switch v := ast.(type) {
	case Value:
		cmd.Start(v.command)
	case TransferFilesToMaster:
		//	cmd.CopyFiles(v.dst)
	case CopyFromNFS:

	case AND:
		interpteretr(v.left, cmd)
		interpteretr(v.rigt, cmd)
	}

}

// CMD represents avaliable operations to perform on a remote host
type CMD interface {
	// CopyFiles copies files to equivalent location in a remote host
	// for example "/tmp/aws.csv" from localhost will be placed in
	// "/tmp/aws.csv" on the remote host
	CopyFiles(files []string) error
	// Run runs commands, for example Run("ls -l")
	Run(command string) (string, error)
	Start(command string) (string, error)
	// Close
	Close()
}
