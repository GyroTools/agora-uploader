package agora

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"path"

	"github.com/sirupsen/logrus"
)

type ApiKeyResponse struct {
	ApiKey string `json:"key"`
}

func join_url(agora_url string, path_str string) string {
	u, _ := url.Parse(agora_url)
	u.Path = path.Join(u.Path, path_str)
	request_url := u.String()
	return request_url
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func HandleNoCertificateCheck(no_certificate_check bool) {
	if no_certificate_check {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
}

func GetRequest(request_url string, api_key string, user string, password string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", request_url, nil)
	if err != nil {
		return nil, err
	}

	if api_key != "" {
		req.Header.Set("Authorization", "X-Agora-Api-Key "+api_key)
	} else if user != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+basicAuth(user, password))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, err
}

func PostRequest(request_url string, body []byte, api_key string, user string, password string, content_type string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", request_url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	if api_key != "" {
		req.Header.Set("Authorization", "X-Agora-Api-Key "+api_key)
	} else if user != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+basicAuth(user, password))
	}
	if content_type != "" {
		req.Header.Set("Content-Type", content_type)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, err
}

func Ping(agora_url string) (bool, error) {
	request_url := join_url(agora_url, "/api/v1/version/")
	resp, err := GetRequest(request_url, "", "", "")
	return err == nil && resp.StatusCode == 200, err
}

func CheckConnection(agora_url string, apikey string) (bool, error) {
	request_url := join_url(agora_url, "/api/v1/user/current/")
	resp, err := GetRequest(request_url, apikey, "", "")
	return err == nil && resp.StatusCode == 200, err
}

func GetApiKey(agora_url string, user string, password string) string {
	success, err := Ping(agora_url)
	if !success {
		logrus.Fatal("Error: Cannot connect to the Agora server: ", err)
	}
	request_url := join_url(agora_url, "/api/v1/apikey/") + "/"
	resp, err := GetRequest(request_url, "", user, password)

	if err != nil {
		logrus.Fatal(err)
	}
	if resp.StatusCode == 404 {
		logrus.Fatal("No api-key found. Please create an api-key in your Agora user profile")
	} else if resp.StatusCode > 299 {
		logrus.Fatal("Could not get the api-key. http status = ", resp.StatusCode)
	}

	target := new(ApiKeyResponse)
	json.NewDecoder(resp.Body).Decode(target)
	return target.ApiKey
}
