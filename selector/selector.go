package selector

import "errors"

const (
	ByUndefined ByType = iota
	ByID
	ByQueryAll
	ByQuery
	ByNodeID
	ByJSPath
	BySearch
)

// ByType is a 'By' selector type enumerator.
type ByType uint

// Validate checks validity of the ByType.
func (b ByType) Validate() error {
	if b >= ByID && b <= BySearch {
		return nil
	}
	return errors.New("invalid by selector")
}
