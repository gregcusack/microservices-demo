#!/bin/bash
docker stop envoy
docker rm envoy
docker build . -t envoy:grpc
docker run -d \
	--restart=always \
	--name envoy \
	-p [::]:80:80 \
	-p [::]:5000:5000 \
	envoy:grpc
