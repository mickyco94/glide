// Package remoteconfig provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v1.11.0 DO NOT EDIT.
package remoteconfig

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
)

// The configuration for a Common Fate deployment.
type DeploymentConfiguration struct {
	// Notifications configuration for the deployment.
	NotificationsConfiguration NotificationsConfiguration `json:"notificationsConfiguration"`

	// Configuration of all Access Providers.
	ProviderConfiguration ProviderMap `json:"providerConfiguration"`
}

// Notifications configuration for the deployment.
type NotificationsConfiguration struct {
	// The Slack notification configuration.
	Slack                 *SlackConfiguration           `json:"slack,omitempty"`
	SlackIncomingWebhooks *map[string]map[string]string `json:"slackIncomingWebhooks,omitempty"`
}

// Configuration settings for an individual Access Provider.
type ProviderConfiguration struct {
	Uses string            `json:"uses"`
	With map[string]string `json:"with"`
}

// Configuration of all Access Providers.
type ProviderMap struct {
	AdditionalProperties map[string]ProviderConfiguration `json:"-"`
}

// The Slack notification configuration.
type SlackConfiguration struct {
	// The Slack API token. Should be a reference to secret in `awsssm://` format.
	ApiToken string `json:"apiToken"`
}

// DeploymentConfigResponse defines model for DeploymentConfigResponse.
type DeploymentConfigResponse struct {
	// The configuration for a Common Fate deployment.
	DeploymentConfiguration DeploymentConfiguration `json:"deploymentConfiguration"`
}

// UpdateProvidersRequest defines model for UpdateProvidersRequest.
type UpdateProvidersRequest struct {
	// Configuration of all Access Providers.
	ProviderConfiguration ProviderMap `json:"providerConfiguration"`
}

// UpdateProviderConfigurationJSONRequestBody defines body for UpdateProviderConfiguration for application/json ContentType.
type UpdateProviderConfigurationJSONRequestBody UpdateProvidersRequest

// Getter for additional properties for ProviderMap. Returns the specified
// element and whether it was found
func (a ProviderMap) Get(fieldName string) (value ProviderConfiguration, found bool) {
	if a.AdditionalProperties != nil {
		value, found = a.AdditionalProperties[fieldName]
	}
	return
}

// Setter for additional properties for ProviderMap
func (a *ProviderMap) Set(fieldName string, value ProviderConfiguration) {
	if a.AdditionalProperties == nil {
		a.AdditionalProperties = make(map[string]ProviderConfiguration)
	}
	a.AdditionalProperties[fieldName] = value
}

// Override default JSON handling for ProviderMap to handle AdditionalProperties
func (a *ProviderMap) UnmarshalJSON(b []byte) error {
	object := make(map[string]json.RawMessage)
	err := json.Unmarshal(b, &object)
	if err != nil {
		return err
	}

	if len(object) != 0 {
		a.AdditionalProperties = make(map[string]ProviderConfiguration)
		for fieldName, fieldBuf := range object {
			var fieldVal ProviderConfiguration
			err := json.Unmarshal(fieldBuf, &fieldVal)
			if err != nil {
				return fmt.Errorf("error unmarshaling field %s: %w", fieldName, err)
			}
			a.AdditionalProperties[fieldName] = fieldVal
		}
	}
	return nil
}

// Override default JSON handling for ProviderMap to handle AdditionalProperties
func (a ProviderMap) MarshalJSON() ([]byte, error) {
	var err error
	object := make(map[string]json.RawMessage)

	for fieldName, field := range a.AdditionalProperties {
		object[fieldName], err = json.Marshal(field)
		if err != nil {
			return nil, fmt.Errorf("error marshaling '%s': %w", fieldName, err)
		}
	}
	return json.Marshal(object)
}

// RequestEditorFn  is the function signature for the RequestEditor callback function
type RequestEditorFn func(ctx context.Context, req *http.Request) error

// Doer performs HTTP requests.
//
// The standard http.Client implements this interface.
type HttpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client which conforms to the OpenAPI3 specification for this service.
type Client struct {
	// The endpoint of the server conforming to this interface, with scheme,
	// https://api.deepmap.com for example. This can contain a path relative
	// to the server, such as https://api.deepmap.com/dev-test, and all the
	// paths in the swagger spec will be appended to the server.
	Server string

	// Doer for performing requests, typically a *http.Client with any
	// customized settings, such as certificate chains.
	Client HttpRequestDoer

	// A list of callbacks for modifying requests which are generated before sending over
	// the network.
	RequestEditors []RequestEditorFn
}

