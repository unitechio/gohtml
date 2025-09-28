// Package html contains a plugin for the UniDoc.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"net/url"
	"os"
	"time"

	"github.com/unitechio/gohtml/client"
	"github.com/unitechio/gohtml/content"
	"github.com/unitechio/gohtml/selector"
	"github.com/unitechio/gohtml/sizes"
	"github.com/unitechio/gopdf/common"
	"github.com/unitechio/gopdf/creator"
	"github.com/unitechio/gopdf/model"
	"github.com/unitechio/gopdf/render"
)

// ===================== ERRORS =====================

var (
	ErrNoClient          = errors.New("UniHTML client not found")
	ErrContentNotDefined = errors.New("html document content not defined")
	unihtmlClient        *client.Client
)

// ===================== DOCUMENT STRUCT =====================

// Document is HTML document wrapper that is used for extracting and converting HTML document into PDF pages.
type Document struct {
	content     content.Content
	margins     margins
	position    creator.Positioning
	posX, posY  float64
	pageSize    sizes.PageSize
	pageWidth   sizes.Length
	pageHeight  sizes.Length
	orientation sizes.Orientation
	trimLast    bool
	waitTime    time.Duration
	waitReady   []client.BySelector
	waitVisible []client.BySelector
	timeout     *time.Duration
}

type margins struct {
	Left, Right, Bottom, Top sizes.Length
}

// ===================== OPTIONS =====================

// Options are the HTML Client options used for establishing the connection.
type Options struct {
	Hostname string
	Port     int
	Secure   bool
	Prefix   string
}

// ===================== CONSTRUCTORS =====================

// NewDocument creates new HTML Document used as an input for the creator.Drawable.
func NewDocument(path string) (*Document, error) {
	doc := &Document{}

	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "http", "https":
		doc.content, err = content.NewWebURL(path)
		if err != nil {
			return nil, err
		}
		return doc, nil
	}

	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		doc.content, err = content.NewHTMLFile(path)
	} else {
		doc.content, err = content.NewZipDirectory(path)
	}
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// NewDocumentFromString creates a new Document from the provided HTML string.
func NewDocumentFromString(html string) (*Document, error) {
	c, err := content.NewStringContent(html)
	if err != nil {
		return nil, err
	}
	return &Document{content: c}, nil
}

// ===================== CONNECTION =====================

