package kaytu

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var ErrLogin = errors.New("your session is expired, please login")

func PodRequest(reqBody KubernetesPodWastageRequest, token string) (*KubernetesPodWastageResponse, error) {
	payloadEncoded, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", "https://api.kaytu.io/kaytu/wastage/api/v1/wastage/kubernetes-pod", bytes.NewBuffer(payloadEncoded))
	if err != nil {
		return nil, fmt.Errorf("[pods]: %v", err)
	}
	req.Header.Add("content-type", "application/json")
	if len(token) > 0 {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[pods]: %v", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("[pods]: %v", err)
	}
	err = res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("[pods]: %v", err)
	}

	if res.StatusCode == 401 {
		return nil, ErrLogin
	}

	if res.StatusCode >= 300 || res.StatusCode < 200 {
		return nil, fmt.Errorf("server returned status code %d, [pods]: %s", res.StatusCode, string(body))
	}

	response := KubernetesPodWastageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("[pods]: %v", err)
	}
	return &response, nil
}

func ConfigurationRequest() (*Configuration, error) {
	req, err := http.NewRequest("POST", "https://api.kaytu.io/kaytu/wastage/api/v1/wastage/configuration", nil)
	if err != nil {
		return nil, fmt.Errorf("[ConfigurationRequest]: %v", err)
	}
	req.Header.Add("content-type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[ConfigurationRequest]: %v", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("[ConfigurationRequest]: %v", err)
	}
	err = res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("[ConfigurationRequest]: %v", err)
	}

	if res.StatusCode == 401 {
		return nil, ErrLogin
	}

	if res.StatusCode >= 300 || res.StatusCode < 200 {
		return nil, fmt.Errorf("server returned status code %d, [ConfigurationRequest]: %s", res.StatusCode, string(body))
	}

	response := Configuration{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("[ConfigurationRequest]: %v", err)
	}
	return &response, nil
}
