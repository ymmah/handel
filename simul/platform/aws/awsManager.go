package aws

import (
	"fmt"
)

//Instance reprezents EC2 Amazon instance
type Instance struct {
	// EC2 ID
	ID *string
	// IP Visible to the outside world
	PublicIP *string
	// State: running, pending, stopped
	State *string
	//AWS region
	region string
	Tag    string
}

//Manager manages group of EC2 instances
type Manager interface {
	// Instances list avaliable instances in any state
	Instances() []Instance
	// StartInstances starts all avaliable instances
	RefreshInstances() ([]Instance, error)
	StartInstances() error
	// StopInstances stops all avaliable instances
	StopInstances() error
}

const base = 3000

// GenRemoteAddresses  generates n * 2 addresses: one for handel, one for the sync
func GenRemoteAddresses(instances []Instance) ([]string, []string) {
	n := len(instances)
	var addresses = make([]string, 0, n)
	var syncs = make([]string, 0, n)
	for _, i := range instances {
		addr1 := GenRemoteAddress(*i.PublicIP, base)
		addr2 := GenRemoteAddress(*i.PublicIP, base+1)
		addresses = append(addresses, addr1)
		syncs = append(syncs, addr2)
	}
	return addresses, syncs
}

func GenRemoteAddress(ip string, port int) string {
	addr := fmt.Sprintf("%s:%d", ip, port)
	return addr
}

func instanceToInstanceID(instances []Instance) []*string {
	var ids []*string
	for _, inst := range instances {
		ids = append(ids, inst.ID)
	}
	return ids
}

func WaitUntilAllInstancesRunning(a Manager, delay func()) (int, error) {
	allRunning := allInstancesRunning(a.Instances())
	if allRunning {
		return 0, nil
	}
	trais := 0

	for {
		trais++
		delay()
		fmt.Println("Waiting for amazon instances to start")

		allInstances, err := a.RefreshInstances()
		if err != nil {
			return trais, err
		}
		allRunning = allInstancesRunning(allInstances)
		if allRunning {
			return trais, nil
		}
	}
}

func allInstancesRunning(instances []Instance) bool {
	okInstances := 0
	for _, inst := range instances {
		if (*inst.State) == running {
			okInstances++
			if okInstances >= len(instances) {
				return true
			}
		}
	}
	return false
}
