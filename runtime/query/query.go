// Package query provides allowlist-based filtering, sorting, and pagination
// for list endpoints, driven by bracketed query parameters.
//
// URL contract:
//
//	GET /tasks?filter[completed]=true&filter[title]=report
//	          &filter[created_at][gte]=2026-01-01
//	          &sort=-created_at,title&page=2&per_page=25
//
// Only developer-declared filters and sorts are accepted; unknown names,
// unsupported operators, and malformed pagination values are rejected with a
// canonical 400 invalid_query error. Column names reach SQL exclusively from
// the allowlist and user values only ever bind as arguments.
package query

import (
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/gin-gonic/gin"
)

// Op identifies a filter operator.
type Op string

const (
	OpEq   Op = "eq"
	OpLike Op = "like"
	OpIn   Op = "in"
	OpGte  Op = "gte"
	OpLte  Op = "lte"
	OpGt   Op = "gt"
	OpLt   Op = "lt"
)

type filterKind int

const (
	kindExact filterKind = iota
	kindPartial
	kindIn
	kindCompare
)

// maxInValues bounds filter[x]=a,b,c,... lists to keep query size sane.
const maxInValues = 100

// Filter declares one allowed filter parameter.
type Filter struct {
	name      string
	column    string
	kind      filterKind
	boolValue bool
}

// Exact matches filter[name]=value as column = value.
func Exact(name string) Filter { return Filter{name: name, column: name, kind: kindExact} }

// Partial matches filter[name]=value as a contains search (LIKE, escaped).
func Partial(name string) Filter { return Filter{name: name, column: name, kind: kindPartial} }

// In matches filter[name]=a,b,c as column IN (a, b, c).
func In(name string) Filter { return Filter{name: name, column: name, kind: kindIn} }

// Compare matches filter[name]=v as equality and filter[name][gte|lte|gt|lt]=v
// as range comparisons.
func Compare(name string) Filter { return Filter{name: name, column: name, kind: kindCompare} }

// Column maps the public parameter name to a different database column.
func (f Filter) Column(column string) Filter {
	f.column = column
	return f
}

// Bool declares the filter value as boolean: accepted values parse with
// strconv.ParseBool and bind as native booleans, which compares correctly
// against boolean columns on every supported database.
func (f Filter) Bool() Filter {
	f.boolValue = true
	return f
}

// Sort declares one allowed sort field.
type Sort struct {
	name   string
	column string
}

// SortBy allows sort=name and sort=-name.
func SortBy(name string) Sort { return Sort{name: name, column: name} }

// Column maps the public sort name to a different database column.
func (s Sort) Column(column string) Sort {
	s.column = column
	return s
}

// Options declares what a list endpoint accepts.
type Options struct {
	AllowedFilters []Filter
	AllowedSorts   []Sort
	// DefaultSort is applied when the request has no sort parameter, e.g.
	// "-created_at". It must reference an allowed sort.
	DefaultSort string
	// DefaultPerPage defaults to 25.
	DefaultPerPage int
	// MaxPerPage clamps per_page and defaults to 100.
	MaxPerPage int
	// CursorSort switches the endpoint to cursor (keyset) pagination on the
	// named allowed sort. In cursor mode the page and sort parameters are
	// rejected, the sort is forced to CursorSort (descending when DefaultSort
	// starts with "-"), rows are tie-broken on the id column, and the cursor
	// parameter carries the keyset position.
	CursorSort string
}

// FilterValue is one accepted filter with its bound values.
type FilterValue struct {
	Name   string
	Column string
	Op     Op
	Values []string
	// Bool marks values that bind as native booleans.
	Bool bool
}

// arg converts one raw value into its bind argument.
func (f FilterValue) arg(value string) any {
	if f.Bool {
		parsed, _ := strconv.ParseBool(value)
		return parsed
	}
	return value
}

// SortValue is one accepted sort field.
type SortValue struct {
	Column string
	Desc   bool
}

// Result is a validated, normalized query ready to apply to a data source.
type Result struct {
	Filters []FilterValue
	Sorts   []SortValue
	Page    int
	PerPage int
	// CursorMode reports that the endpoint uses cursor (keyset) pagination.
	CursorMode bool
	// Cursor is the decoded keyset position, or nil on the first page.
	Cursor *Cursor
}

// Offset returns the row offset for the current page.
func (r Result) Offset() int { return (r.Page - 1) * r.PerPage }

// Meta is the standard pagination metadata for httpx.List.
type Meta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

// Meta builds pagination metadata from a total row count.
func (r Result) Meta(total int64) Meta {
	pages := int64(0)
	if total > 0 {
		pages = int64(math.Ceil(float64(total) / float64(r.PerPage)))
	}
	return Meta{Page: r.Page, PerPage: r.PerPage, Total: total, TotalPages: pages}
}

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)
	filterKeyPattern  = regexp.MustCompile(`^filter\[([^\[\]]+)\](?:\[([^\[\]]+)\])?$`)
)

