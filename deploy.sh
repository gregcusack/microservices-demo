#!/bin/bash

eval $(minikube docker-env)

BASE_DIR="$( cd "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

SERVICES=(
    "adservice"
    "cartservice"
    "checkoutservice"
    "currencyservice"
    "emailservice"
    "frontend"
    "loadgenerator"
    "paymentservice"
    "productcatalogservice"
    "recommendationservice"
    "shippingservice"
)

for i in ${!SERVICES[@]};
do
    SERVICE=${SERVICES[$i]}
    
    cd $BASE_DIR/src/$SERVICE

    docker build . -t $SERVICE:latest
done

cd $BASE_DIR

TAG=latest skaffold run --default-repo=gregcusack$TAG


