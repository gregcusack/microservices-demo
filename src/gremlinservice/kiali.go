package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var (
	ErrServiceNotFound = errors.New("service not found")
)

type KialiClient struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

func NewKialiClient(addr string) *KialiClient {
	k := readConfig(addr)
	resp, err := k.doRequest("/authenticate")
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		if err := k.Authenticate(); err != nil {
			panic(err)
		}
	}
	return k
}

func readConfig(host string) *KialiClient {
	defaultClient := KialiClient{
		Host:     fmt.Sprintf("http://%v/kiali/api", host),
		Username: "",
		Password: "",
		Token:    "",
	}

	// Check if file doesn't exist
	if _, err := os.Stat("config.json"); err != nil {
		saveConfig(defaultClient)
	}

	// Read from config file
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		panic(err)
	}

	// Unmarshal json
	if err := json.Unmarshal(data, &defaultClient); err != nil {
		panic(err)
	}
	return &defaultClient
}

func saveConfig(k KialiClient) {
	data, err := json.Marshal(k)
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile("config.json", data, os.FileMode(0644)); err != nil {
		panic(err)
	}
}

func (k *KialiClient) doRequest(url string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", k.Host+url, nil)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&http.Cookie{
		Name:  "kiali-token",
		Value: k.Token,
	})
	return client.Do(req)
}

func (k *KialiClient) Authenticate() error {
	client := &http.Client{}

	req, err := http.NewRequest("GET", k.Host+"/authenticate", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(k.Username, k.Password)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	k.Token = result["token"]

	saveConfig(*k)

	return nil
}

func (k *KialiClient) GetServices() (svcs []string, err error) {
	resp, err := k.doRequest("/namespaces/default/services")
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err = json.Unmarshal(body, &result); err != nil {
		return
	}

	for _, s := range result["services"].([]interface{}) {
		m := s.(map[string]interface{})
		svcs = append(svcs, m["name"].(string))
	}

	return
}

func (k *KialiClient) GetServiceGraph(svc string) (*Config, error) {
	resp, err := k.doRequest(fmt.Sprintf("/namespaces/default/services/%s/graph?duration=600s&graphType=workload&injectServiceNodes=true&appenders=deadNode", svc))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(body, &config)

	return &config, err
}

func (k *KialiClient) GetMeshOverview() (map[string]map[string]float64, error) {
	res := make(map[string]map[string]float64, 0)

	svcs, err := k.GetServices()
	if err != nil {
		return nil, err
	}

	for _, svc := range svcs {
		config, err := k.GetServiceGraph(svc)
		if err != nil {
			return nil, err
		}

		rates, err := func() (map[string]float64, error) {
			res := make(map[string]float64, 0)

			for _, n := range *config.Elements.Nodes {
				data := n.Data
				if *data.NodeType == "service" || *data.App == svc {
					continue
				}
				if data.Traffic != nil && len(*data.Traffic) > 0 {
					traffic := (*data.Traffic)[0].Rates.AdditionalProperties
					for key, value := range traffic {
						if !strings.Contains(key, "Out") {
							continue
						}
						rate, err := strconv.ParseFloat(value, 64)
						if err != nil {
							return nil, err
						}
						res[*data.App] = rate
					}
				}
			}

			return res, nil
		}()

		res[svc] = rates
	}

	return res, nil
}

func (k *KialiClient) GetAllTrafficRates() (map[string]float64, error) {
	res := make(map[string]float64, 0)

	svcs, err := k.GetServices()
	if err != nil {
		return nil, err
	}

	for _, svc := range svcs {
		config, err := k.GetServiceGraph(svc)
		if err != nil {
			return nil, err
		}
		rate, err := func() (float64, error) {
			for _, n := range *config.Elements.Nodes {
				data := n.Data
				if *data.NodeType == "service" {
					if data.Traffic != nil && len(*data.Traffic) > 0 {
						traffic := (*data.Traffic)[0].Rates.AdditionalProperties
						for key, value := range traffic {
							if !strings.Contains(key, "In") {
								continue
							}
							return strconv.ParseFloat(value, 64)
						}
					}
				}
			}
			return 0, ErrServiceNotFound
		}()
		if err != nil {
			continue
		}
		res[svc] = rate
	}

	return res, nil
}

func (c Config) GoString() string {
	s := strings.Builder{}

	ids := make(map[string]string, 0)

	s.WriteString("Nodes:\n")
	for _, n := range *c.Elements.Nodes {
		data := n.Data
		if *data.NodeType == "service" {
			ids[*data.Id] = *data.Service
		} else {
			ids[*data.Id] = *data.App
		}
		s.WriteString(fmt.Sprintf("id: %v\n", *data.Id))
		s.WriteString(fmt.Sprintf("nodeType: %v\n", *data.NodeType))
		s.WriteString(fmt.Sprintf("app: %v\n", *data.App))
		s.WriteString("Traffic:\n")
		for _, t := range *data.Traffic {
			s.WriteString(fmt.Sprintf("rates: %v\n", *t.Rates))
		}
		s.WriteString("\n")
	}

	s.WriteString("Edges:\n")
	for _, e := range *c.Elements.Edges {
		data := e.Data
		source := ids[*data.Source]
		target := ids[*data.Target]
		if source == target {
			continue
		}
		s := strings.Builder{}
		s.WriteString(fmt.Sprintf("id: %v\n", *data.Id))
		s.WriteString(fmt.Sprintf("source: %v\n", source))
		s.WriteString(fmt.Sprintf("target: %v\n", target))
		s.WriteString(fmt.Sprintf("Traffic rates: %v\n", *data.Traffic.Rates))
		s.WriteString("\n")
	}
	return s.String()
}