// Parse validates the request's filter, sort, page, and per_page parameters
// against options. Rejections return a 400 *httpx.Error with code
// invalid_query listing the offending parameters; allowlist configuration
// mistakes return a 500-class *httpx.Error. When Options.CursorSort is set
// the endpoint uses cursor (keyset) pagination instead: page and sort are
// rejected and the cursor parameter is decoded into Result.Cursor.
func Parse(c *gin.Context, options Options) (Result, error) {
	if options.DefaultPerPage < 1 {
		options.DefaultPerPage = 25
	}
	if options.MaxPerPage < 1 {
		options.MaxPerPage = 100
	}
	filters := make(map[string]Filter, len(options.AllowedFilters))
	for _, filter := range options.AllowedFilters {
		if !identifierPattern.MatchString(filter.column) {
			return Result{}, configError(fmt.Sprintf("filter %q maps to invalid column %q", filter.name, filter.column))
		}
		filters[filter.name] = filter
	}
	sorts := make(map[string]Sort, len(options.AllowedSorts))
	for _, sort := range options.AllowedSorts {
		if !identifierPattern.MatchString(sort.column) {
			return Result{}, configError(fmt.Sprintf("sort %q maps to invalid column %q", sort.name, sort.column))
		}
		sorts[sort.name] = sort
	}

	result := Result{Page: 1, PerPage: options.DefaultPerPage}
	var invalid []string

	values := c.Request.URL.Query()
	for key, keyValues := range values {
		if key != "filter" && !strings.HasPrefix(key, "filter[") {
			continue
		}
		match := filterKeyPattern.FindStringSubmatch(key)
		if match == nil {
			invalid = append(invalid, key)
			continue
		}
		name, operator := match[1], match[2]
		filter, allowed := filters[name]
		if !allowed {
			invalid = append(invalid, key)
			continue
		}
		value := keyValues[0]
		if value == "" {
			continue
		}
		filterValue, err := buildFilterValue(filter, operator, value)
		if err != nil {
			invalid = append(invalid, key)
			continue
		}
		result.Filters = append(result.Filters, filterValue)
	}

	sortParameter := c.Query("sort")
	if sortParameter == "" {
		sortParameter = options.DefaultSort
	}
	for _, field := range splitNonEmpty(sortParameter) {
		desc := strings.HasPrefix(field, "-")
		name := strings.TrimPrefix(field, "-")
		sort, allowed := sorts[name]
		if !allowed {
			if c.Query("sort") == "" {
				return Result{}, configError(fmt.Sprintf("default sort %q is not an allowed sort", name))
			}
			invalid = append(invalid, "sort="+field)
			continue
		}
		result.Sorts = append(result.Sorts, SortValue{Column: sort.column, Desc: desc})
	}

	if raw := c.Query("page"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 {
			invalid = append(invalid, "page")
		} else {
			result.Page = page
		}
	}
	if raw := c.Query("per_page"); raw != "" {
		perPage, err := strconv.Atoi(raw)
		if err != nil || perPage < 1 {
			invalid = append(invalid, "per_page")
		} else {
			result.PerPage = min(perPage, options.MaxPerPage)
		}
	}

	if options.CursorSort != "" {
		sort, allowed := sorts[options.CursorSort]
		if !allowed {
			return Result{}, configError(fmt.Sprintf("cursor sort %q is not an allowed sort", options.CursorSort))
		}
		result.CursorMode = true
		desc := strings.HasPrefix(options.DefaultSort, "-")
		result.Sorts = []SortValue{{Column: sort.column, Desc: desc}, {Column: "id", Desc: desc}}
		if c.Query("sort") != "" {
			invalid = append(invalid, "sort")
		}
		if c.Query("page") != "" {
			invalid = append(invalid, "page")
		}
		if raw := c.Query("cursor"); raw != "" {
			cursor, err := DecodeCursor(raw)
			if err != nil {
				invalid = append(invalid, "cursor")
			} else {
				result.Cursor = &cursor
			}
		}
	}

	if len(invalid) > 0 {
		return Result{}, &httpx.Error{
			Status:  http.StatusBadRequest,
			Code:    "invalid_query",
			Message: "The request query parameters are invalid.",
			Details: gin.H{"invalid_parameters": invalid},
		}
	}
	return result, nil
}

func buildFilterValue(filter Filter, operator, value string) (FilterValue, error) {
	filterValue := FilterValue{Name: filter.name, Column: filter.column, Bool: filter.boolValue}
	switch filter.kind {
	case kindExact, kindPartial, kindIn:
		if operator != "" {
			return FilterValue{}, fmt.Errorf("operator %q not supported", operator)
		}
	case kindCompare:
		switch Op(operator) {
		case "", OpGte, OpLte, OpGt, OpLt:
		default:
			return FilterValue{}, fmt.Errorf("operator %q not supported", operator)
		}
	}
	switch filter.kind {
	case kindExact:
		filterValue.Op = OpEq
		filterValue.Values = []string{value}
	case kindPartial:
		filterValue.Op = OpLike
		filterValue.Values = []string{value}
	case kindIn:
		filterValue.Op = OpIn
		filterValue.Values = splitNonEmpty(value)
		if len(filterValue.Values) == 0 || len(filterValue.Values) > maxInValues {
			return FilterValue{}, fmt.Errorf("in list must contain between 1 and %d values", maxInValues)
		}
	case kindCompare:
		filterValue.Op = OpEq
		if operator != "" {
			filterValue.Op = Op(operator)
		}
		filterValue.Values = []string{value}
	}
	if filter.boolValue {
		for _, item := range filterValue.Values {
			if _, err := strconv.ParseBool(item); err != nil {
				return FilterValue{}, fmt.Errorf("value %q is not a boolean", item)
			}
		}
	}
	return filterValue, nil
}

func configError(message string) *httpx.Error {
	return httpx.NewError(http.StatusInternalServerError, "query_configuration_invalid", "The endpoint's query configuration is invalid: "+message)
}

func splitNonEmpty(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
