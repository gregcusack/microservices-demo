package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// VirtualService represents istio CRD for virtual service
// Note: struct fields must be public in order for unmarshal to
// correctly populate the data.
type VirtualService struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string
	Metadata   Metadata
	Spec       Spec
}

// Metadata is k8s metadata
type Metadata struct {
	Name string
}

// Spec is istio spec for VirtualService
type Spec struct {
	Hosts []string
	HTTP  []HTTPRoute `yaml:"http"`
}

// HTTPRoute is an ordered list of route rules for HTTP traffic.
type HTTPRoute struct {
	Route []HTTPRouteDestination
	Fault HTTPFaultInjection `yaml:"fault,omitempty"`
}

// HTTPRouteDestination is a HTTP rule that can either redirect or forward (default) traffic
type HTTPRouteDestination struct {
	Destination Destination
}

// Destination as an HTTPRouteDestination
type Destination struct {
	Host string
}

// HTTPFaultInjection can be used to specify one or more faults to inject while forwarding
// HTTP requests to the destination specified in a route
type HTTPFaultInjection struct {
	Abort Abort
}

// Abort is type of HTTPFaultInjection
type Abort struct {
	HTTPStatus int32 `yaml:"httpStatus"`
	Percentage Percent
}

// Percent represents a float [0,100]
type Percent struct {
	Value float32
}

func createFaultInjection(svc string) (string, error) {
	vs := VirtualService{
		APIVersion: "networking.istio.io/v1alpha3",
		Kind:       "VirtualService",
		Metadata: Metadata{
			Name: svc,
		},
		Spec: Spec{
			Hosts: []string{svc},
			HTTP: []HTTPRoute{
				{
					Route: []HTTPRouteDestination{
						{
							Destination: Destination{
								Host: svc,
							},
						},
					},
					Fault: HTTPFaultInjection{
						Abort: Abort{
							HTTPStatus: 500,
							Percentage: Percent{
								Value: 100.0,
							},
						},
					},
				},
				{
					Route: []HTTPRouteDestination{
						{
							Destination: Destination{
								Host: svc,
							},
						},
					},
				},
			},
		},
	}

	out, err := yaml.Marshal(vs)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll("manifests", 0755); err != nil {
		return "", err
	}

	filename := filepath.Join("manifests", fmt.Sprintf("%v-fault.yml", svc))

	return filename, ioutil.WriteFile(filename, out, 0644)
}
