package digest_auth_client

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

type DigestRequest struct {
	Body          string
	Method        string
	Password      string
	Uri           string
	Username      string
	Auth          *authorization
	Wa            *wwwAuthenticate
	ContentType   string
	SkipTLSVerify bool
}

func NewRequest(username string, password string, method string, uri string, body string) DigestRequest {

	dr := DigestRequest{}
	dr.UpdateRequest(username, password, method, uri, body)
	return dr
}

func (dr *DigestRequest) UpdateRequest(username string,
	password string, method string, uri string, body string) *DigestRequest {

	dr.Body = body
	dr.Method = method
	dr.Password = password
	dr.Uri = uri
	dr.Username = username
	return dr
}

func (dr *DigestRequest) generateClient(timeout time.Duration) *http.Client {
	if dr.SkipTLSVerify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}

		return &http.Client{
			Timeout:   timeout,
			Transport: tr,
		}
	}

	return &http.Client{Timeout: timeout}
}

func (dr *DigestRequest) Execute() (resp *http.Response, err error) {

	if dr.Auth == nil {
		var req *http.Request
		if req, err = http.NewRequest(dr.Method, dr.Uri, bytes.NewReader([]byte(dr.Body))); err != nil {
			return nil, err
		}

		if dr.ContentType != "" {
			req.Header.Set("Content-Type", dr.ContentType)
		}

		client := dr.generateClient(30 * time.Second)
		resp, err = client.Do(req)

		if resp.StatusCode == 401 {
			return dr.executeNewDigest(resp)
		}
		return
	}

	return dr.executeExistingDigest()
}

func (dr *DigestRequest) executeNewDigest(resp *http.Response) (*http.Response, error) {
	var (
		auth *authorization
		err  error
		wa   *wwwAuthenticate
	)

	waString := resp.Header.Get("WWW-Authenticate")
	if waString == "" {
		return nil, fmt.Errorf("Failed to get WWW-Authenticate header, please check your server configuration.")
	}
	wa = newWwwAuthenticate(waString)
	dr.Wa = wa

	if auth, err = newAuthorization(dr); err != nil {
		return nil, err
	}
	authString := auth.toString()

	if resp, err := dr.executeRequest(authString); err != nil {
		return nil, err
	} else {
		dr.Auth = auth
		return resp, nil
	}
}

func (dr *DigestRequest) executeExistingDigest() (*http.Response, error) {
	var (
		auth *authorization
		err  error
	)

	if auth, err = dr.Auth.refreshAuthorization(dr); err != nil {
		return nil, err
	}
	dr.Auth = auth

	authString := dr.Auth.toString()
	return dr.executeRequest(authString)
}

func (dr *DigestRequest) executeRequest(authString string) (*http.Response, error) {
	var (
		err error
		req *http.Request
	)

	if req, err = http.NewRequest(dr.Method, dr.Uri, bytes.NewReader([]byte(dr.Body))); err != nil {
		return nil, err
	}

	// fmt.Printf("AUTHSTRING: %s\n\n", authString)
	req.Header.Add("Authorization", authString)

	if dr.ContentType != "" {
		req.Header.Set("Content-Type", dr.ContentType)
	}

	client := dr.generateClient(30 * time.Second)

	return client.Do(req)
}
