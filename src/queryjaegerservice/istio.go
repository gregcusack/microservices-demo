package main

import (
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	istio "istio.io/client-go/pkg/clientset/versioned"
)

func setupInCluster() (*istio.Clientset, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	// create the clientset
	ic, err := istio.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return ic, nil
}

func setupOutCluster(kubeconfig string) (*istio.Clientset, error) {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	// create the clientset
	ic, err := istio.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return ic, nil
}

func applyFaultInjection(ic *istio.Clientset, svc string) error {
	vs := networkingv1alpha3.VirtualService{
		Hosts: []string{svc},
		Http: []*networkingv1alpha3.HTTPRoute{
			{
				Route: []*networkingv1alpha3.HTTPRouteDestination{
					{
						Destination: &networkingv1alpha3.Destination{
							Host: svc,
						},
					},
				},
				Fault: &networkingv1alpha3.HTTPFaultInjection{
					Abort: &networkingv1alpha3.HTTPFaultInjection_Abort{
						ErrorType: &networkingv1alpha3.HTTPFaultInjection_Abort_HttpStatus{HttpStatus: 500},
						Percentage: &networkingv1alpha3.Percent{
							Value: 100.0,
						},
					},
				},
			},
			{
				Route: []*networkingv1alpha3.HTTPRouteDestination{
					{
						Destination: &networkingv1alpha3.Destination{
							Host: svc,
						},
					},
				},
			},
		},
	}

	_, err := ic.NetworkingV1alpha3().VirtualServices("default").Create(&v1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name: svc,
		},
		Spec: vs,
	})
	return err
}

func deleteFaultInjection(ic *istio.Clientset, svc string) error {
	deletePolicy := metav1.DeletePropagationForeground
	return ic.NetworkingV1alpha3().VirtualServices("default").Delete(svc, &metav1.DeleteOptions{
		PropagationPolicy:  &deletePolicy,
	})
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
