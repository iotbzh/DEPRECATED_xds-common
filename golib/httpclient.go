package common

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// HTTPClient .
type HTTPClient struct {
	LoggerOut    io.Writer
	LoggerLevel  int
	LoggerPrefix string

	httpClient http.Client
	initDone   bool
	endpoint   string
	apikey     string
	username   string
	password   string
	id         string
	csrf       string
	conf       HTTPClientConfig
}

// HTTPClientConfig is used to config HTTPClient
type HTTPClientConfig struct {
	URLPrefix           string
	HeaderAPIKeyName    string
	Apikey              string
	HeaderClientKeyName string
	CsrfDisable         bool
	LogOut              io.Writer
	LogLevel            int
	LogPrefix           string
}

// Logger levels constants
const (
	HTTPLogLevelPanic   = 0
	HTTPLogLevelError   = 1
	HTTPLogLevelWarning = 2
	HTTPLogLevelInfo    = 3
	HTTPLogLevelDebug   = 4
)

// Inspired by syncthing/cmd/cli

const insecure = false

// HTTPNewClient creates a new HTTP client to deal with Syncthing
func HTTPNewClient(baseURL string, cfg HTTPClientConfig) (*HTTPClient, error) {

	// Create w new Http client
	httpClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecure,
			},
		},
	}

	lOut := cfg.LogOut
	if cfg.LogOut == nil {
		lOut = os.Stdout
	}
	client := HTTPClient{
		LoggerOut:    lOut,
		LoggerLevel:  cfg.LogLevel,
		LoggerPrefix: cfg.LogPrefix,

		httpClient: httpClient,
		initDone:   false,
		endpoint:   baseURL,
		apikey:     cfg.Apikey,
		conf:       cfg,
		/* TODO - add user + pwd support
		username:   c.GlobalString("username"),
		password:   c.GlobalString("password"),
		*/
	}

	if err := client.getCidAndCsrf(); err != nil {
		return &client, err
	}

	client.log(HTTPLogLevelDebug, "HTTP client url %s init Done", client.endpoint)
	client.initDone = true
	return &client, nil
}

// GetLogLevel Get a readable string representing the log level
func (c *HTTPClient) GetLogLevel() string {
	return c.logLevelToString(c.LoggerLevel)
}

// logLevelToString Convert an integer log level to string
func (c *HTTPClient) logLevelToString(lvl int) string {
	switch lvl {
	case HTTPLogLevelPanic:
		return "panic"
	case HTTPLogLevelError:
		return "error"
	case HTTPLogLevelWarning:
		return "warning"
	case HTTPLogLevelInfo:
		return "info"
	case HTTPLogLevelDebug:
		return "debug"
	}
	return "Unknown"
}

// SetLogLevel set the log level from a readable string
func (c *HTTPClient) SetLogLevel(lvl string) error {
	switch strings.ToLower(lvl) {
	case "panic":
		c.LoggerLevel = HTTPLogLevelPanic
	case "error":
		c.LoggerLevel = HTTPLogLevelError
	case "warn", "warning":
		c.LoggerLevel = HTTPLogLevelWarning
	case "info":
		c.LoggerLevel = HTTPLogLevelInfo
	case "debug":
		c.LoggerLevel = HTTPLogLevelDebug
	default:
		return fmt.Errorf("Unknown level")
	}
	return nil
}

func (c *HTTPClient) log(level int, format string, args ...interface{}) {
	if level > c.LoggerLevel {
		return
	}
	sLvl := strings.ToUpper(c.logLevelToString(level))
	fmt.Fprintf(c.LoggerOut, sLvl+": "+c.LoggerPrefix+format+"\n", args...)
}

// Send request to retrieve Client id and/or CSRF token
func (c *HTTPClient) getCidAndCsrf() error {
	// Don't use cid + csrf when apikey is set
	if c.apikey != "" {
		return nil
	}
	request, err := http.NewRequest("GET", c.endpoint, nil)
	if err != nil {
		return err
	}
	if _, err := c.handleRequest(request); err != nil {
		return err
	}
	if c.id == "" {
		return errors.New("Failed to get device ID")
	}
	if !c.conf.CsrfDisable && c.csrf == "" {
		return errors.New("Failed to get CSRF token")
	}
	return nil
}

// GetClientID returns the id
func (c *HTTPClient) GetClientID() string {
	return c.id
}

// formatURL Build full url by concatenating all parts
func (c *HTTPClient) formatURL(endURL string) string {
	url := c.endpoint
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += strings.TrimLeft(c.conf.URLPrefix, "/")
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return url + strings.TrimLeft(endURL, "/")
}

// HTTPGet Send a Get request to client and return an error object
func (c *HTTPClient) HTTPGet(url string, data *[]byte) error {
	_, err := c._HTTPRequest("GET", url, nil, data)
	return err
}

// HTTPGetWithRes Send a Get request to client and return both response and error
func (c *HTTPClient) HTTPGetWithRes(url string, data *[]byte) (*http.Response, error) {
	return c._HTTPRequest("GET", url, nil, data)
}

