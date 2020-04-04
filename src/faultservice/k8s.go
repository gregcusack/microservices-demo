package main

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type k8sClient struct {
	kc *kubernetes.Clientset
}

// NewK8sClient creates a new k8s client
func NewK8sClient(kubeconfig string) (*k8sClient, error) {
	// creates the in-cluster config
	if kubeconfig == "" {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		// creates the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}

		return &k8sClient{kc: clientset}, nil
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &k8sClient{kc: clientset}, nil
}

func (k *k8sClient) DeletePod(namespace, deployment string) error {
	pods, err := k.kc.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	var pod string
	for _, p := range pods.Items {
		if strings.HasPrefix(strings.ToLower(p.Name), deployment) {
			pod = p.Name
			break
		}
	}

	if pod == "" {
		return fmt.Errorf("could not find pod for deployment %v", deployment)
	}

	policy := metav1.DeletePropagationForeground
	return k.kc.CoreV1().Pods(namespace).Delete(pod, &metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
}
