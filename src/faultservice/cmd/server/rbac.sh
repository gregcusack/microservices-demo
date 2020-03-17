#!/bin/bash
kubectl create clusterrolebinding default-view --clusterrole=view --serviceaccount=default:default