// ClientOption allows setting custom parameters during construction
type ClientOption func(*Client) error

// Creates a new Client, with reasonable defaults
func NewClient(server string, opts ...ClientOption) (*Client, error) {
	// create a client with sane default values
	client := Client{
		Server: server,
	}
	// mutate client and add all optional params
	for _, o := range opts {
		if err := o(&client); err != nil {
			return nil, err
		}
	}
	// ensure the server URL always has a trailing slash
	if !strings.HasSuffix(client.Server, "/") {
		client.Server += "/"
	}
	// create httpClient, if not already present
	if client.Client == nil {
		client.Client = &http.Client{}
	}
	return &client, nil
}

// WithHTTPClient allows overriding the default Doer, which is
// automatically created using http.Client. This is useful for tests.
func WithHTTPClient(doer HttpRequestDoer) ClientOption {
	return func(c *Client) error {
		c.Client = doer
		return nil
	}
}

// WithRequestEditorFn allows setting up a callback function, which will be
// called right before sending the request. This can be used to mutate the request.
func WithRequestEditorFn(fn RequestEditorFn) ClientOption {
	return func(c *Client) error {
		c.RequestEditors = append(c.RequestEditors, fn)
		return nil
	}
}

// The interface specification for the client above.
type ClientInterface interface {
	// GetConfig request
	GetConfig(ctx context.Context, reqEditors ...RequestEditorFn) (*http.Response, error)

	// UpdateProviderConfiguration request with any body
	UpdateProviderConfigurationWithBody(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error)

	UpdateProviderConfiguration(ctx context.Context, body UpdateProviderConfigurationJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error)
}

func (c *Client) GetConfig(ctx context.Context, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewGetConfigRequest(c.Server)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func (c *Client) UpdateProviderConfigurationWithBody(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewUpdateProviderConfigurationRequestWithBody(c.Server, contentType, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func (c *Client) UpdateProviderConfiguration(ctx context.Context, body UpdateProviderConfigurationJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewUpdateProviderConfigurationRequest(c.Server, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewGetConfigRequest generates requests for GetConfig
func NewGetConfigRequest(server string) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/api/v1/config")
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// NewUpdateProviderConfigurationRequest calls the generic UpdateProviderConfiguration builder with application/json body
func NewUpdateProviderConfigurationRequest(server string, body UpdateProviderConfigurationJSONRequestBody) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewUpdateProviderConfigurationRequestWithBody(server, "application/json", bodyReader)
}

// NewUpdateProviderConfigurationRequestWithBody generates requests for UpdateProviderConfiguration with any type of body
func NewUpdateProviderConfigurationRequestWithBody(server string, contentType string, body io.Reader) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/api/v1/config/providers")
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)

	return req, nil
}

func (c *Client) applyEditors(ctx context.Context, req *http.Request, additionalEditors []RequestEditorFn) error {
	for _, r := range c.RequestEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	for _, r := range additionalEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

// ClientWithResponses builds on ClientInterface to offer response payloads
type ClientWithResponses struct {
	ClientInterface
}

// NewClientWithResponses creates a new ClientWithResponses, which wraps
// Client with return type handling
func NewClientWithResponses(server string, opts ...ClientOption) (*ClientWithResponses, error) {
	client, err := NewClient(server, opts...)
	if err != nil {
		return nil, err
	}
	return &ClientWithResponses{client}, nil
}

// WithBaseURL overrides the baseURL.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) error {
		newBaseURL, err := url.Parse(baseURL)
		if err != nil {
			return err
		}
		c.Server = newBaseURL.String()
		return nil
	}
}

// ClientWithResponsesInterface is the interface specification for the client with responses above.
type ClientWithResponsesInterface interface {
	// GetConfig request
	GetConfigWithResponse(ctx context.Context, reqEditors ...RequestEditorFn) (*GetConfigResponse, error)

	// UpdateProviderConfiguration request with any body
	UpdateProviderConfigurationWithBodyWithResponse(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*UpdateProviderConfigurationResponse, error)

	UpdateProviderConfigurationWithResponse(ctx context.Context, body UpdateProviderConfigurationJSONRequestBody, reqEditors ...RequestEditorFn) (*UpdateProviderConfigurationResponse, error)
}

type GetConfigResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *struct {
		// The configuration for a Common Fate deployment.
		DeploymentConfiguration DeploymentConfiguration `json:"deploymentConfiguration"`
	}
}

