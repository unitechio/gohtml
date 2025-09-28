// Package client contains HTML Converter HTTP Client. The Client implements htmlcreator.HTMLConverter interface
// for the UniPDF module and can be used as a plugin for the UniPDF creator.Creator.
package client

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/unitechio/gohtml/content"
	"github.com/unitechio/gohtml/selector"
	"github.com/unitechio/gohtml/sizes"
	"github.com/unitechio/gopdf/common"
)

// Options are the client options used by the HTTP client.
type Options struct {
	HTTPS          bool
	Hostname       string
	Port           int
	DefaultTimeout time.Duration
	Prefix         string
}

// ParseOptions parses options for the Client.
func ParseOptions(connectPath string) (Options, error) {
	if !strings.HasPrefix(connectPath, "http") {
		connectPath = "http://" + connectPath
	}
	cntPath, err := url.Parse(connectPath)
	if err != nil {
		return Options{}, fmt.Errorf("provided invalid unihtml-server url")
	}
	var port int
	if cntPath.Port() != "" {
		port, err = strconv.Atoi(cntPath.Port())
		if err != nil {
			return Options{}, fmt.Errorf("parsing port failed: %w", err)
		}
	}
	return Options{
		Hostname: cntPath.Hostname(),
		Port:     port,
		HTTPS:    cntPath.Scheme == "https",
		Prefix:   cntPath.Path}, nil
}

// Option is a function that changes client options.
type Option func(_fa *Options)

// Orientation sets the page orientation for the Query.
func (q *QueryBuilder) Orientation(orientation sizes.Orientation) *QueryBuilder {
	q.query.PageParameters.Orientation = orientation
	return q
}

// New creates new client with provided options.
func New(o Options) *Client {
	o.DefaultTimeout = time.Second * 30
	if o.Port <= 0 {
		o.Port = 8080
	}
	if o.Hostname == "" {
		o.Hostname = "127.0.0.1"
	}
	var transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second}
	common.Log.Info("Client Addr: %s", o.Addr())
	return &Client{Options: o, Client: &http.Client{Transport: transport, Timeout: o.DefaultTimeout}}
}

// BuildHTMLQuery creates a Query builder that is supposed to create valid
func BuildHTMLQuery() *QueryBuilder { return &QueryBuilder{} }

// WithDefaultTimeout sets the DefaultTimeout option for the client options.
func WithDefaultTimeout(option time.Duration) Option {
	return func(_db *Options) { _db.DefaultTimeout = option }
}

// Query gets the Query from provided query builder. If some error occurred during build process
// or the input is not valid the function would return an error.
func (q *QueryBuilder) Query() (*Query, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	return &q.query, nil
}

// TimeoutDuration sets the server query duration timeout.
// Once the timeout is reached the server will return an error.
func (qb *QueryBuilder) TimeoutDuration(d time.Duration) *QueryBuilder {
	qb.query.TimeoutDuration = d
	return qb
}

// WithPort sets the Port option for the client options.
func WithPort(option int) Option { return func(_ee *Options) { _ee.Port = option } }

// DefaultPageParameters creates default parameters.
func DefaultPageParameters() PageParameters { return PageParameters{Orientation: sizes.Portrait} }

// Addr gets the HTTP address URI used by the http.Client.
func (o *Options) Addr() string {
	builder := strings.Builder{}
	builder.WriteString("http")
	if o.HTTPS {
		builder.WriteRune('s')
	}
	builder.WriteString("://")
	builder.WriteString(o.Hostname)
	builder.WriteRune(':')
	builder.WriteString(strconv.Itoa(o.Port))
	if o.Prefix != "" {
		builder.WriteString(o.Prefix)
	}
	return builder.String()
}

// PaperWidth sets up the PaperWidth (in cm) parameter for the query.
func (q *QueryBuilder) PaperWidth(paperWidth sizes.Length) *QueryBuilder {
	q.query.PageParameters.PaperWidth = paperWidth
	return q
}

// PageParameters are the query parameters used in the PDF generation.
type PageParameters struct {

	// PaperWidth sets the width of the paper.
	PaperWidth sizes.Length `schema:"paper-width" json:"paperWidth"`

	// PaperHeight is the height of the output paper.
	PaperHeight sizes.Length `schema:"paper-height" json:"paperHeight"`

	// PageSize is the page size string.
	PageSize *sizes.PageSize `schema:"page-size" json:"pageSize"`

	// Orientation defines if the output should be in a landscape format.
	Orientation sizes.Orientation `schema:"orientation" json:"orientation"`

	// MarginTop sets up the Top Margin for the output.
	MarginTop sizes.Length `schema:"margin-top" json:"marginTop"`

	// MarginBottom sets up the Bottom Margin for the output.
	MarginBottom sizes.Length `schema:"margin-bottom" json:"marginBottom"`

	// MarginLeft sets up the Left Margin for the output.
	MarginLeft sizes.Length `schema:"margin-left" json:"marginLeft"`

	// MarginRight sets up the Right Margin for the output.
	MarginRight sizes.Length `schema:"margin-right" json:"marginRight"`
}

