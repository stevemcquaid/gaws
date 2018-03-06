# Oneliner
Oneliner to create an EC2 instance running the default page of a web application.

The one liner calls a longer script. AWS credentials for a dedicated region are provided. The web application is a Flask app, but not default nginx or apache

The pre-requisite configs for AWS credentials, as well as the amiID for the right region are provided out of scope of this code . The idea is that before you hit enter, there are no compute instances running and after, you supply an IP address, and at that address is a working web application.

# Summary
I decided to develop everything in golang to explore AWS's golang SDK. While the route of provisioning everything using ansible or terraform would have been easier, a dockerized golang package would allow me the flexibility in the future to automate or adjust precise elements of the deployment should I want to expand on the project or run it under kubernetes for example.

The overall call stack is:
```bash
make -> bash -> Docker [ -> Golang ] -> AWS API
```

The golang scripts will create one instance, install docker, start the specified docker container on 

# Usage
### Oneliner
```bash
docker run -it --rm stevemcquaid/gaws:1.0 \
    gaws \
    --amiID="<my_amiid_for_amazon_linux> \
    --cidr="0.0.0.0/0" \
    --container="stevemcquaid/python-flask-docker-hello-world:latest" \
    --instanceType="t2.micro" \
    --instanceTagKey="Name" \
    --instanceTagValue="gaws" \
    --port=80 \
    --securityGroupName="gaws-SG" \
    --securityGroupDesc="Allow access to my docker container" \
    --vpcID="" \
    --AWS_REGION=<my_aws_region> \
    --AWS_ACCESS_KEY_ID="<my_aws_access_key>" \
    --AWS_SECRET_ACCESS_KEY="<my_aws_secret_access_key>"
```

### How help options
```bash
make help
```

### Build the container locally
```bash
make build
```

### How to avoid showing sensitive creds
Populate the aws.env file to avoid having to specify creds as flags or enter credentials on the CLI

```bash
echo "AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=SECRET
AWS_SECRET_ACCESS_KEY=longsecret" >> aws.env
```

### Build & Run the container locally (My development workflow)
With a populated aws.env file. Might have to modify the amiID though.

```bash
make run
```

### Raw run usage.
All parameters specified for the gaws tool are optional. If you have a valid aws.env, you can get away with just:
```bash
docker run -it --env-file aws.env --rm stevemcquaid/gaws:latest
```
