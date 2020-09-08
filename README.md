# Hipster Shop: Cloud-Native Microservices Demo Application

This project contains a 10-tier microservices application. The application is a
web-based e-commerce app called **“Hipster Shop”** where users can browse items,
add them to the cart, and purchase them.

**Google uses this application to demonstrate use of technologies like
Kubernetes/GKE, Istio, and gRPC**. This application
works on any Kubernetes cluster (such as a local one), as well as Google
Kubernetes Engine. It’s **easy to deploy with little to no configuration**.

If you’re using this demo, please **★Star** this repository to show your interest!

## Screenshots

| Home Page                                                                                                         | Checkout Screen                                                                                                    |
| ----------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| [![Screenshot of store homepage](./docs/img/hipster-shop-frontend-1.png)](./docs/img/hipster-shop-frontend-1.png) | [![Screenshot of checkout screen](./docs/img/hipster-shop-frontend-2.png)](./docs/img/hipster-shop-frontend-2.png) |

## Service Architecture

**Hipster Shop** is composed of many microservices written in different
languages that talk to each other over gRPC.

[![Architecture of
microservices](./docs/img/architecture-diagram.png)](./docs/img/architecture-diagram.png)

Find **Protocol Buffers Descriptions** at the [`./pb` directory](./pb).

| Service                                              | Language      | Description                                                                                                                       |
| ---------------------------------------------------- | ------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| [frontend](./src/frontend)                           | Go            | Exposes an HTTP server to serve the website. Does not require signup/login and generates session IDs for all users automatically. |
| [cartservice](./src/cartservice)                     | Go            | Stores the items in the user's shopping cart in Redis and retrieves it.                                                           |
| [productcatalogservice](./src/productcatalogservice) | Go            | Provides the list of products from a JSON file and ability to search products and get individual products.                        |
| [currencyservice](./src/currencyservice)             | Go       | Converts one money amount to another currency. Uses real values fetched from European Central Bank. It's the highest QPS service. |
| [paymentservice](./src/paymentservice)               | Go       | Charges the given credit card info (mock) with the given amount and returns a transaction ID.                                     |
| [shippingservice](./src/shippingservice)             | Go            | Gives shipping cost estimates based on the shopping cart. Ships items to the given address (mock)                                 |
| [emailservice](./src/emailservice)                   | Go        | Sends users an order confirmation email (mock).                                                                                   |
| [checkoutservice](./src/checkoutservice)             | Go            | Retrieves user cart, prepares order and orchestrates the payment, shipping and the email notification.                            |
| [recommendationservice](./src/recommendationservice) | Go        | Recommends other products based on what's given in the cart.                                                                      |
| [adservice](./src/adservice)                         | Java          | Provides text ads based on given context words.                                                                                   |
| [loadgenerator](./src/loadgenerator)                 | Python/Locust | Continuously sends requests imitating realistic user shopping flows to the frontend.                                              |

## Features

