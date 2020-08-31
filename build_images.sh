#!/bin/bash

BASE_DIR=$(dirname "$0")

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

    sudo docker build . -t $SERVICE:latest

done
