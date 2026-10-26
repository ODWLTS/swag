package external

import "github.com/ODWLTS/swag/testdata/lazy_dependency/external/nested"

// Thing is referenced by a handler annotation but not imported by main.
type Thing struct {
	ID     int           `json:"id"`
	Detail nested.Detail `json:"detail"`
}
