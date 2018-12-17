package aws

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SshCMD struct {
	client  *ssh.Client
	sshHost string
	config  *ssh.ClientConfig
	Sync    string
}

// NewSSHClient creates CMD backed by ssh
func NewSSHNodeContlorrer(pemBytes []byte, host string, user string, sync string) (*SshCMD, error) {
	sshHost, err := sshHostAddr(host)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return &SshCMD{sshHost: sshHost, config: config, Sync: sync}, nil
}

func (sshCMD *SshCMD) Init() error {
	conn, err := ssh.Dial("tcp", sshCMD.sshHost, sshCMD.config)
	if err != nil {
		return err
	}
	sshCMD.client = conn
	return nil
}

//CopyFiles cipies files from local to remote host using sftp
func (sshCMD *SshCMD) CopyFiles(files ...string) error {
	// create new SFTP client
	sftpClient, err := sftp.NewClient(sshCMD.client)
	if err != nil {
		return err
	}
	//defer sftpClient.Close()
	for _, file := range files {
		copyFile(sftpClient, file)
	}
	return nil
}

func copyFile(sftpClient *sftp.Client, file string) error {
	// create destination file
	dstFile, err := sftpClient.Create(file)

	if err != nil {
		return err
	}
	defer dstFile.Close()

	// create source file
	srcFile, err := os.Open(file)
	if err != nil {
		return err
	}

	// copy source file to destination file
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}

//Run runs command on a remote host using ssh and waits for output
func (sshCMD *SshCMD) Run(command string) (string, error) {
	//fmt.Println(">>>> Runnning >>>> ", command)
	var stdoutBuf bytes.Buffer
	session, err := sshCMD.client.NewSession()

	session.Stdout = &stdoutBuf
	if err != nil {
		return "", err
	}

	defer session.Close()

	err = session.Run(command)
	if err != nil {
		fmt.Println("SSH Run error ", err)
		return "", err
	}
	return stdoutBuf.String(), nil
}

//Run runs command on a remote host using ssh
func (sshCMD *SshCMD) Start(command string) (string, error) {
	//fmt.Println(">>>> Runnning >>>> ", command)
	var stdoutBuf bytes.Buffer
	session, err := sshCMD.client.NewSession()

	session.Stdout = &stdoutBuf
	if err != nil {
		return "", err
	}

	defer session.Close()

	err = session.Start(command)
	if err != nil {
		fmt.Println("ERRRR ", err)
		return "", err
	}
	fmt.Println("OUT: ", stdoutBuf.String())
	return "", nil
}

//Close closes ssh session
func (sshCMD *SshCMD) Close() {
	sshCMD.client.Close()
}

func sshHostAddr(addr string) (string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	newAddr := net.JoinHostPort(host, "22")
	return newAddr, nil
}
