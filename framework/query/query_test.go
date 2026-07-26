package query

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/gin-gonic/gin"
)

func testContext(t *testing.T, rawQuery string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/list?"+rawQuery, nil)
	return c
}

func taskOptions() Options {
	return Options{
		AllowedFilters: []Filter{
			Exact("completed").Bool(),
			Partial("title"),
			In("priority"),
			Compare("created_at"),
			Exact("state").Column("status_col"),
		},
		AllowedSorts: []Sort{SortBy("created_at"), SortBy("title").Column("title_col")},
		DefaultSort:  "-created_at",
	}
}

func TestParseAcceptsDeclaredParameters(t *testing.T) {
	result, err := Parse(testContext(t,
		"filter[completed]=true&filter[title]=report&filter[priority]=1,2"+
			"&filter[created_at][gte]=2026-01-01&filter[state]=open"+
			"&sort=-created_at,title&page=3&per_page=10&unrelated=ignored"), taskOptions())
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]FilterValue{}
	for _, filter := range result.Filters {
		byName[filter.Name] = filter
	}
	if byName["completed"].Op != OpEq || byName["title"].Op != OpLike {
		t.Fatalf("unexpected operators: %+v", byName)
	}
	if in := byName["priority"]; in.Op != OpIn || len(in.Values) != 2 {
		t.Fatalf("unexpected in filter: %+v", in)
	}
	if compare := byName["created_at"]; compare.Op != OpGte || compare.Values[0] != "2026-01-01" {
		t.Fatalf("unexpected compare filter: %+v", compare)
	}
	if byName["state"].Column != "status_col" {
		t.Fatalf("column mapping not applied: %+v", byName["state"])
	}
	if len(result.Sorts) != 2 || !result.Sorts[0].Desc || result.Sorts[0].Column != "created_at" ||
		result.Sorts[1].Desc || result.Sorts[1].Column != "title_col" {
		t.Fatalf("unexpected sorts: %+v", result.Sorts)
	}
	if result.Page != 3 || result.PerPage != 10 || result.Offset() != 20 {
		t.Fatalf("unexpected pagination: %+v", result)
	}
}

func TestParseDefaults(t *testing.T) {
	result, err := Parse(testContext(t, ""), taskOptions())
	if err != nil {
		t.Fatal(err)
	}
	if result.Page != 1 || result.PerPage != 25 {
		t.Fatalf("unexpected pagination defaults: %+v", result)
	}
	if len(result.Sorts) != 1 || !result.Sorts[0].Desc || result.Sorts[0].Column != "created_at" {
		t.Fatalf("default sort not applied: %+v", result.Sorts)
	}
	if len(result.Filters) != 0 {
		t.Fatalf("unexpected filters: %+v", result.Filters)
	}
}

func TestParseRejections(t *testing.T) {
	for _, test := range []struct {
		name     string
		query    string
		offender string
	}{
		{"unknown filter", "filter[secret]=1", "filter[secret]"},
		{"operator on exact filter", "filter[completed][gte]=1", "filter[completed][gte]"},
		{"unsupported operator", "filter[created_at][within]=1", "filter[created_at][within]"},
		{"malformed nesting", "filter[a][b][c]=1", "filter[a][b][c]"},
		{"empty name", "filter[]=1", "filter[]"},
		{"bare filter key", "filter=1", "filter"},
		{"non-boolean value for bool filter", "filter[completed]=maybe", "filter[completed]"},
		{"unknown sort", "sort=secret", "sort=secret"},
		{"page zero", "page=0", "page"},
		{"page not a number", "page=abc", "page"},
		{"negative per_page", "per_page=-2", "per_page"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(testContext(t, test.query), taskOptions())
			var public *httpx.Error
			if !errors.As(err, &public) || public.Status != http.StatusBadRequest || public.Code != "invalid_query" {
				t.Fatalf("expected invalid_query error, got %v", err)
			}
			details, ok := public.Details.(gin.H)
			if !ok || !strings.Contains(strings.Join(details["invalid_parameters"].([]string), ";"), test.offender) {
				t.Fatalf("offender %q missing from details: %+v", test.offender, public.Details)
			}
		})
	}
}

func TestParseClampsAndSkips(t *testing.T) {
	result, err := Parse(testContext(t, "per_page=9999&filter[title]="), taskOptions())
	if err != nil {
		t.Fatal(err)
	}
	if result.PerPage != 100 {
		t.Fatalf("per_page not clamped: %d", result.PerPage)
	}
	if len(result.Filters) != 0 {
		t.Fatalf("empty filter value should be skipped: %+v", result.Filters)
	}
}

func TestParseRejectsOversizedInList(t *testing.T) {
	values := strings.TrimSuffix(strings.Repeat("v,", maxInValues+1), ",")
	_, err := Parse(testContext(t, "filter[priority]="+values), taskOptions())
	var public *httpx.Error
	if !errors.As(err, &public) || public.Code != "invalid_query" {
		t.Fatalf("expected invalid_query error, got %v", err)
	}
}

func TestParseConfigurationErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		options Options
	}{
		{"invalid filter column", Options{AllowedFilters: []Filter{Exact("a").Column("a; DROP TABLE")}}},
		{"invalid sort column", Options{AllowedSorts: []Sort{SortBy("a").Column("b--")}}},
		{"default sort not allowed", Options{AllowedSorts: []Sort{SortBy("title")}, DefaultSort: "-created_at"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(testContext(t, ""), test.options)
			var public *httpx.Error
			if !errors.As(err, &public) || public.Status != http.StatusInternalServerError ||
				public.Code != "query_configuration_invalid" {
				t.Fatalf("expected configuration error, got %v", err)
			}
		})
	}
}

func TestMeta(t *testing.T) {
	result := Result{Page: 2, PerPage: 25}
	for _, test := range []struct {
		total int64
		pages int64
	}{{0, 0}, {1, 1}, {25, 1}, {26, 2}, {101, 5}} {
		meta := result.Meta(test.total)
		if meta.TotalPages != test.pages || meta.Total != test.total || meta.Page != 2 || meta.PerPage != 25 {
			t.Fatalf("Meta(%d) = %+v", test.total, meta)
		}
	}
}
