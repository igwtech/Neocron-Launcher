package neocronapi

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultAPIBase = "http://api.neocron-game.com:8100"
	soapNS         = "http://schemas.xmlsoap.org/soap/envelope/"
	tempuriNS      = "http://tempuri.org/"
)

// Client communicates with the Neocron SOAP APIs.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	token      string
}

// NewClient creates a new API client.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultAPIBase
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Token returns the current session token.
func (c *Client) Token() string {
	return c.token
}

// SetToken sets the session token directly.
func (c *Client) SetToken(token string) {
	c.token = token
}

// --- SOAP helpers ---

func (c *Client) soapCall(endpoint, action, body string) ([]byte, error) {
	envelope := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="%s" xmlns:tem="%s">
  <soapenv:Header/>
  <soapenv:Body>%s</soapenv:Body>
</soapenv:Envelope>`, soapNS, tempuriNS, body)

	url := fmt.Sprintf("%s/%s", c.BaseURL, endpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(envelope))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", action)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SOAP call %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SOAP %s returned HTTP %d: %s", endpoint, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// extractElement extracts content between XML tags from raw SOAP response.
// This is simpler and more reliable than full namespace-aware XML parsing
// for the WCF-style responses Neocron uses.
func extractElement(data []byte, tag string) string {
	s := string(data)
	// Try with namespace prefix variants
	for _, prefix := range []string{"", "a:", "b:", "s:"} {
		open := "<" + prefix + tag + ">"
		close := "</" + prefix + tag + ">"
		start := strings.Index(s, open)
		if start == -1 {
			// Try with attributes
			open2 := "<" + prefix + tag + " "
			start = strings.Index(s, open2)
			if start != -1 {
				// Find the > after attributes
				gtPos := strings.Index(s[start:], ">")
				if gtPos == -1 {
					continue
				}
				start = start + gtPos + 1
				end := strings.Index(s[start:], close)
				if end == -1 {
					continue
				}
				return s[start : start+end]
			}
			continue
		}
		start += len(open)
		end := strings.Index(s[start:], close)
		if end == -1 {
			continue
		}
		return s[start : start+end]
	}
	return ""
}

func extractBool(data []byte, tag string) bool {
	v := extractElement(data, tag)
	return v == "true"
}

// --- SessionManagement API ---

// SessionDetails holds the response from a Login call.
type SessionDetails struct {
	RequestSucceeded bool   `json:"requestSucceeded"`
	ExceptionMessage string `json:"exceptionMessage,omitempty"`
	Token            string `json:"token"`
	Name             string `json:"name"`
	IsLoggedIn       bool   `json:"isLoggedIn"`
	IsGameMaster     bool   `json:"isGameMaster"`
	BackendVersion   string `json:"backendVersion"`
}

// Login authenticates with username and password.
func (c *Client) Login(user, password string) (*SessionDetails, error) {
	body := fmt.Sprintf(`<tem:Login>
      <tem:user>%s</tem:user>
      <tem:password>%s</tem:password>
    </tem:Login>`, xmlEscape(user), xmlEscape(password))

	resp, err := c.soapCall("SessionManagement",
		tempuriNS+"ISessionManagementServiceContract/Login", body)
	if err != nil {
		return nil, err
	}

	details := &SessionDetails{
		RequestSucceeded: extractBool(resp, "RequestSucceeded"),
		ExceptionMessage: extractElement(resp, "ExceptionMessage"),
		Token:            extractElement(resp, "Token"),
		Name:             extractElement(resp, "Name"),
		IsLoggedIn:       extractBool(resp, "IsLoggedIn"),
		IsGameMaster:     extractBool(resp, "IsGameMaster"),
		BackendVersion:   extractElement(resp, "BackendVersion"),
	}

	if details.RequestSucceeded && details.Token != "" {
		c.token = details.Token
	}

	return details, nil
}

// IsSessionValid checks if the current session token is still valid.
func (c *Client) IsSessionValid() (bool, error) {
	if c.token == "" {
		return false, nil
	}

	body := fmt.Sprintf(`<tem:IsSessionValid>
      <tem:token>%s</tem:token>
    </tem:IsSessionValid>`, c.token)

	resp, err := c.soapCall("SessionManagement",
		tempuriNS+"ISessionManagementServiceContract/IsSessionValid", body)
	if err != nil {
		return false, err
	}

	return extractBool(resp, "Value"), nil
}

// RefreshSession refreshes the current session.
func (c *Client) RefreshSession() (*SessionDetails, error) {
	if c.token == "" {
		return nil, fmt.Errorf("no active session")
	}

	body := fmt.Sprintf(`<tem:RefreshSession>
      <tem:token>%s</tem:token>
    </tem:RefreshSession>`, c.token)

	resp, err := c.soapCall("SessionManagement",
		tempuriNS+"ISessionManagementServiceContract/RefreshSession", body)
	if err != nil {
		return nil, err
	}

	return &SessionDetails{
		RequestSucceeded: extractBool(resp, "RequestSucceeded"),
		Token:            extractElement(resp, "Token"),
		Name:             extractElement(resp, "Name"),
		IsLoggedIn:       extractBool(resp, "IsLoggedIn"),
	}, nil
}

// Logout ends the current session.
func (c *Client) Logout() error {
	if c.token == "" {
		return nil
	}

	body := fmt.Sprintf(`<tem:Logout>
      <tem:token>%s</tem:token>
    </tem:Logout>`, c.token)

	_, err := c.soapCall("SessionManagement",
		tempuriNS+"ISessionManagementServiceContract/Logout", body)
	if err == nil {
		c.token = ""
	}
	return err
}

// --- LauncherInterface API ---

// Application represents a game application from the launcher API.
type Application struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Key         string `json:"key"`
	Executable  string `json:"executable"`
	Endpoint    string `json:"endpoint"`
	UpdateURI   string `json:"updateUri"`
	Server      string `json:"server"`
	Type        string `json:"type"`
	NewsFeedURL string `json:"newsFeedUrl"`
}

// Endpoint represents a server endpoint from the launcher API.
type Endpoint struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Address     string `json:"endpoint"`
	ServerFlags int    `json:"serverFlags"`
}

// GetAvailableApplications fetches the list of available game applications.
func (c *Client) GetAvailableApplications() ([]Application, error) {
	token := c.token
	if token == "" {
		token = "00000000-0000-0000-0000-000000000000"
	}

	body := fmt.Sprintf(`<tem:GetAvailableApplications>
      <tem:token>%s</tem:token>
    </tem:GetAvailableApplications>`, token)

	resp, err := c.soapCall("LauncherInterface",
		tempuriNS+"ILauncherInterfaceContract/GetAvailableApplications", body)
	if err != nil {
		return nil, err
	}

	return parseApplications(resp), nil
}

// GetEndpoints fetches server endpoints for a given application endpoint name.
func (c *Client) GetEndpoints(endpointName string) ([]Endpoint, error) {
	token := c.token
	if token == "" {
		token = "00000000-0000-0000-0000-000000000000"
	}

	body := fmt.Sprintf(`<tem:GetEndpoints>
      <tem:token>%s</tem:token>
      <tem:endpoint>%s</tem:endpoint>
    </tem:GetEndpoints>`, token, xmlEscape(endpointName))

	resp, err := c.soapCall("LauncherInterface",
		tempuriNS+"ILauncherInterfaceContract/GetEndpoints", body)
	if err != nil {
		return nil, err
	}

	return parseEndpoints(resp), nil
}

// --- PublicInterface API ---

// ServerStatistics holds server statistics.
type ServerStatistics struct {
	RequestSucceeded bool   `json:"requestSucceeded"`
	RawXML           string `json:"rawXml,omitempty"`
}

// GetServerStatistics fetches game server statistics.
func (c *Client) GetServerStatistics() (*ServerStatistics, error) {
	token := c.token
	if token == "" {
		token = "00000000-0000-0000-0000-000000000000"
	}

	body := fmt.Sprintf(`<tem:GetServerStatistics>
      <tem:token>%s</tem:token>
    </tem:GetServerStatistics>`, token)

	resp, err := c.soapCall("PublicInterface",
		tempuriNS+"IPublicInterfaceServiceContract/GetServerStatistics", body)
	if err != nil {
		return nil, err
	}

	return &ServerStatistics{
		RequestSucceeded: extractBool(resp, "RequestSucceeded"),
		RawXML:           string(resp),
	}, nil
}

// --- XML parsing helpers ---

func parseApplications(data []byte) []Application {
	s := string(data)
	var apps []Application

	// Split on ApplicationConfiguration elements
	parts := strings.Split(s, "<ApplicationConfiguration>")
	if len(parts) <= 1 {
		// Try with namespace prefix
		parts = strings.Split(s, ":ApplicationConfiguration>")
	}

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		partBytes := []byte(part)
		app := Application{
			Name:        extractElement(partBytes, "Name"),
			Description: extractElement(partBytes, "Description"),
			Key:         extractElement(partBytes, "Key"),
			Executable:  extractElement(partBytes, "Executable"),
			Endpoint:    extractElement(partBytes, "Endpoint"),
			UpdateURI:   extractElement(partBytes, "UpdateUri"),
			Server:      extractElement(partBytes, "Server"),
			Type:        extractElement(partBytes, "Type"),
			NewsFeedURL: extractElement(partBytes, "NewsFeedUrl"),
		}
		if app.Name != "" {
			apps = append(apps, app)
		}
	}

	return apps
}

func parseEndpoints(data []byte) []Endpoint {
	s := string(data)
	var endpoints []Endpoint

	parts := strings.Split(s, "<EndpointDescription>")
	if len(parts) <= 1 {
		parts = strings.Split(s, ":EndpointDescription>")
	}

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		partBytes := []byte(part)
		ep := Endpoint{
			Name:        extractElement(partBytes, "Name"),
			Description: extractElement(partBytes, "Description"),
			Address:     extractElement(partBytes, "Endpoint"),
		}
		if ep.Name != "" || ep.Address != "" {
			endpoints = append(endpoints, ep)
		}
	}

	return endpoints
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	xml.Escape(&b, []byte(s))
	return b.String()
}
