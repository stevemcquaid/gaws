#!/bin/bash
set -ex

docker run -it --env-file aws.env --rm stevemcquaid/gaws:latest /bin/bash