- **[Kubernetes](https://kubernetes.io)/[GKE](https://cloud.google.com/kubernetes-engine/):**
  The app is designed to run on Kubernetes (both locally on "Docker for
  Desktop", as well as on the cloud with GKE).
- **[gRPC](https://grpc.io):** Microservices use a high volume of gRPC calls to
  communicate to each other.
- **[Istio](https://istio.io):** Application works on Istio service mesh.
- **[Skaffold](https://skaffold.dev):** Application
  is deployed to Kubernetes with a single command using Skaffold.
- **Synthetic Load Generation:** The application demo comes with a background
  job that creates realistic usage patterns on the website using
  [Locust](https://locust.io/) load generator.

## Installation

We offer the following installation methods:

1. **Running locally** (~20 minutes) You will build
   and deploy microservices images to a single-node Kubernetes cluster running
   on your development machine. There are two options to run a Kubernetes
   cluster locally for this demo:
   - [Minikube](https://github.com/kubernetes/minikube). Recommended for the
     Linux hosts (also supports Mac/Windows).
   - [Docker for Desktop](https://www.docker.com/products/docker-desktop).
     Recommended for Mac/Windows.
     
2. **Running on Google Kubernetes Engine (GKE)”** (~30 minutes) You will build,
   upload and deploy the container images to a Kubernetes cluster on Google
   Cloud.

### Option 2: Running on Google Kubernetes Engine (GKE)

1. Create a new project for [Google Kubernetes Engine](https://console.cloud.google.com/projectselector2/kubernetes)

2. Run `gcloud init` to configure the GCloud SDK.

3.  Create a Google Kubernetes Engine cluster and make sure `kubectl` is pointing
    to the cluster.

    ```sh
    gcloud services enable container.googleapis.com
    ```

    ```sh
    gcloud container clusters create demo --enable-autoupgrade \
        --enable-autoscaling --min-nodes=3 --max-nodes=10 --num-nodes=5 --zone=us-central1-a
    ```

    ```
    kubectl get nodes
    ```
4.  Enable Google Container Registry (GCR) on your GCP project and configure the
    `docker` CLI to authenticate to GCR:

    ```sh
    gcloud services enable containerregistry.googleapis.com
    ```

    ```sh
    gcloud auth configure-docker -q
    ```

5. Prepare the GKE cluster for `istio`.

   1. [Prepare GKE cluster for Istio](https://istio.io/latest/docs/setup/platform-setup/gke/). Skip the first step which sets up a new cluster.
    
6. Make sure you have `istio` running in your cluster already with `Jaeger` add-on.

   1. [Install and run Istio](https://istio.io/latest/docs/setup/getting-started/#install). Only follow up to the 'Install Istio' step. Don't deploy their sample application.
   2. [Install Jaeger](https://istio.io/latest/docs/ops/integrations/jaeger/#installation)

7. Run `deploy.sh` (first time will be slow, it can take ~20 minutes). 

   1. First, this script sets the Docker env to that of minikube. 
   2. Second, it builds all Docker images.
   3. Third, it will run skaffold to deploy the built Docker images to minikube.
   4. It will most likely encounter an error deploying the services due to timeout exception. Don't worry about this. It takes a bit for the services to start up in Kubernetes.
   
8.  Find the IP address of your application, then visit the application on your
    browser to confirm installation.

        kubectl get service frontend-external

### Option 1: Running locally

> 💡 Recommended if you're planning to develop the application or giving it a
> try on your local cluster.

1. Install tools to run a Kubernetes cluster locally:

   - kubectl (can be installed via `gcloud components install kubectl`)
   - Local Kubernetes cluster deployment tool:
        - [Minikube (recommended for
         Linux)](https://kubernetes.io/docs/setup/minikube/).
        - Docker for Desktop (recommended for Mac/Windows): It provides Kubernetes support as [noted
     here](https://docs.docker.com/docker-for-mac/kubernetes/).
   - [skaffold]( https://skaffold.dev/docs/install/) (ensure version ≥v0.20)

1. Launch the local Kubernetes cluster with one of the following tools:

    - Launch Minikube (tested with Ubuntu Linux). Please, ensure that the
       local Kubernetes cluster has at least:
        - 4 CPU's
        - 4.0 GiB memory

        To run a Kubernetes cluster with Minikube using the described configuration, please run:

    ```shell
    minikube start --cpus=4 --memory 4096
    ```
    
    - Launch “Docker for Desktop” (tested with Mac/Windows). Go to Preferences:
        - choose “Enable Kubernetes”,
        - set CPUs to at least 3, and Memory to at least 6.0 GiB
        - on the "Disk" tab, set at least 32 GB disk space

1. Run `kubectl get nodes` to verify you're connected to “Kubernetes on Docker”.

2. Make sure you have `istio` running in your cluster already with `Jaeger` add-on.

   1. [Install and run Istio](https://istio.io/latest/docs/setup/getting-started/#install). Only follow up to the 'Install Istio' step. Don't deploy their sample application.
   2. [Install Jaeger](https://istio.io/latest/docs/ops/integrations/jaeger/#installation)

3. Run `deploy.sh` (first time will be slow, it can take ~20 minutes). 

   1. First, this script sets the Docker env to that of minikube. 
   2. Second, it builds all Docker images.
   3. Third, it will run skaffold to deploy the built Docker images to minikube.
   4. It will most likely encounter an error deploying the services due to timeout exception. Don't worry about this. It takes a bit for the services to start up in Kubernetes.

4. Run `kubectl get pods` to verify the Pods are ready and running. 

5. To check out traces, run `istioctl dashboard jaeger`

6. To check out the application frontend:

   1. Run `kubectl get services | grep frontend` to get the frontend node port.
   2. Run `minikube ip` to get the ip address of your minikube cluster.
   3. Go to http://$MINIKUBE_IP:$FRONTEND_PORT in your browser to see Hipster Shop.

### Updating Services

When making code changes to a specifc service you need to rebuild the docker image. Then you have to delete the Kubernetes pod with that is running the service so that when it rescales the deployment, it grabs the new docker image.

### Cleanup

If you've deployed the application with `skaffold run` command, you can run
`skaffold delete` to clean up the deployed resources.

If you've deployed the application with `kubectl apply -f [...]`, you can
run `kubectl delete -f [...]` with the same argument to clean up the deployed
resources.

## Conferences featuring Hipster Shop

- [Google Cloud Next'18 London – Keynote](https://youtu.be/nIq2pkNcfEI?t=3071)
  showing Stackdriver Incident Response Management
- Google Cloud Next'18 SF
  - [Day 1 Keynote](https://youtu.be/vJ9OaAqfxo4?t=2416) showing GKE On-Prem
  - [Day 3 – Keynote](https://youtu.be/JQPOPV_VH5w?t=815) showing Stackdriver
    APM (Tracing, Code Search, Profiler, Google Cloud Build)
  - [Introduction to Service Management with Istio](https://www.youtube.com/watch?v=wCJrdKdD6UM&feature=youtu.be&t=586)
- [KubeCon EU 2019 - Reinventing Networking: A Deep Dive into Istio's Multicluster Gateways - Steve Dake, Independent](https://youtu.be/-t2BfT59zJA?t=982)

---

This is not an official Google project.