// HTTPPost Send a POST request to client and return an error object
func (c *HTTPClient) HTTPPost(url string, body string) error {
	_, err := c._HTTPRequest("POST", url, &body, nil)
	return err
}

// HTTPPostWithRes Send a POST request to client and return both response and error
func (c *HTTPClient) HTTPPostWithRes(url string, body string) (*http.Response, error) {
	return c._HTTPRequest("POST", url, &body, nil)
}

// HTTPPut Send a PUT request to client and return an error object
func (c *HTTPClient) HTTPPut(url string, body string) error {
	_, err := c._HTTPRequest("PUT", url, &body, nil)
	return err
}

// HTTPPutWithRes Send a PUT request to client and return both response and error
func (c *HTTPClient) HTTPPutWithRes(url string, body string) (*http.Response, error) {
	return c._HTTPRequest("PUT", url, &body, nil)
}

// HTTPDelete Send a DELETE request to client and return an error object
func (c *HTTPClient) HTTPDelete(url string) error {
	_, err := c._HTTPRequest("DELETE", url, nil, nil)
	return err
}

// HTTPDeleteWithRes Send a DELETE request to client and return both response and error
func (c *HTTPClient) HTTPDeleteWithRes(url string) (*http.Response, error) {
	return c._HTTPRequest("DELETE", url, nil, nil)
}

// _HTTPRequest Generic function that returns a new Request given a method, URL, and optional body and data.
func (c *HTTPClient) _HTTPRequest(method, url string, body *string, data *[]byte) (*http.Response, error) {
	if !c.initDone {
		if err := c.getCidAndCsrf(); err == nil {
			c.initDone = true
		}
	}

	var err error
	var request *http.Request
	if body != nil {
		request, err = http.NewRequest(method, c.formatURL(url), bytes.NewBufferString(*body))
	} else {
		request, err = http.NewRequest(method, c.formatURL(url), nil)
	}

	if err != nil {
		return nil, err
	}
	res, err := c.handleRequest(request)
	if err != nil {
		return res, err
	}
	if res.StatusCode != 200 {
		return res, errors.New(res.Status)
	}

	if data != nil {
		*data = c.ResponseToBArray(res)
	}

	return res, nil
}

// ResponseToBArray converts an Http response to a byte array
func (c *HTTPClient) ResponseToBArray(response *http.Response) []byte {
	defer response.Body.Close()
	bytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.log(HTTPLogLevelError, "ResponseToBArray failure: %v", err.Error())
	}
	return bytes
}

func (c *HTTPClient) handleRequest(request *http.Request) (*http.Response, error) {
	if c.conf.HeaderAPIKeyName != "" && c.apikey != "" {
		request.Header.Set(c.conf.HeaderAPIKeyName, c.apikey)
	}
	if c.conf.HeaderClientKeyName != "" && c.id != "" {
		request.Header.Set(c.conf.HeaderClientKeyName, c.id)
	}
	if c.username != "" || c.password != "" {
		request.SetBasicAuth(c.username, c.password)
	}
	if c.csrf != "" {
		request.Header.Set("X-CSRF-Token-"+c.id[:5], c.csrf)
	}

	c.log(HTTPLogLevelDebug, "HTTP %s %v", request.Method, request.URL)
	response, err := c.httpClient.Do(request)
	c.log(HTTPLogLevelDebug, "HTTP RESPONSE: %v\n", response)
	if err != nil {
		c.log(HTTPLogLevelInfo, "%v", err)
		return nil, err
	}

	// Detect client ID change
	cid := response.Header.Get(c.conf.HeaderClientKeyName)
	if cid != "" && c.id != cid {
		c.id = cid
	}

	// Detect CSR token change
	for _, item := range response.Cookies() {
		if c.id != "" && item.Name == "CSRF-Token-"+c.id[:5] {
			c.csrf = item.Value
			goto csrffound
		}
	}
	// OK CSRF found
csrffound:

	if response.StatusCode == 404 {
		return nil, errors.New("Invalid endpoint or API call")
	} else if response.StatusCode == 401 {
		return nil, errors.New("Invalid username or password")
	} else if response.StatusCode == 403 {
		if c.apikey == "" {
			// Request a new Csrf for next requests
			c.getCidAndCsrf()
			return nil, errors.New("Invalid CSRF token")
		}
		return nil, errors.New("Invalid API key")
	} else if response.StatusCode != 200 {
		data := make(map[string]interface{})
		// Try to decode error field of APIError struct
		json.Unmarshal(c.ResponseToBArray(response), &data)
		if err, found := data["error"]; found {
			return nil, fmt.Errorf(err.(string))
		}
		body := strings.TrimSpace(string(c.ResponseToBArray(response)))
		if body != "" {
			return nil, fmt.Errorf(body)
		}
		return nil, errors.New("Unknown HTTP status returned: " + response.Status)
	}
	return response, nil
}
