// Package sizes defines basic types that determines the size units i.e. lengths.
package sizes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Length is the default dimension unit.
type Length interface {
	Millimeters() Millimeter
	Inches() Inch
	Points() Point
	String() string
}

// Millimeter is the dimension unit that defines a millimeter.
type Millimeter float64

// Inch is a unit that defines an inch.
type Inch float64

// Point is a unit of Length commonly used to measure the height of fonts.
type Point float64

// Orientation is the page orientation type wrapper.
type Orientation bool

// PageSize is the enum used for defining the page size.
type PageSize int

// LengthFlag is a pflag wrapper for the Length value.
type LengthFlag struct{ Length Length }

const (
	// Conversion constants
	mmToInch    = float64(1) / 25.4
	inchToMm    = 25.4
	pointToMm   = 0.3528
	inchToPoint = 1.0 / 64
	mmToPoint   = 1.0 / pointToMm

	pointToInch = 0.0139
)

// Page size enum
const (
	Undefined PageSize = iota
	A0
	A1
	A2
	A3
	A4
	A5
	A6
	A7
	A8
	A9
	A10
	B0
	B1
	B2
	B3
	B4
	B5
	B6
	B7
	B8
	B9
	B10
	Letter
)

const (
	Portrait  = Orientation(false)
	Landscape = Orientation(true)
)

// Length Implementations ===========================

func (m Millimeter) Millimeters() Millimeter { return m }
func (m Millimeter) Inches() Inch            { return Inch(float64(m) * mmToInch) }
func (m Millimeter) Points() Point           { return Point(m * mmToPoint) }
func (m Millimeter) String() string {
	var sb strings.Builder
	sb.WriteString(strconv.FormatFloat(float64(m), 'f', 1, 64))
	sb.WriteString("mm")
	return sb.String()
}
func (m Millimeter) MarshalJSON() ([]byte, error) { return marshalUnit(m) }

func (i Inch) Millimeters() Millimeter { return Millimeter(float64(i) * inchToMm) }
func (i Inch) Inches() Inch            { return i }
func (i Inch) Points() Point           { return Point(float64(i) * inchToPoint) }
func (i Inch) String() string {
	var sb strings.Builder
	sb.WriteString(strconv.FormatFloat(float64(i), 'f', 1, 64))
	sb.WriteString("in")
	return sb.String()
}
func (i Inch) MarshalJSON() ([]byte, error) { return marshalUnit(i) }

func (p Point) Millimeters() Millimeter { return Millimeter(float64(p) * pointToMm) }
func (p Point) Inches() Inch            { return Inch(float64(p) * pointToInch) }
func (p Point) Points() Point           { return p }
func (p Point) String() string {
	var sb strings.Builder
	sb.WriteString(strconv.FormatFloat(float64(p), 'f', 1, 64))
	sb.WriteString("pt")
	return sb.String()
}
func (p Point) MarshalJSON() ([]byte, error) { return marshalUnit(p) }

// Marshal helpers =================================

func marshalUnit(unit Length) ([]byte, error) {
	if unit == nil {
		return nil, nil
	}
	str, err := MarshalUnit(unit)
	if err != nil {
		return nil, err
	}
	return []byte("\"" + str + "\""), nil
}

func MarshalUnit(unit Length) (string, error) {
	switch v := unit.(type) {
	case Millimeter:
		return fmt.Sprintf("%.0fmm", v), nil
	case Inch:
		return fmt.Sprintf("%.0fin", v), nil
	case Point:
		return fmt.Sprintf("%.0fpt", v), nil
	default:
		return "", fmt.Errorf("invalid unit type: %T", unit)
	}
}

func UnmarshalLength(length string) (Length, error) {
	if strings.HasSuffix(length, "mm") {
		return parseMillimeter(length)
	}
	if strings.HasSuffix(length, "in") {
		return parseInch(length)
	}
	if strings.HasSuffix(length, "pt") {
		return parsePoint(length)
	}
	return nil, fmt.Errorf("invalid length input: %s", length)
}

func parseMillimeter(s string) (Millimeter, error) {
	s = strings.TrimSpace(strings.TrimSuffix(s, "mm"))
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid millimeter value: %w", err)
	}
	return Millimeter(val), nil
}

func parseInch(s string) (Inch, error) {
	s = strings.TrimSpace(strings.TrimSuffix(s, "in"))
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid inch value: %w", err)
	}
	return Inch(val), nil
}

func parsePoint(s string) (Point, error) {
	s = strings.TrimSpace(strings.Trim(s, "pt"))
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return Point(val), nil
}

func UnmarshalInch(unit string) (Inch, error) {
	if strings.HasSuffix(unit, "mm") {
		mm, err := parseMillimeter(unit)
		if err != nil {
			return 0, err
		}
		return mm.Inches(), nil
	}
	if strings.HasSuffix(unit, "in") {
		return parseInch(unit)
	}
	return 0, fmt.Errorf("invalid inch input: %s", unit)
}

// Orientation ======================================

