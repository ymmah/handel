package aws

type multiRegionAWSManager struct {
	managers []Manager
}

//NewMultiRegionAWSManager creates AWS manager for list of regions
func NewMultiRegionAWSManager(regions []string) Manager {
	var managers []Manager
	for _, reg := range regions {
		managers = append(managers, NewAWS(reg))
	}
	return &multiRegionAWSManager{managers}
}

//Instances returns instances from all regions
func (a *multiRegionAWSManager) Instances() []Instance {
	var instances []Instance
	for _, m := range a.managers {
		instances = append(instances, m.Instances()...)
	}
	return instances
}

func (a *multiRegionAWSManager) StartInstances() error {
	for _, m := range a.managers {
		err := m.StartInstances()
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *multiRegionAWSManager) StopInstances() error {
	for _, m := range a.managers {
		err := m.StopInstances()
		if err != nil {
			return err
		}
	}
	return nil
}
