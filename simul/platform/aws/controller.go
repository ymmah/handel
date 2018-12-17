package aws

// NodeController represents avaliable operations to perform on a remote node
type NodeController interface {
	// CopyFiles copies files to equivalent location in a remote host
	// for example "/tmp/aws.csv" from localhost will be placed in
	// "/tmp/aws.csv" on the remote host
	CopyFiles(files []string) error
	// Run runs command on a remote node, for example Run("ls -l") and blocks until completion
	Run(command string) (string, error)
	// Start runs command on a remote node, doesn't block
	Start(command string) (string, error)
	// Init inits connection to the remote node
	Init() error
	// Close
	Close()
}
