package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"gopkg.in/yaml.v2"
)

func processError(err error) {
	fmt.Println(err)
	os.Exit(2)
}

func readFile(cfg *Cluster) {
	f, err := os.Open("cluster.yaml")
	if err != nil {
		processError(err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		processError(err)
	}
}

func main() {

	var cfg Cluster
	readFile(&cfg)

	pulumi.Run(func(ctx *pulumi.Context) error {
		vpc, err := ec2.NewVpc(ctx, cfg.Vpc.Name, &ec2.VpcArgs{
			CidrBlock:       pulumi.String(cfg.Vpc.CidrBlock),
			InstanceTenancy: pulumi.String(cfg.Vpc.Tenancy),
			Tags: pulumi.StringMap{
				"Name": pulumi.String(cfg.Vpc.Name),
			},
		})
		if err != nil {
			return err
		}

		subnets := cfg.Vpc.Subnets

		pub, err := ec2.NewSubnet(ctx, subnets[0].Name, &ec2.SubnetArgs{
			VpcId:            pulumi.StringInput(vpc.ID()),
			CidrBlock:        pulumi.String(subnets[0].CidrBlock),
			AvailabilityZone: pulumi.String(subnets[0].Az),
			Tags: pulumi.StringMap{
				"Name": pulumi.String(subnets[0].Name),
				"AZ":   pulumi.String(subnets[0].Az),
				"VPC":  pulumi.String(cfg.Vpc.Name),
			},
		})
		if err != nil {
			return err
		}

		priv, err := ec2.NewSubnet(ctx, subnets[1].Name, &ec2.SubnetArgs{
			VpcId:            pulumi.StringInput(vpc.ID()),
			CidrBlock:        pulumi.String(subnets[1].CidrBlock),
			AvailabilityZone: pulumi.String(subnets[1].Az),
			Tags: pulumi.StringMap{
				"Name": pulumi.String(subnets[1].Name),
				"AZ":   pulumi.String(subnets[1].Az),
				"VPC":  pulumi.String(cfg.Vpc.Name),
			},
		})
		if err != nil {
			return err
		}

		infra, err := ec2.NewSubnet(ctx, "infra-"+cfg.Vpc.Name, &ec2.SubnetArgs{
			VpcId:            pulumi.StringInput(vpc.ID()),
			CidrBlock:        pulumi.String(cfg.Vpc.Infra.CidrBlock),
			AvailabilityZone: pulumi.String(cfg.Vpc.Infra.Az),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("infra-" + cfg.Vpc.Name),
				"AZ":   pulumi.String(cfg.Vpc.Infra.Az),
				"VPC":  pulumi.String(cfg.Vpc.Name),
			},
		})
		if err != nil {
			return err
		}

		eip, err := ec2.NewEip(ctx, "eip-"+cfg.Vpc.Name, &ec2.EipArgs{
			Vpc: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}

		_, err = ec2.NewNatGateway(ctx, "ntgw"+cfg.Vpc.Name, &ec2.NatGatewayArgs{
			AllocationId: pulumi.StringInput(eip.ID()),
			SubnetId:     pulumi.StringInput(infra.ID()),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("ntgw" + cfg.Vpc.Name),
			},
		})
		if err != nil {
			return err
		}

		itgw, err := ec2.NewInternetGateway(ctx, "igw"+cfg.Vpc.Name, &ec2.InternetGatewayArgs{
			VpcId: pulumi.StringInput(vpc.ID()),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("igw" + cfg.Vpc.Name),
				"VPC":  pulumi.String(cfg.Vpc.Name),
			},
		})
		if err != nil {
			return err
		}

		rtinfra, err := ec2.NewRouteTable(ctx, "routeTable-"+cfg.Vpc.Name, &ec2.RouteTableArgs{
			VpcId: pulumi.StringInput(vpc.ID()),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("routeTable-" + cfg.Vpc.Name),
			},
		})
		if err != nil {
			return err
		}

		rinfra, err := ec2.NewRoute(ctx, "route"+cfg.Vpc.Name, &ec2.RouteArgs{
			RouteTableId:         pulumi.StringInput(rtinfra.ID()),
			DestinationCidrBlock: pulumi.String("0.0.0.0/0"),
			EgressOnlyGatewayId:  pulumi.StringInput(itgw.ID()),
		})
		if err != nil {
			return err
		}

		_, err = ec2.NewRouteTableAssociation(ctx, "routeTableAssociation-"+cfg.Vpc.Name, &ec2.RouteTableAssociationArgs{
			SubnetId:     pulumi.StringInput(rinfra.ID()),
			RouteTableId: pulumi.StringInput(rtinfra.ID()),
		})
		if err != nil {
			return err
		}

		_, err = ec2.NewSecurityGroup(ctx, cfg.Vpc.Name+"-vpn-sg", &ec2.SecurityGroupArgs{
			Egress: ec2.SecurityGroupEgressArray{
				&ec2.SecurityGroupEgressArgs{
					FromPort: pulumi.Int(0),
					ToPort:   pulumi.Int(0),
					Protocol: pulumi.String("-1"),
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
					Description: pulumi.String("Global egress allow"),
				},
			},
			Ingress: ec2.SecurityGroupIngressArray{
				&ec2.SecurityGroupIngressArgs{
					FromPort: pulumi.Int(22),
					ToPort:   pulumi.Int(22),
					Protocol: pulumi.String("-1"),
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
					Description: pulumi.String("Global ICMP allow (ingress)"),
				},
				&ec2.SecurityGroupIngressArgs{
					FromPort: pulumi.Int(-1),
					ToPort:   pulumi.Int(-1),
					Protocol: pulumi.String("icmp"),
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
					Description: pulumi.String("Global SSH access allow (ingress)"),
				},
				&ec2.SecurityGroupIngressArgs{
					FromPort: pulumi.Int(-1),
					ToPort:   pulumi.Int(-1),
					Protocol: pulumi.String("-1"),
					CidrBlocks: pulumi.StringArray{
						pulumi.String(cfg.Security.Ingress),
					},
					Description: pulumi.String("Global internet access allow from cidr_blocks (ingress)"),
				},
			},
		})
		if err != nil {
			return err
		}

		// Create EKS IAM Roles
		eksRole, err := iam.NewRole(ctx, "iam-eks-role-"+cfg.Vpc.Name, &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
		    "Version": "2008-10-17",
		    "Statement": [{
		        "Sid": "",
		        "Effect": "Allow",
		        "Principal": {
		            "Service": "eks.amazonaws.com"
		        },
		        "Action": "sts:AssumeRole"
		    }]
		}`),
		})
		if err != nil {
			return err
		}
		eksPolicies := []string{
			"arn:aws:iam::aws:policy/AmazonEKSServicePolicy",
			"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
		}
		for i, eksPolicy := range eksPolicies {
			_, err := iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("rpa-%d-"+cfg.Vpc.Name, i), &iam.RolePolicyAttachmentArgs{
				PolicyArn: pulumi.String(eksPolicy),
				Role:      eksRole.Name,
			})
			if err != nil {
				return err
			}
		}

		// Create the EC2 NodeGroup Role
		nodeGroupRole, err := iam.NewRole(ctx, "nodegroup-iam-role-"+cfg.Vpc.Name, &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
		    "Version": "2012-10-17",
		    "Statement": [{
		        "Sid": "",
		        "Effect": "Allow",
		        "Principal": {
		            "Service": "ec2.amazonaws.com"
		        },
		        "Action": "sts:AssumeRole"
		    }]
		}`),
		})
		if err != nil {
			return err
		}
		nodeGroupPolicies := []string{
			"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
			"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
			"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
		}
		for i, nodeGroupPolicy := range nodeGroupPolicies {
			_, err := iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("ngpa-%d-"+cfg.Vpc.Name, i), &iam.RolePolicyAttachmentArgs{
				Role:      nodeGroupRole.Name,
				PolicyArn: pulumi.String(nodeGroupPolicy),
			})
			if err != nil {
				return err
			}
		}
		// Create a Security Group that we can use to actually connect to our cluster
		clusterSg, err := ec2.NewSecurityGroup(ctx, "pulumi-cfg-"+cfg.Vpc.Name, &ec2.SecurityGroupArgs{
			VpcId: pulumi.StringInput(vpc.ID()),
			Egress: ec2.SecurityGroupEgressArray{
				ec2.SecurityGroupEgressArgs{
					Protocol:   pulumi.String("-1"),
					FromPort:   pulumi.Int(0),
					ToPort:     pulumi.Int(0),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
			},
			Ingress: ec2.SecurityGroupIngressArray{
				ec2.SecurityGroupIngressArgs{
					Protocol:   pulumi.String("-1"),
					FromPort:   pulumi.Int(0),
					ToPort:     pulumi.Int(0),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
				ec2.SecurityGroupIngressArgs{
					Protocol:   pulumi.String("-1"),
					FromPort:   pulumi.Int(443),
					ToPort:     pulumi.Int(443),
					CidrBlocks: pulumi.StringArray{priv.CidrBlock, pub.CidrBlock},
				},
				ec2.SecurityGroupIngressArgs{
					Protocol:   pulumi.String("-1"),
					FromPort:   pulumi.Int(0),
					ToPort:     pulumi.Int(0),
					CidrBlocks: pulumi.StringArray{pulumi.String(cfg.Security.Ingress)},
				},
			},
		})
		if err != nil {
			return err
		}
		// Create EKS Cluster

		eksArgs := &eks.ClusterArgs{
			RoleArn: pulumi.StringInput(eksRole.Arn),
			VpcConfig: &eks.ClusterVpcConfigArgs{
				PublicAccessCidrs: pulumi.StringArray{
					pulumi.String(cfg.Security.Ingress),
					priv.CidrBlock, pub.CidrBlock,
				},
				SecurityGroupIds: pulumi.StringArray{clusterSg.ID()},
				SubnetIds:        pulumi.StringArray{pub.ID(), priv.ID()},
			},
		}

		eksCluster, err := eks.NewCluster(ctx, "eks-cluster-"+cfg.Vpc.Name, eksArgs)
		if err != nil {
			return err
		}

		nodegrp := cfg.Eks.NodeGroup

		for i := range nodegrp {
			_, err = eks.NewNodeGroup(ctx, "asg-"+nodegrp[i].Name, &eks.NodeGroupArgs{
				ClusterName:   eksCluster.Name,
				NodeGroupName: pulumi.String(nodegrp[i].Name),
				NodeRoleArn:   pulumi.StringInput(nodeGroupRole.Arn),
				SubnetIds:     pulumi.StringArray{priv.ID(), pub.ID()},
				ScalingConfig: &eks.NodeGroupScalingConfigArgs{
					DesiredSize: pulumi.Int(nodegrp[i].MinSize),
					MaxSize:     pulumi.Int(nodegrp[i].MaxSize),
					MinSize:     pulumi.Int(nodegrp[i].MinSize),
				},
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
}