func (o Orientation) String() string {
	if o == Portrait {
		return "portrait"
	}
	return "landscape"
}

func (o *Orientation) Set(s string) error {
	switch s {
	case "portrait":
		*o = Portrait
	case "landscape":
		*o = Landscape
	default:
		return fmt.Errorf("invalid orientation: '%s'", s)
	}
	return nil
}

func (o Orientation) Type() string { return "orientation" }

// PageSize ==========================================

var (
	pageSizeNames = "UndefinedA0A1A2A3A4A5A6A7A8A9A10B0B1B2B3B4B5B6B7B8B9B10Letter"
	pageSizeIdx   = [...]uint8{0, 9, 11, 13, 15, 17, 19, 21, 23, 25, 27, 29, 32, 34, 36, 38, 40, 42, 44, 46, 48, 50, 52, 55, 61}
	pageSizeMap   = map[string]PageSize{}
	pageSizes     = []PageSize{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23}
)

func init() {
	for i := range pageSizes {
		name := pageSizeNames[pageSizeIdx[i]:pageSizeIdx[i+1]]
		pageSizeMap[name] = PageSize(i)
	}
}

func (p PageSize) String() string {
	if p < 0 || p >= PageSize(len(pageSizeIdx)-1) {
		return fmt.Sprintf("PageSize(%d)", p)
	}
	return pageSizeNames[pageSizeIdx[p]:pageSizeIdx[p+1]]
}

func (p PageSize) MarshalText() ([]byte, error) { return []byte(p.String()), nil }
func (p PageSize) MarshalJSON() ([]byte, error) { return json.Marshal(p.String()) }

func (p *PageSize) UnmarshalText(text []byte) error {
	val, err := PageSizeString(string(text))
	if err != nil {
		return err
	}
	*p = val
	return nil
}

func (p *PageSize) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("PageSize should be a string, got %s", data)
	}
	val, err := PageSizeString(str)
	if err != nil {
		return err
	}
	*p = val
	return nil
}

func PageSizeValues() []PageSize { return pageSizes }

func PageSizeString(s string) (PageSize, error) {
	if val, ok := pageSizeMap[s]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s does not belong to PageSize values", s)
}

func (p PageSize) Dimensions() (Millimeter, Millimeter) {
	switch p {
	case A0:
		return 841, 1189
	case A1:
		return 594, 841
	case A2:
		return 420, 594
	case A3:
		return 297, 420
	case A4:
		return 210, 297
	case A5:
		return 148, 210
	case A6:
		return 105, 148
	case A7:
		return 74, 105
	case A8:
		return 52, 74
	case A9:
		return 37, 52
	case A10:
		return 26, 37
	case B0:
		return 1000, 1414
	case B1:
		return 707, 1000
	case B2:
		return 500, 707
	case B3:
		return 353, 500
	case B4:
		return 250, 353
	case B5:
		return 176, 250
	case B6:
		return 125, 176
	case B7:
		return 88, 125
	case B8:
		return 66, 88
	case B9:
		return 44, 62
	case B10:
		return 31, 44
	case Letter:
		return 215.9, 279.4
	}
	return 0, 0
}

func (p PageSize) IsAPageSize() bool {
	for _, v := range pageSizes {
		if p == v {
			return true
		}
	}
	return false
}

// Flag + Viper compatibility ========================

func (lf *LengthFlag) String() string {
	if lf.Length == nil {
		return "undefined"
	}
	return lf.Length.String()
}

func (lf *LengthFlag) Set(s string) error {
	if s == "undefined" {
		lf.Length = nil
		return nil
	}
	val, err := UnmarshalLength(s)
	if err != nil {
		return err
	}
	lf.Length = val
	return nil
}

func (lf *LengthFlag) Type() string { return "unit" }

func (i *Inch) Set(s string) error {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("invalid inch value: %w", err)
	}
	*i = Inch(val)
	return nil
}
func (i *Inch) HasChanged() bool { return i != nil }
func (i Inch) Name() string      { return "inch" }
func (i Inch) Type() string      { return "inch" }
func (i Inch) ValueString() string {
	return i.String()
}
func (i Inch) ValueType() string { return i.Type() }

func (p *Point) Set(s string) error {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("invalid point value: %w", err)
	}
	*p = Point(val)
	return nil
}
func (p *Point) HasChanged() bool { return p != nil }
func (p Point) Name() string      { return "point" }
func (p Point) Type() string      { return "point" }
func (p Point) ValueString() string {
	return p.String()
}
func (p Point) ValueType() string { return p.Type() }

func (p *PageSize) Set(s string) error {
	val, err := UnmarshalPageSize(s)
	if err != nil {
		return err
	}
	*p = val
	return nil
}
func (p PageSize) Type() string { return "page-size" }

// Unmarshal helpers
func UnmarshalPageSize(pageSize string) (PageSize, error) {
	var ps PageSize
	if err := (&ps).UnmarshalText([]byte(pageSize)); err != nil {
		return 0, fmt.Errorf("provided invalid page size: %w", err)
	}
	return ps, nil
}
