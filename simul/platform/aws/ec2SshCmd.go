package aws

import (
	"fmt"
	"io"
	"net"
	"os"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type sshCMD struct {
	client  *ssh.Client
	session *ssh.Session
}

// NewSSHClient creates CMD backed by ssh
func NewSSHClient(pemBytes []byte, host string, user string) (CMD, error) {
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

	conn, err := ssh.Dial("tcp", sshHost, config)
	if err != nil {
		return nil, err
	}
	session, err := conn.NewSession()
	if err != nil {
		return nil, err
	}
	return &sshCMD{conn, session}, nil
}

//CopyFiles cipies files from local to remote host using sftp
func (sshCMD *sshCMD) CopyFiles(files []string) error {
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
func (sshCMD *sshCMD) Run(command string) (string, error) {
	fmt.Println(">>>> Runnning >>>> ", command)
	//var stdoutBuf bytes.Buffer
	//	sshCMD.session.Stdout = &stdoutBuf
	err := sshCMD.session.Run(command)
	if err != nil {
		fmt.Println("ERRRR")
		return "", err
	}
	//fmt.Println("OUT: ", stdoutBuf.String())
	return "", nil
}

//Run runs command on a remote host using ssh
func (sshCMD *sshCMD) Start(command string) (string, error) {
	fmt.Println(">>>> Runnning >>>> ", command)
	//var stdoutBuf bytes.Buffer
	//	sshCMD.session.Stdout = &stdoutBuf
	err := sshCMD.session.Start(command)
	if err != nil {
		fmt.Println("ERRRR")
		return "", err
	}
	//fmt.Println("OUT: ", stdoutBuf.String())
	return "", nil
}

//Close closes ssh session
func (sshCMD *sshCMD) Close() {
	sshCMD.session.Close()
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