// MarginRight sets up the MarginRight parameter for the query.
func (q *QueryBuilder) MarginRight(marginRight sizes.Length) *QueryBuilder {
	q.query.PageParameters.MarginRight = marginRight
	return q
}

// HealthCheck connects to the server and check the health status of the server.
func (cli *Client) HealthCheck(ctx context.Context) error {
	address := cli.Options.Addr()
	address = fmt.Sprintf("%s/health", address)
	req, err := http.NewRequest("GET", address, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	resp, err := cli.Client.Do(req)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusInternalServerError:
		return ErrInternalError
	case http.StatusBadGateway:
		return ErrBadGateway
	default:
		return ErrNotImplemented
	}
}

// Client is a structure that is a HTTP client for the unihtml server.
type Client struct {
	Options Options
	Client  *http.Client
}

// BySelector is a structure that defines a selector with it's query 'by' type.
type BySelector struct {
	Selector string          `json:"selector"`
	By       selector.ByType `json:"by"`
}

// PageSize sets up the PageSize parameter for the query.
func (q *QueryBuilder) PageSize(pageSize sizes.PageSize) *QueryBuilder {
	if pageSize != sizes.Undefined {
		q.query.PageParameters.PageSize = &pageSize
	}
	return q
}

// WithHTTPS sets the TLS option for the client options.
func WithHTTPS(useHTTPS bool) Option { return func(_ga *Options) { _ga.HTTPS = useHTTPS } }

// Validate checks if the QueryBuilder had no errors during composition and creation.
func (q *QueryBuilder) Validate() error {
	if q.err != nil {
		return q.err
	}
	return q.query.Validate()
}

// PDFResponse is the response used by the HTMLConverter.
type PDFResponse struct {
	ID   string `json:"id"`
	Data []byte `json:"data"`
}

// MarginLeft sets up the MarginLeft parameter for the query.
func (q *QueryBuilder) MarginLeft(marginLeft sizes.Length) *QueryBuilder {
	q.query.PageParameters.MarginLeft = marginLeft
	return q
}

// RenderParameters are the parameters related with the rendering.
type RenderParameters struct {
	WaitTime    time.Duration `schema:"minimum-load-time" json:"waitTime"`
	WaitReady   []BySelector  `json:"waitReady"`
	WaitVisible []BySelector  `json:"waitVisible"`
}

// WaitReady waits for the selector to get ready - 'loaded'.
func (q *QueryBuilder) WaitReady(selector string, by selector.ByType) *QueryBuilder {
	q.query.RenderParameters.WaitReady = append(q.query.RenderParameters.WaitReady, BySelector{Selector: selector, By: by})
	return q
}

// PaperHeight sets up the PaperHeight (in cm) parameter for the query.
func (q *QueryBuilder) PaperHeight(paperHeight sizes.Length) *QueryBuilder {
	q.query.PageParameters.PaperHeight = paperHeight
	return q
}

// Validate checks the validity of the RenderParameters.
func (rp *RenderParameters) Validate() error {
	if rp.WaitTime > time.Minute*3 {
		return errors.New("too long minimum load time. Maximum is 3 minutes")
	}
	for _, _de := range rp.WaitReady {
		if _cgf := _de.Validate(); _cgf != nil {
			return fmt.Errorf("one of wait ready selector is not valid: %w", _cgf)
		}
	}
	return nil
}

type generatePDFRequestV1 struct {
	Content         []byte `json:"content"`
	ContentType     string `json:"contentType"`
	ContentURL      string `json:"contentURL"`
	Method          string `json:"method"`
	ExpiresAt       int64  `json:"expiresAt"`
	TimeoutDuration int64  `json:"timeoutDuration,omitempty"`
	PageParameters
	RenderParameters
}