// Status returns HTTPResponse.Status
func (r GetConfigResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r GetConfigResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

type UpdateProviderConfigurationResponse struct {
	Body         []byte
	HTTPResponse *http.Response
}

// Status returns HTTPResponse.Status
func (r UpdateProviderConfigurationResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r UpdateProviderConfigurationResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// GetConfigWithResponse request returning *GetConfigResponse
func (c *ClientWithResponses) GetConfigWithResponse(ctx context.Context, reqEditors ...RequestEditorFn) (*GetConfigResponse, error) {
	rsp, err := c.GetConfig(ctx, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseGetConfigResponse(rsp)
}

// UpdateProviderConfigurationWithBodyWithResponse request with arbitrary body returning *UpdateProviderConfigurationResponse
func (c *ClientWithResponses) UpdateProviderConfigurationWithBodyWithResponse(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*UpdateProviderConfigurationResponse, error) {
	rsp, err := c.UpdateProviderConfigurationWithBody(ctx, contentType, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseUpdateProviderConfigurationResponse(rsp)
}

func (c *ClientWithResponses) UpdateProviderConfigurationWithResponse(ctx context.Context, body UpdateProviderConfigurationJSONRequestBody, reqEditors ...RequestEditorFn) (*UpdateProviderConfigurationResponse, error) {
	rsp, err := c.UpdateProviderConfiguration(ctx, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseUpdateProviderConfigurationResponse(rsp)
}

// ParseGetConfigResponse parses an HTTP response from a GetConfigWithResponse call
func ParseGetConfigResponse(rsp *http.Response) (*GetConfigResponse, error) {
	bodyBytes, err := ioutil.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetConfigResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest struct {
			// The configuration for a Common Fate deployment.
			DeploymentConfiguration DeploymentConfiguration `json:"deploymentConfiguration"`
		}
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	}

	return response, nil
}

// ParseUpdateProviderConfigurationResponse parses an HTTP response from a UpdateProviderConfigurationWithResponse call
func ParseUpdateProviderConfigurationResponse(rsp *http.Response) (*UpdateProviderConfigurationResponse, error) {
	bodyBytes, err := ioutil.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &UpdateProviderConfigurationResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	return response, nil
}

// ServerInterface represents all server handlers.
type ServerInterface interface {
	// Get Deployment Configuration
	// (GET /api/v1/config)
	GetConfig(w http.ResponseWriter, r *http.Request)
	// Update Access Provider configuration
	// (PUT /api/v1/config/providers)
	UpdateProviderConfiguration(w http.ResponseWriter, r *http.Request)
}

// ServerInterfaceWrapper converts contexts to parameters.
type ServerInterfaceWrapper struct {
	Handler            ServerInterface
	HandlerMiddlewares []MiddlewareFunc
	ErrorHandlerFunc   func(w http.ResponseWriter, r *http.Request, err error)
}

type MiddlewareFunc func(http.HandlerFunc) http.HandlerFunc

// GetConfig operation middleware
func (siw *ServerInterfaceWrapper) GetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var handler = func(w http.ResponseWriter, r *http.Request) {
		siw.Handler.GetConfig(w, r)
	}

	for _, middleware := range siw.HandlerMiddlewares {
		handler = middleware(handler)
	}

	handler(w, r.WithContext(ctx))
}

// UpdateProviderConfiguration operation middleware
func (siw *ServerInterfaceWrapper) UpdateProviderConfiguration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var handler = func(w http.ResponseWriter, r *http.Request) {
		siw.Handler.UpdateProviderConfiguration(w, r)
	}

	for _, middleware := range siw.HandlerMiddlewares {
		handler = middleware(handler)
	}

	handler(w, r.WithContext(ctx))
}

type UnescapedCookieParamError struct {
	ParamName string
	Err       error
}

func (e *UnescapedCookieParamError) Error() string {
	return fmt.Sprintf("error unescaping cookie parameter '%s'", e.ParamName)
}

func (e *UnescapedCookieParamError) Unwrap() error {
	return e.Err
}

type UnmarshalingParamError struct {
	ParamName string
	Err       error
}

func (e *UnmarshalingParamError) Error() string {
	return fmt.Sprintf("Error unmarshaling parameter %s as JSON: %s", e.ParamName, e.Err.Error())
}

func (e *UnmarshalingParamError) Unwrap() error {
	return e.Err
}

type RequiredParamError struct {
	ParamName string
}

func (e *RequiredParamError) Error() string {
	return fmt.Sprintf("Query argument %s is required, but not found", e.ParamName)
}

type RequiredHeaderError struct {
	ParamName string
	Err       error
}

func (e *RequiredHeaderError) Error() string {
	return fmt.Sprintf("Header parameter %s is required, but not found", e.ParamName)
}

func (e *RequiredHeaderError) Unwrap() error {
	return e.Err
}

type InvalidParamFormatError struct {
	ParamName string
	Err       error
}

func (e *InvalidParamFormatError) Error() string {
	return fmt.Sprintf("Invalid format for parameter %s: %s", e.ParamName, e.Err.Error())
}

func (e *InvalidParamFormatError) Unwrap() error {
	return e.Err
}

type TooManyValuesForParamError struct {
	ParamName string
	Count     int
}

func (e *TooManyValuesForParamError) Error() string {
	return fmt.Sprintf("Expected one value for %s, got %d", e.ParamName, e.Count)
}

// Handler creates http.Handler with routing matching OpenAPI spec.
func Handler(si ServerInterface) http.Handler {
	return HandlerWithOptions(si, ChiServerOptions{})
}

type ChiServerOptions struct {
	BaseURL          string
	BaseRouter       chi.Router
	Middlewares      []MiddlewareFunc
	ErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)
}

// HandlerFromMux creates http.Handler with routing matching OpenAPI spec based on the provided mux.
func HandlerFromMux(si ServerInterface, r chi.Router) http.Handler {
	return HandlerWithOptions(si, ChiServerOptions{
		BaseRouter: r,
	})
}

func HandlerFromMuxWithBaseURL(si ServerInterface, r chi.Router, baseURL string) http.Handler {
	return HandlerWithOptions(si, ChiServerOptions{
		BaseURL:    baseURL,
		BaseRouter: r,
	})
}

// HandlerWithOptions creates http.Handler with additional options
func HandlerWithOptions(si ServerInterface, options ChiServerOptions) http.Handler {
	r := options.BaseRouter

	if r == nil {
		r = chi.NewRouter()
	}
	if options.ErrorHandlerFunc == nil {
		options.ErrorHandlerFunc = func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}
	wrapper := ServerInterfaceWrapper{
		Handler:            si,
		HandlerMiddlewares: options.Middlewares,
		ErrorHandlerFunc:   options.ErrorHandlerFunc,
	}

	r.Group(func(r chi.Router) {
		r.Get(options.BaseURL+"/api/v1/config", wrapper.GetConfig)
	})
	r.Group(func(r chi.Router) {
		r.Put(options.BaseURL+"/api/v1/config/providers", wrapper.UpdateProviderConfiguration)
	})

	return r
}

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"H4sIAAAAAAAC/7xX3W7bOBN9lQG/D9gb13L+mkZ32Ra7CBabDdIuepEGKC2OJMYShyEpu07gd1+QkhP9",
	"WI6LBdY3lsThcM6ZM+TwmSVUalKonGXxMzP4WKF1v5KQGD78rQV3eGNoKQUae1uP+5GElEMVHrnWhUy4",
	"k6SiB0vKf7NJjiX3T9qQRuMah7px9ZFUKrPKhFl+4P8GUxaz/0WvEUW1Extt1/+Ta7bZTEKY0qBg8d2I",
	"w/sJc2uNLGY0f8DEsY3/+ZlWk7J1LJ9QF7QuUbl68m0z+C/giZ7LAwF+GpnWBzvm/j6AE2gTI3W9IPuS",
	"IyRtK6AUXC4tfKSyJAW/cYfw6nEaVmsi2kVPC8tbK6VkgI+uM+mRpsjJtGHY/hRv1+MzN5P/SmuTfQC8",
	"EKUrvBLH+OxLdcJ+vLOOdCGzPAhQChYzeeSW+uSHWOaLRITIrvfS1k1Rx3ZHsly+P0W24MniLeo+e6NB",
	"FsLUK5VQKVX2Fec50SL4HKLO6F3zseT6zjojVXbfeqz/AvgtqXtIOIxXo/jq4mk2f3o6Pi2D65sx2XQp",
	"7QyDReekymwtfQVSCbmUouIFXCYJWgtbt0Nyq2Y/asLdopywlXT5TzDVoqet2uC+cdaS482ImA8h7fGM",
	"zOL8fE3nj+K0Q5ovG79nCiG9P17cdKAeUnkDBe3jnVLgxYBj60nuA/WhHQbvgpJja9zD0Yezp3mAt0Pa",
	"O7fBYAft/aBbbMPkcy2/0AL3+ru8uQLnjabwOaeqEDBH4GAwRYMqQXAEFhODDqSC73xlrS3jKPru1Vjy",
	"UM89cfU08hJFSyA7MB9GHz5mx/rpw+nJ2cWMs/rglSql7bnKk2CreIkhny+nBJuwyhQsZrlz2saRV0lJ",
	"KuUOp5KGUrhUgRpfcgVxIVXW2sS6xE/AVkkO3IJU1vGiQDEQDXAlOrmzL2Xd1lP7WLvFkhxCn6UlGluH",
	"eDSd+bhJo+JaspidTGfTmVcBd3kQQMS1jJZHUR2u/5KhG6rhd3Sj4Hx0XlPh5UrU1nVMrNfzHM9mY3X4",
	"YheNNkahQ6jKkpt1E9KrKQxbly60aHt81o1gtQPkVyMd2n5ieueVo3BcmZr6OU8WqMT0m/qmrslhDCv8",
	"xYSGxPrJXhTedOkfQvvjJYNKaJIq1AuHtHKVQWiyBu9qO2khqYxB5Yq1t6sswhwT7v99AJfaA+KFBcFt",
	"PiduBDQQLXDIKilQeAVVGtKCVnWV+rfBbgVXTWvWsvZvuUwdCj+Rg5BpKHcHFs1SJjjpReFxrWRRgCIo",
	"SGVoYOX5fINOT2CP06GBZ3ggsu7loF8Dr3eJ9bjiWteNaOSusdmt4K5w/vqjp83a2X7oLOxMns2gybvn",
	"1u4TR1FBCS9ysi5+f/b+jG3uN/8EAAD///ha7xYvDQAA",
}