// Connect creates UniHTML HTTP Client and tries to establish connection with the server.
func Connect(path string) error {
	opts, err := client.ParseOptions(path)
	if err != nil {
		return err
	}
	unihtmlClient = client.New(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := unihtmlClient.HealthCheck(ctx); err != nil {
		return err
	}
	return nil
}

// ConnectOptions creates UniHTML HTTP Client and tries to establish connection with the server.
func ConnectOptions(o Options) error {
	unihtmlClient = client.New(client.Options{Hostname: o.Hostname, Port: o.Port, HTTPS: o.Secure})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := unihtmlClient.HealthCheck(ctx); err != nil {
		return err
	}
	return nil
}

// ===================== DOCUMENT METHODS =====================

func (d *Document) validate() error {
	if unihtmlClient == nil {
		return ErrNoClient
	}
	if d.content == nil {
		return ErrContentNotDefined
	}
	return nil
}

func (d *Document) GetContent() content.Content { return d.content }

func (d *Document) SetMargins(left, right, top, bottom float64) {
	d.margins.Left = sizes.Point(left)
	d.margins.Right = sizes.Point(right)
	d.margins.Top = sizes.Point(top)
	d.margins.Bottom = sizes.Point(bottom)
	d.position = creator.PositionAbsolute
}

func (d *Document) SetMarginLeft(m sizes.Length)   { d.margins.Left = m }
func (d *Document) SetMarginRight(m sizes.Length)  { d.margins.Right = m }
func (d *Document) SetMarginTop(m sizes.Length)    { d.margins.Top = m }
func (d *Document) SetMarginBottom(m sizes.Length) { d.margins.Bottom = m }

func (d *Document) SetPageSize(ps sizes.PageSize) error {
	if !ps.IsAPageSize() {
		return errors.New("provided invalid page size")
	}
	d.pageSize = ps
	d.position = creator.PositionAbsolute
	return nil
}

func (d *Document) SetPageWidth(w sizes.Length) error {
	d.pageWidth = w
	d.position = creator.PositionAbsolute
	return nil
}

func (d *Document) SetPageHeight(h sizes.Length) error {
	d.pageHeight = h
	d.position = creator.PositionAbsolute
	return nil
}

func (d *Document) SetLandscapeOrientation() {
	d.orientation = sizes.Landscape
}

func (d *Document) SetPos(x, y float64) {
	d.position = creator.PositionAbsolute
	d.posX, d.posY = x, y
}

func (d *Document) SetTimeoutDuration(duration time.Duration) { d.timeout = &duration }
func (d *Document) getTimeoutDuration() time.Duration {
	if d.timeout != nil {
		return *d.timeout
	}
	return 0
}

func (d *Document) WaitTime(duration time.Duration) { d.waitTime = duration }
func (d *Document) TrimLastPageContent()            { d.trimLast = true }

func (d *Document) WaitReady(sel string, by ...selector.ByType) {
	byType := selector.BySearch
	if len(by) > 0 {
		byType = by[0]
	}
	d.waitReady = append(d.waitReady, client.BySelector{Selector: sel, By: byType})
}

func (d *Document) WaitVisible(sel string, by ...selector.ByType) {
	byType := selector.BySearch
	if len(by) > 0 {
		byType = by[0]
	}
	d.waitVisible = append(d.waitVisible, client.BySelector{Selector: sel, By: byType})
}

// ===================== EXPORT =====================

func (d *Document) WriteToFile(outputPath string) error {
	if err := d.validate(); err != nil {
		return err
	}
	timeout := 20*time.Second + d.waitTime
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pages, err := d.extract(ctx, d.pageWidth, d.pageHeight, d.getMargins())
	if err != nil {
		return err
	}

	c := creator.New()
	for _, p := range pages {
		if err := c.AddPage(p); err != nil {
			return err
		}
	}
	return c.WriteToFile(outputPath)
}

func (d *Document) GetPdfPages(ctx context.Context) ([]*model.PdfPage, error) {
	if err := d.validate(); err != nil {
		return nil, err
	}
	return d.extract(ctx, d.pageWidth, d.pageHeight, d.getMargins())
}

// ===================== INTERNAL =====================

func (d *Document) getMargins() margins {
	m := d.margins
	if d.position.IsRelative() {
		m.Top = sizes.Millimeter(1)
		m.Left = sizes.Millimeter(1)
		m.Bottom = sizes.Millimeter(1)
		m.Right = sizes.Millimeter(1)
		return m
	}
	if m.Top == nil {
		m.Top = sizes.Millimeter(10)
	}
	if m.Bottom == nil {
		m.Bottom = sizes.Millimeter(10)
	}
	if m.Left == nil {
		m.Left = sizes.Millimeter(10)
	}
	if m.Right == nil {
		m.Right = sizes.Millimeter(10)
	}
	return m
}

func (d *Document) extract(ctx context.Context, w, h sizes.Length, m margins) ([]*model.PdfPage, error) {
	query := client.BuildHTMLQuery().
		SetContent(d.content).
		PageSize(d.pageSize).
		PaperWidth(w).
		PaperHeight(h).
		Orientation(d.orientation).
		MarginLeft(m.Left).
		MarginRight(m.Right).
		MarginTop(m.Top).
		MarginBottom(m.Bottom).
		TimeoutDuration(d.getTimeoutDuration()).
		WaitTime(d.waitTime)

	for _, sel := range d.waitReady {
		query.WaitReady(sel.Selector, sel.By)
	}
	for _, sel := range d.waitVisible {
		query.WaitVisible(sel.Selector, sel.By)
	}

	req, err := query.Query()
	if err != nil {
		return nil, err
	}

	var cancel context.CancelFunc
	if d.timeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *d.timeout)
	} else {
		ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	}
	defer cancel()

	resp, err := unihtmlClient.ConvertHTML(ctx, req)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(resp.Data)
	pdfReader, err := model.NewPdfReader(reader)
	if err != nil {
		return nil, err
	}
	return pdfReader.PageList, nil
}