// SetContent sets custom data with it's content type.
func (q *QueryBuilder) SetContent(content content.Content) *QueryBuilder {
	if q.err != nil {
		return q
	}
	switch content.Method() {
	case "dir", "html":
		if q.query.ContentType != "" {
			q.err = ErrContentTypeDeclared
			return q
		}
		if content.ContentType() == "" {
			q.err = fmt.Errorf("empty custom content type %w", ErrContentType)
			return q
		}
		q.query.Content = content.Data()
		q.query.ContentType = content.ContentType()
	case "web":
		if q.query.ContentType != "" {
			q.err = ErrContentTypeDeclared
			return q
		}
		q.query.URL = string(content.Data())
		q.query.ContentType = content.ContentType()
	default:
		q.err = fmt.Errorf("invalid content method: %s", content.Method())
		return q
	}
	q.query.Method = content.Method()
	return q
}

// WithHostname sets the Hostname option for the client options.
func WithHostname(option string) Option { return func(_cad *Options) { _cad.Hostname = option } }

// Err gets the error which could occur in the query.
func (q *QueryBuilder) Err() error { return q.err }

// WaitVisible waits for the selector to get visible.
func (q *QueryBuilder) WaitVisible(selector string, by selector.ByType) *QueryBuilder {
	q.query.RenderParameters.WaitVisible = append(q.query.RenderParameters.WaitVisible, BySelector{Selector: selector, By: by})
	return q
}

// WithPrefix sets the client prefix.
func WithPrefix(prefix string) Option { return func(_ag *Options) { _ag.Prefix = prefix } }

// Validate checks if the parameters are valid.
func (p *PageParameters) Validate() error {
	if p.PaperWidth != nil {
		if p.PaperWidth.Millimeters() < 0 {
			return errors.New("negative value for PaperWidth")
		}
	}
	if p.PaperHeight != nil {
		if p.PaperHeight.Millimeters() < 0 {
			return errors.New("negative value for PaperHeight")
		}
	}
	if p.MarginTop != nil {
		if p.MarginTop.Millimeters() < 0 {
			return errors.New("negative value for MarginTop")
		}
	}
	if p.MarginBottom != nil {
		if p.MarginBottom.Millimeters() < 0 {
			return errors.New("negative value for MarginBottom")
		}
	}
	if p.MarginLeft != nil {
		if p.MarginLeft.Millimeters() < 0 {
			return errors.New("negative value for MarginLeft")
		}
	}
	if p.MarginRight != nil {
		if p.MarginRight.Millimeters() < 0 {
			return errors.New("negative value for MarginRight")
		}
	}
	if p.PageSize != nil && !p.PageSize.IsAPageSize() {
		return errors.New("invalid page size")
	}
	return nil
}
func (cli *Client) getGenerateRequest(ctx context.Context, q *Query) (*http.Request, error) {
	reqData := generatePDFRequestV1{
		Method:           q.Method,
		PageParameters:   q.PageParameters,
		RenderParameters: q.RenderParameters,
		TimeoutDuration:  int64(q.TimeoutDuration),
	}

	switch q.Method {
	case "web":
		reqData.ContentURL = q.URL
	case "dir", "html":
		reqData.ContentType = q.ContentType
		reqData.Content = q.Content
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(&reqData); err != nil {
		return nil, fmt.Errorf("encoding request failed: %v", err)
	}

	url := fmt.Sprintf("%s/v1/pdf", cli.Options.Addr())
	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "deflate, gzip;q=1.0, *;q=0.5")

	return req.WithContext(ctx), nil
}

// MarginBottom sets up the MarginBottom parameter for the query.
func (q *QueryBuilder) MarginBottom(marginBottom sizes.Length) *QueryBuilder {
	q.query.PageParameters.MarginBottom = marginBottom
	return q
}

var (
	ErrMissingData         = errors.New("missing input data")
	ErrContentType         = errors.New("invalid content type")
	ErrContentTypeDeclared = errors.New("content type is already declared")
)

// Validate checks validity of the selector.
func (b BySelector) Validate() error {
	if b.Selector == "" {
		return errors.New("provided empty selector")
	}
	if isValid := b.By.Validate(); isValid != nil {
		return isValid
	}
	return nil
}

// MarginTop sets up the MarginTop parameter for the query.
func (q *QueryBuilder) MarginTop(marginTop sizes.Length) *QueryBuilder {
	q.query.PageParameters.MarginTop = marginTop
	return q
}

var (
	ErrNotFound       = errors.New("not found")
	ErrBadRequest     = errors.New("bad request")
	ErrNotImplemented = errors.New("not implemented")
	ErrInternalError  = errors.New("internal server error")
	ErrBadGateway     = errors.New("bad gateway")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrTimedOut       = errors.New("request timed out")
)

// Query is a structure that contains query parameters and the content used for the HTMLConverter conversion process.
type Query struct {
	Content          []byte
	ContentType      string
	URL              string
	Method           string
	PageParameters   PageParameters
	RenderParameters RenderParameters
	TimeoutDuration  time.Duration
}

