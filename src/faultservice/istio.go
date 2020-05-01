package main

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	istio "istio.io/client-go/pkg/clientset/versioned"
)

// IstioClient is a wrapper for istio client
type IstioClient struct {
	ic *istio.Clientset
}

// NewIstioClient creates a new client
func NewIstioClient(kubeconfig string) (*IstioClient, error) {
	var ic *istio.Clientset
	var err error

	if kubeconfig == "" {
		ic, err = setupInCluster()
	} else {
		ic, err = setupOutCluster(kubeconfig)
	}

	return &IstioClient{ic: ic}, err
}

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

// ApplyFaultInjection applies a 100% fault injection to the inputted service
func (c *IstioClient) ApplyFaultInjection(svc, uri string, percent float64) error {
	// remove .default suffix
	arr := strings.Split(svc, ".")
	svc = arr[0]

	_, err := c.ic.NetworkingV1alpha3().VirtualServices("default").Create(&v1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VirtualService",
			APIVersion: "networking.istio.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: svc,
		},
		Spec: networkingv1alpha3.VirtualService{
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
					Match: []*networkingv1alpha3.HTTPMatchRequest{
						{
							IgnoreUriCase: true,
							Uri: &networkingv1alpha3.StringMatch{
								MatchType: &networkingv1alpha3.StringMatch_Prefix{
									Prefix: uri,
								},
							},
						},
					},
					Fault: &networkingv1alpha3.HTTPFaultInjection{
						Abort: &networkingv1alpha3.HTTPFaultInjection_Abort{
							ErrorType: &networkingv1alpha3.HTTPFaultInjection_Abort_HttpStatus{HttpStatus: 500},
							Percentage: &networkingv1alpha3.Percent{
								Value: percent,
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
		},
	})
	return err
}

// DeleteFaultInjection deletes a virtual service with name from inputted string
func (c *IstioClient) DeleteFaultInjection(svc string) error {
	// remove .default suffix
	arr := strings.Split(svc, ".")
	svc = arr[0]

	deletePolicy := metav1.DeletePropagationForeground
	return c.ic.NetworkingV1alpha3().VirtualServices("default").Delete(svc, &metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
}