// ===================== INTERFACE IMPLEMENTATIONS =====================

// Implements creator.Drawable
func (d *Document) ContainerComponent(container creator.Drawable) (creator.Drawable, error) {
	switch container.(type) {
	case *creator.Chapter:
	default:
		return nil, fmt.Errorf("unihtml.Document can't be a component of the %T container", container)
	}
	return d, nil
}

// Implements creator.Drawable
func (d *Document) GeneratePageBlocks(ctx creator.DrawContext) ([]*creator.Block, creator.DrawContext, error) {
	if err := d.validate(); err != nil {
		return nil, ctx, err
	}
	var blocks []*creator.Block
	m := d.getMargins()
	w, h := d.pageWidth, d.pageHeight

	if d.position.IsRelative() {
		w, h = sizes.Point(ctx.Width), sizes.Point(ctx.Height)
		ctx.X -= float64(m.Left.Points())
	} else {
		ctx.X, ctx.Y = d.posX, d.posY
	}

	pages, err := d.extract(context.Background(), w, h, m)
	if err != nil {
		return nil, creator.DrawContext{}, err
	}

	for i, p := range pages {
		block, err := creator.NewBlockFromPage(p)
		if err != nil {
			return nil, creator.DrawContext{}, err
		}
		var trimHeight float64
		if d.trimLast && i == len(pages)-1 {
			imgDev := render.NewImageDevice()
			img, err := imgDev.Render(p)
			if err != nil {
				return nil, creator.DrawContext{}, err
			}
			mbox, err := p.GetMediaBox()
			if err != nil {
				return nil, creator.DrawContext{}, err
			}
			start := time.Now()
			trimRatio := detectTrimHeight(img)
			trimHeight = mbox.Height() * trimRatio
			common.Log.Trace("Trimming last document page taken: %v", time.Since(start))
			if d.margins.Bottom != nil {
				trimHeight -= float64(d.margins.Bottom.Points())
			}
			if trimHeight < 0 {
				trimHeight = 0
			}
			common.Log.Trace("Cropping document's page %.2f points off bottom of media box", trimHeight)
		}
		pageBlocks, newCtx, err := block.GeneratePageBlocks(ctx)
		if err != nil {
			return nil, creator.DrawContext{}, err
		}
		ctx = newCtx
		ctx.Y -= trimHeight
		if i != len(pages)-1 && ctx.Y > (ctx.PageHeight-ctx.Margins.Bottom)*.95 {
			ctx.X = ctx.Margins.Left
			ctx.Y = ctx.Margins.Top
			ctx.Page++
		}
		blocks = append(blocks, pageBlocks...)
	}
	return blocks, ctx, nil
}

// Unused methods just to satisfy interface
func (d *Document) GenerateKDict() (*model.KDict, error)   { return nil, nil }
func (d *Document) SetStructureType(t model.StructureType) {}
func (d *Document) SetMarkedContentID(id int64)            {}

// ===================== HELPERS =====================

func detectTrimHeight(img image.Image) float64 {
	bounds := img.Bounds()
	var lastNonEmptyY int
	ref := img.At(bounds.Min.X, bounds.Max.Y-1)
	r, g, b, _ := ref.RGBA()
	isWhite := r == math.MaxUint16 && g == math.MaxUint16 && b == math.MaxUint16

	for y := bounds.Max.Y - 1; y >= bounds.Min.Y; y-- {
		var rowHasContent bool
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			if (isWhite && !(cr == r && cg == g && cb == b)) ||
				(!isWhite && (math.Abs(float64(cr)-float64(r))/float64(math.MaxUint16) > 0.03 ||
					math.Abs(float64(cg)-float64(g))/float64(math.MaxUint16) > 0.03 ||
					math.Abs(float64(cb)-float64(b))/float64(math.MaxUint16) > 0.03)) {
				rowHasContent = true
				break
			}
		}
		if rowHasContent {
			break
		}
		lastNonEmptyY = y
	}
	return float64(bounds.Max.Y-lastNonEmptyY) / float64(bounds.Max.Y)
}