// GetSwagger returns the content of the embedded swagger specification file
// or error if failed to decode
func decodeSpec() ([]byte, error) {
	zipped, err := base64.StdEncoding.DecodeString(strings.Join(swaggerSpec, ""))
	if err != nil {
		return nil, fmt.Errorf("error base64 decoding spec: %s", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(zr)
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}

	return buf.Bytes(), nil
}

var rawSpec = decodeSpecCached()

// a naive cached of a decoded swagger spec
func decodeSpecCached() func() ([]byte, error) {
	data, err := decodeSpec()
	return func() ([]byte, error) {
		return data, err
	}
}

// Constructs a synthetic filesystem for resolving external references when loading openapi specifications.
func PathToRawSpec(pathToFile string) map[string]func() ([]byte, error) {
	var res = make(map[string]func() ([]byte, error))
	if len(pathToFile) > 0 {
		res[pathToFile] = rawSpec
	}

	return res
}

// GetSwagger returns the Swagger specification corresponding to the generated code
// in this file. The external references of Swagger specification are resolved.
// The logic of resolving external references is tightly connected to "import-mapping" feature.
// Externally referenced files must be embedded in the corresponding golang packages.
// Urls can be supported but this task was out of the scope.
func GetSwagger() (swagger *openapi3.T, err error) {
	var resolvePath = PathToRawSpec("")

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = func(loader *openapi3.Loader, url *url.URL) ([]byte, error) {
		var pathToFile = url.String()
		pathToFile = path.Clean(pathToFile)
		getSpec, ok := resolvePath[pathToFile]
		if !ok {
			err1 := fmt.Errorf("path not found: %s", pathToFile)
			return nil, err1
		}
		return getSpec()
	}
	var specData []byte
	specData, err = rawSpec()
	if err != nil {
		return
	}
	swagger, err = loader.LoadFromData(specData)
	if err != nil {
		return
	}
	return
}
