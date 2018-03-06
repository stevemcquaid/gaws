#!/bin/bash
set -ex

docker run -it --env-file aws.env --rm stevemcquaid/gaws:latest \
    gaws \
    --container="stevemcquaid/python-flask-docker-hello-world:latest" \
    --amiID="ami-97785bed" \
    --instanceType="t2.micro" \
    --instanceTagKey="Name" \
    --instanceTagValue="gaws" \
    --securityGroupName="gaws-SG" \
    --securityGroupDesc="Allow access to my docker container" \
    --vpcID="" \
    --port=80 \
    --cidr="0.0.0.0/0"

