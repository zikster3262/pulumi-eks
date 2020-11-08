package main

// Cluster constructed as input for cluster.yaml
type Cluster struct {
	Vpc struct {
		Name               string `yaml:"name"`
		CidrBlock          string `yaml:"cidr_block"`
		Tenancy            string `yaml:"tenancy"`
		EnableDNSHostnames bool   `yaml:"enable_dns_hostnames"`
		Infra              struct {
			CidrBlock string `yaml:"cidr_block"`
			Az        string `yaml:"az"`
		} `yaml:"infra"`
		Subnets []struct {
			Name      string `yaml:"name"`
			CidrBlock string `yaml:"cidr_block"`
			Az        string `yaml:"az"`
		} `yaml:"subnets"`
	} `yaml:"vpc"`
	Eks struct {
		Name      string `yaml:"name"`
		NodeGroup []struct {
			Name    string `yaml:"name"`
			MinSize int    `yaml:"min_size"`
			MaxSize int    `yaml:"max_size"`
		} `yaml:"node_group"`
	} `yaml:"eks"`
	Security struct {
		Ingress string `yaml:"ingress"`
	} `yaml:"security"`
}
