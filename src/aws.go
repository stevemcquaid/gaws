package main

import (
	"errors"
	"fmt"
	"log"

	"encoding/base64"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// MaxUserDataSize Max Side for User Data
const MaxUserDataSize = 16384

// Create aws ec2 api client
func createClient(awsStruct *awsInput) *ec2.EC2 {
	return ec2.New(session.New(&aws.Config{Region: aws.String(awsStruct.AWSRegion)}))
}

func createEC2(client *ec2.EC2, amiID string, instanceType string, instanceTagKey string, instanceTagValue string, userdata string, securityGroupIDs []*string) ([]*ec2.Instance, error) {

	// Specify the details of the instance that you want to create.
	request := &ec2.RunInstancesInput{
		// An Amazon Linux AMI ID for t2.micro instances in the us-east-1 region
		ImageId:      aws.String(amiID),
		InstanceType: aws.String(instanceType),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
	}
	// Add Security Groups to instance request
	request.SecurityGroupIds = securityGroupIDs

	// Stupid nested resources required to add the tags to the instance request. Could be a oneliner, but verbosity is nice sometimes.
	tagSpecs := []*ec2.TagSpecification{}
	tagSpec := ec2.TagSpecification{}
	tags := []*ec2.Tag{}

	tag := ec2.Tag{}
	tag.Key = aws.String(instanceTagKey)
	tag.Value = aws.String(instanceTagValue)

	tags = append(tags, &tag)
	tagSpec.Tags = tags
	resourceType := "instance" // weird enum
	tagSpec.ResourceType = &resourceType
	tagSpecs = append(tagSpecs, &tagSpec)

	// Add the tags to the request
	request.TagSpecifications = tagSpecs

	// Check if userdata is too large
	d := []byte(userdata)
	if d != nil {
		if len(d) > MaxUserDataSize {
			msg := fmt.Sprintf("Instance UserData was too large (%d bytes)", len(d))
			check(errors.New(msg), msg)
		}
		// Attach userdata to instance request
		request.UserData = aws.String(base64.StdEncoding.EncodeToString(d))
	}

	// Do the stuff - create instance
	runResult, err := client.RunInstances(request)
	if err != nil {
		msg := fmt.Sprintf("Could not create instance")
		check(err, msg)
	}

	log.Printf("Created instance <%s>", *runResult.Instances[0].InstanceId)

	return runResult.Instances, nil
}

func createSecurityGroup(client *ec2.EC2, name string, desc string, vpcID string, port int, cidr string) ([]*ec2.SecurityGroup, error) {
	// Defensive programming logic stolen from aws-sdk-example-code
	if len(name) == 0 || len(desc) == 0 {
		// flag.PrintDefaults()
		msg := fmt.Sprintf("Group name and description required. ")
		check(errors.New(msg), msg)
	}

	// Retrieve the first VPCID in the account.
	// If the VPC ID wasn't provided, then retrieve the first one in the account.
	if len(vpcID) == 0 {
		// Get a list of VPCs so we can associate the group with the first VPC.
		result, err := client.DescribeVpcs(nil)
		if err != nil {
			check(err, "Unable to describe VPCs")
		}
		if len(result.Vpcs) == 0 {
			msg := fmt.Sprintf("No VPCs found to associate security group with.")
			check(errors.New(msg), msg)
		}
		vpcID = aws.StringValue(result.Vpcs[0].VpcId)
	}

	// Create the security group with the VPC, name and description.
	createRes, err := client.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(name),
		Description: aws.String(desc),
		VpcId:       aws.String(vpcID),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidVpcID.NotFound":
				msg := fmt.Sprintf("Unable to find VPC with ID %q.", vpcID)
				check(errors.New(msg), msg)
			case "InvalidGroup.Duplicate":
				log.Printf("Security group already exists: %s. Be careful as any changes to rules will not be applied.", name)
				names := []string{}
				result, _ := getGroups(client, append(names, name), "group-name")
				return result, nil
			}
		}
		msg := fmt.Sprintf("Unable to create security group %q, %v", name)
		check(errors.New(msg), msg)
	}
	log.Printf("Created security group: %s <%s> with VPC: <%s>.\n", name, aws.StringValue(createRes.GroupId), vpcID)

	// Add permissions to the security group
	_, err = client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupName: aws.String(name),
		IpPermissions: []*ec2.IpPermission{
			// Can use setters to simplify seting multiple values without the
			// needing to use aws.String or associated helper utilities.
			(&ec2.IpPermission{}).
				SetIpProtocol("tcp").
				SetFromPort(int64(port)).
				SetToPort(int64(port)).
				SetIpRanges([]*ec2.IpRange{
					{CidrIp: aws.String(cidr)},
				}),
		},
	})
	if err != nil {
		msg := fmt.Sprintf("Unable to set security group %q ingress, %v", name)
		check(errors.New(msg), msg)
	}

	log.Println("Successfully created security group ingress rules with port: ", port)

	// Get security group object as []*ec2.SecurityGroup
	names := []string{}
	result, err := getGroups(client, append(names, name), "group-name")
	check(err, "Unable to find group: "+name)

	return result, nil
}

func getEC2(client *ec2.EC2, tagKey string, tagValue string) ([]*ec2.Instance, error) {
	// Get all the instances that are running and have the tag that we created our instance with.
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:" + tagKey),
				Values: []*string{aws.String(tagValue)},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []*string{aws.String("running"), aws.String("pending")},
			},
		},
	}
	resp, err := client.DescribeInstances(params)
	if err != nil {
		fmt.Println("there was an error listing instances in", err.Error())
		log.Fatal(err.Error())
	}
	// Get the instance objects
	var instances []*ec2.Instance
	for idx, _ := range resp.Reservations {
		//log.Println(" > Reservation Id", *res.ReservationId, " Num Instances: ", len(res.Instances))
		for _, inst := range resp.Reservations[idx].Instances {
			instances = append(instances, inst)
			//fmt.Println(" - Instance ID: ", *inst.InstanceId)
		}
	}
	return instances, nil
}

// Return array of security group objects for given groups, by filter (group-name, group-id, tag:Name, etc)
func getGroups(client *ec2.EC2, names []string, filter string) ([]*ec2.SecurityGroup, error) {

	// get names as array of aws.String objects
	values := make([]*string, len(names))
	for i, name := range names {
		values[i] = aws.String(name)
	}

	// request params as filter for names
	params := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(filter),
				Values: values,
			},
		},
	}

	// send request
	resp, err := client.DescribeSecurityGroups(params)
	if err != nil {
		return nil, err
	}

	return resp.SecurityGroups, nil
}

// Util function to get an instance from an array of instances
func getEC2Instance(instances []*ec2.Instance, id string) (*ec2.Instance, error) {
	for _, i := range instances {
		if *i.InstanceId == id {
			return i, nil
		}
	}
	msg := fmt.Sprintf("Instance: %s not found.", id)
	return nil, errors.New(msg)
}
