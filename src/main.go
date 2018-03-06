package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
)

// URL for service to get IP
const identURL = "http://v4.ident.me/"

// struct to pass around with params
type awsInput struct {
	AWSRegion          string
	AWSAccessKey       string
	AWSSecretAccessKey string
	Client             *ec2.EC2
	container          string
	amiID              string
	instanceType       string
	instanceTagKey     string
	instanceTagValue   string
	securityGroupName  string
	securityGroupDesc  string
	vpcID              string
	port               int
	cidr               string
}

// error handler
func check(e error, msg string) {
	if e != nil {
		log.Println(msg, e.Error())
		panic(e.Error())
	}
}

// Parge ALL the Arguments
func argParse() (*awsInput, error) {
	// AWS Creds are overridden if provided, if not, default to (docker) environment variables, if nothing exists then prompt user
	AWSRegionFlag := flag.String("AWS_REGION", os.Getenv("AWS_REGION"), "AWS Credentials: AWS_REGION")
	AWSAccessKeyFlag := flag.String("AWS_ACCESS_KEY_ID", os.Getenv("AWS_ACCESS_KEY_ID"), "AWS Credentials: AWS_ACCESS_KEY_ID")
	AWSSecretAccessKeyFlag := flag.String("AWS_SECRET_ACCESS_KEY", os.Getenv("AWS_SECRET_ACCESS_KEY"), "AWS Credentials: AWS_SECRET_ACCESS_KEY")

	// Optional flags for specifying deployment
	containerFlag := flag.String("container", "stevemcquaid/python-flask-docker-hello-world:latest", "Container to run")
	amiIDFlag := flag.String("amiID", "ami-97785bed", "amiID to provision")
	instanceTypeFlag := flag.String("instanceType", "t2.micro", "Size of instance to run")
	instanceTagKeyFlag := flag.String("instanceTagKey", "Name", "EC2 Tag Key")
	instanceTagValueFlag := flag.String("instanceTagValue", "gaws", "EC2 Tag Value")
	securityGroupNameFlag := flag.String("securityGroupName", "gaws-SG", "Name of Security Group")
	securityGroupDescFlag := flag.String("securityGroupDesc", "Allow access to my docker container", "Description of Security Group")
	vpcIDFlag := flag.String("vpcID", "", "VPC ID")
	portFlag := flag.Int("port", 80, "Port to allow access to container")
	cidrFlag := flag.String("cidr", getMyIP(identURL)+"/32", "Cidr in which to allow ingress traffic to container")

	// Do the parsing
	flag.Parse()

	// Store values in the struct
	aws := awsInput{}
	aws.container = *containerFlag
	aws.amiID = *amiIDFlag
	aws.instanceType = *instanceTypeFlag
	aws.instanceTagKey = *instanceTagKeyFlag
	aws.instanceTagValue = *instanceTagValueFlag
	aws.securityGroupName = *securityGroupNameFlag
	aws.securityGroupDesc = *securityGroupDescFlag
	aws.vpcID = *vpcIDFlag
	aws.port = *portFlag
	aws.cidr = *cidrFlag

	// Do the logic for aws creds. 1- CLI flag, 2- (docker) environment vars, 3- prompt user
	aws.AWSRegion = *AWSRegionFlag
	if *AWSRegionFlag == "" {
		aws.AWSRegion = readInput("Enter AWS_REGION: ")
	}
	os.Setenv("AWS_REGION", aws.AWSRegion)

	aws.AWSAccessKey = *AWSAccessKeyFlag
	if *AWSAccessKeyFlag == "" {
		aws.AWSAccessKey = readInput("Enter AWS_ACCESS_KEY_ID: ")
	}
	os.Setenv("AWS_ACCESS_KEY_ID", aws.AWSAccessKey)

	aws.AWSSecretAccessKey = *AWSSecretAccessKeyFlag
	if *AWSSecretAccessKeyFlag == "" {
		aws.AWSSecretAccessKey = readInput("Enter AWS_SECRET_ACCESS_KEY: ")
	}
	os.Setenv("AWS_SECRET_ACCESS_KEY", aws.AWSSecretAccessKey)

	return &aws, nil
}

// Get my external-facing IP as a string
func getMyIP(ident string) string {
	resp, err := http.Get(ident)
	check(err, "Could not get HTTP request")
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	check(err, "Could not read body of response")

	return string(body)
}

// Simple bash script to run on boot
func getUserData(container string, port int) string {
	// This will start the container on first boot,
	// but will not restart it in the event of reboot.
	// To start container on future runs, create an init.d file here
	ud := fmt.Sprintf(`#!/bin/bash
yum update -y
yum install -y docker
service docker start
chkconfig docker on
usermod -a -G docker ec2-user
docker run -it -d -p %d:%d --rm %s`, port, port, container)
	return ud
}

// Do the stuffs
func run() {
	log.Println("Running... ")
	aws, err := argParse()
	check(err, "Could not parse arguments.")

	// Init aws client
	aws.Client = createClient(aws)

	// Create Security Group
	groups, err := createSecurityGroup(aws.Client, aws.securityGroupName, aws.securityGroupDesc, aws.vpcID, aws.port, aws.cidr)
	check(err, "Could not createSecurityGroup. ")

	// Create array of security group ids
	securityGroupIDs := []*string{}
	for _, group := range groups {
		securityGroupIDs = append(securityGroupIDs, group.GroupId)
	}

	// Create EC2 Instance
	userdata := getUserData(aws.container, aws.port)
	instances := []*ec2.Instance{}
	instances, err = createEC2(aws.Client, aws.amiID, aws.instanceType, aws.instanceTagKey, aws.instanceTagValue, userdata, securityGroupIDs)
	check(err, "Could not createEC2. ")

	// Get id of instance created
	createdInstanceID := *instances[0].InstanceId
	//log.Println(createdInstanceID)

	// Get the instance object using the createdInstanceID
	ec2s, err := getEC2(aws.Client, aws.instanceTagKey, aws.instanceTagValue)
	instance, err := getEC2Instance(ec2s, createdInstanceID)
	log.Println("Waiting for AWS to assign PublicDNSName & IP ...")

	// Wait until instance is assigned IP + DNS
	for *instance.PublicDnsName == "" {
		// wait for instance to get a public dns name
		time.Sleep(500 * time.Millisecond)
		ec2s, err = getEC2(aws.Client, aws.instanceTagKey, aws.instanceTagValue)
		instance, err = getEC2Instance(ec2s, createdInstanceID)
	}
	check(err, "Could not find instance")

	log.Printf("Public DNS: http://%s:%d <--> Public IP: http://%s:%d", *instance.PublicDnsName, aws.port, *instance.PublicIpAddress, aws.port)
	//log.Println("Instances: ", instances)
}

// Used if no aws creds are provided as flags or env vars
func readInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	text, _ := reader.ReadString('\n')
	text = strings.Trim(text, "\r\n"+string(0))
	fmt.Println(text)

	return text
}

func main() {
	run()
	log.Printf("Finshed. Please wait for instances to boot and provision. ")
}
