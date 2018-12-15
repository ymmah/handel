package aws

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