// Portrait sets up the portrait page orientation.
func (q *QueryBuilder) Portrait() *QueryBuilder {
	q.query.PageParameters.Orientation = sizes.Portrait
	return q
}

// QueryBuilder is the query that converts HTMLConverter defined data
type QueryBuilder struct {
	query Query
	err   error
}

// ConvertHTML converts provided Query input into PDF file data.
// Implements creator.HTMLConverter interface.
func (cli *Client) ConvertHTML(ctx context.Context, q *Query) (*PDFResponse, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	req, err := cli.getGenerateRequest(ctx, q)
	if err != nil {
		return nil, err
	}

	common.Log.Trace("Request - %s - %s%s, Headers: %v, Query: %v",
		req.Method, req.URL.Hostname(), req.URL.Path, req.Header, req.URL.Query())

	httpClient := *cli.Client
	if q.TimeoutDuration != 0 {
		httpClient.Timeout = q.TimeoutDuration
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var httpErr error
	switch resp.StatusCode {
	case http.StatusNotFound:
		httpErr = ErrNotFound
	case http.StatusBadRequest:
		httpErr = ErrBadRequest
	case http.StatusNotImplemented:
		httpErr = ErrNotImplemented
	case http.StatusUnauthorized:
		httpErr = ErrUnauthorized
	case http.StatusRequestTimeout:
		httpErr = ErrTimedOut
	case http.StatusCreated:
		// OK
	default:
		httpErr = ErrInternalError
	}

	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
	case "deflate":
		reader = flate.NewReader(resp.Body)
	case "":
		reader = resp.Body
	default:
		return nil, fmt.Errorf("unsupported Content-Encoding: %s header", resp.Header.Get("Content-Encoding"))
	}

	data, readErr := io.ReadAll(reader)
	if readErr != nil && httpErr == nil {
		return nil, fmt.Errorf("UniHTML server error %s", readErr)
	}

	common.Log.Trace("[%d] %s %s%s", resp.StatusCode, req.Method, req.URL.Host, req.URL.Path)
	if httpErr != nil {
		return nil, fmt.Errorf("%s %w", string(data), httpErr)
	}

	jobID := resp.Header.Get("X-Job-ID")
	common.Log.Trace("Response ID %s", jobID)

	return &PDFResponse{ID: jobID, Data: data}, nil
}

// Validate checks if provided Query is valid.
func (q *Query) Validate() error {
	switch q.Method {
	case "web":
		if q.URL == "" {
			return ErrMissingData
		}
	case "dir", "html":
		if len(q.Content) == 0 {
			return ErrMissingData
		}
		if q.ContentType == "" {
			return ErrContentType
		}
	default:
		return fmt.Errorf("undefined content query method: %s", q.Method)
	}

	if err := q.PageParameters.Validate(); err != nil {
		return err
	}
	if err := q.RenderParameters.Validate(); err != nil {
		return err
	}
	return nil
}

// Landscape sets up the landscape portrait orientation.
func (q *QueryBuilder) Landscape() *QueryBuilder {
	q.query.PageParameters.Orientation = sizes.Landscape
	return q
}

// WaitTime sets the minimum load time parameter for the page rendering.
func (q *QueryBuilder) WaitTime(d time.Duration) *QueryBuilder {
	q.query.RenderParameters.WaitTime = d
	return q
}
func (cli *Client) setQueryValues(req *http.Request, query *Query) {
	values := req.URL.Query()
	page := query.PageParameters
	if page.PageSize != nil {
		values.Set("page-size", page.PageSize.String())
	}
	if page.PaperHeight != nil {
		values.Set("paper-height", page.PaperHeight.String())
	}
	if page.PaperWidth != nil {
		values.Set("paper-width", page.PaperWidth.String())
	}
	if page.MarginTop != nil {
		values.Set("margin-top", page.MarginTop.String())
	}
	if page.MarginBottom != nil {
		values.Set("margin-bottom", page.MarginBottom.String())
	}
	if page.MarginRight != nil {
		values.Set("margin-right", page.MarginRight.String())
	}
	if page.MarginLeft != nil {
		values.Set("margin-left", page.MarginLeft.String())
	}
	if page.Orientation == sizes.Landscape {
		values.Set("orientation", page.Orientation.String())
	}
	if query.RenderParameters.WaitTime != 0 {
		values.Set("minimum-load-time", strconv.FormatInt(int64(query.RenderParameters.WaitTime/time.Millisecond), 10))
	}
	req.URL.RawQuery = values.Encode()
}
