package query

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/gin-gonic/gin"
)

func cursorOptions() Options {
	options := taskOptions()
	options.CursorSort = "created_at"
	return options
}

func cursorResult(desc bool, cursor *Cursor) Result {
	return Result{
		CursorMode: true,
		Cursor:     cursor,
		Sorts:      []SortValue{{Column: "created_at", Desc: desc}, {Column: "id", Desc: desc}},
		PerPage:    10,
	}
}

func TestCursorEncodeDecodeRoundTrip(t *testing.T) {
	original := Cursor{Value: "2026-01-01T10:20:30.000000004Z", ID: "42"}
	decoded, err := DecodeCursor(EncodeCursor(original))
	if err != nil {
		t.Fatal(err)
	}
	if decoded != original {
		t.Fatalf("round trip = %+v, want %+v", decoded, original)
	}
}

func TestDecodeCursorRejectsMalformedTokens(t *testing.T) {
	for name, raw := range map[string]string{
		"invalid base64url": "%%%not-base64%%%",
		"invalid JSON":      base64.RawURLEncoding.EncodeToString([]byte("not json")),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeCursor(raw); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestParseCursorModeFirstPage(t *testing.T) {
	result, err := Parse(testContext(t, "per_page=5&filter[completed]=true"), cursorOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !result.CursorMode || result.Cursor != nil {
		t.Fatalf("unexpected cursor state: %+v", result)
	}
	wantSorts := []SortValue{{Column: "created_at", Desc: true}, {Column: "id", Desc: true}}
	if !reflect.DeepEqual(result.Sorts, wantSorts) {
		t.Fatalf("sorts = %+v, want %+v", result.Sorts, wantSorts)
	}
	if result.PerPage != 5 || result.CursorLimit() != 6 {
		t.Fatalf("unexpected pagination: %+v", result)
	}
	if len(result.Filters) != 1 {
		t.Fatalf("filters should still apply: %+v", result.Filters)
	}
}

func TestParseCursorModeDecodesCursor(t *testing.T) {
	token := EncodeCursor(Cursor{Value: "2026-01-01", ID: "7"})
	result, err := Parse(testContext(t, "cursor="+token), cursorOptions())
	if err != nil {
		t.Fatal(err)
	}
	if result.Cursor == nil || result.Cursor.Value != "2026-01-01" || result.Cursor.ID != "7" {
		t.Fatalf("cursor not decoded: %+v", result.Cursor)
	}
}

func TestParseCursorModeUsesColumnMappingAndAscendingDefault(t *testing.T) {
	options := taskOptions()
	options.CursorSort = "title"
	options.DefaultSort = "title"
	result, err := Parse(testContext(t, ""), options)
	if err != nil {
		t.Fatal(err)
	}
	wantSorts := []SortValue{{Column: "title_col", Desc: false}, {Column: "id", Desc: false}}
	if !reflect.DeepEqual(result.Sorts, wantSorts) {
		t.Fatalf("sorts = %+v, want %+v", result.Sorts, wantSorts)
	}
}

func TestParseCursorModeRejections(t *testing.T) {
	for _, test := range []struct {
		name     string
		query    string
		offender string
	}{
		{"page parameter", "page=2", "page"},
		{"sort parameter", "sort=title", "sort"},
		{"malformed cursor", "cursor=@@@", "cursor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(testContext(t, test.query), cursorOptions())
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

func TestParseCursorSortMustBeAllowed(t *testing.T) {
	options := Options{AllowedSorts: []Sort{SortBy("title")}, CursorSort: "created_at"}
	_, err := Parse(testContext(t, ""), options)
	var public *httpx.Error
	if !errors.As(err, &public) || public.Status != http.StatusInternalServerError ||
		public.Code != "query_configuration_invalid" {
		t.Fatalf("expected configuration error, got %v", err)
	}
}

func TestBuildCursorSQLFirstPage(t *testing.T) {
	statement, args := cursorResult(true, nil).BuildCursorSQL("SELECT id FROM tasks")
	want := "SELECT id FROM tasks ORDER BY created_at DESC, id DESC LIMIT ?"
	if statement != want || !reflect.DeepEqual(args, []any{11}) {
		t.Fatalf("sql: %q args=%#v", statement, args)
	}
}

func TestBuildCursorSQLKeyset(t *testing.T) {
	cursor := &Cursor{Value: "2026-01-01", ID: "7"}
	wantArgs := []any{"2026-01-01", "2026-01-01", "7", 11}

	statement, args := cursorResult(true, cursor).BuildCursorSQL("SELECT id FROM tasks")
	wantDesc := "SELECT id FROM tasks WHERE (created_at < ? OR (created_at = ? AND id < ?)) " +
		"ORDER BY created_at DESC, id DESC LIMIT ?"
	if statement != wantDesc || !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("desc sql: %q args=%#v", statement, args)
	}

	statement, args = cursorResult(false, cursor).BuildCursorSQL("SELECT id FROM tasks")
	wantAsc := "SELECT id FROM tasks WHERE (created_at > ? OR (created_at = ? AND id > ?)) " +
		"ORDER BY created_at ASC, id ASC LIMIT ?"
	if statement != wantAsc || !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("asc sql: %q args=%#v", statement, args)
	}
}

func TestBuildCursorSQLComposesWithFilters(t *testing.T) {
	result := cursorResult(true, &Cursor{Value: "2026-01-01", ID: "7"})
	result.Filters = []FilterValue{{Column: "completed", Op: OpEq, Values: []string{"true"}, Bool: true}}
	statement, args := result.BuildCursorSQL("SELECT id FROM tasks")
	want := "SELECT id FROM tasks WHERE completed = ? AND (created_at < ? OR (created_at = ? AND id < ?)) " +
		"ORDER BY created_at DESC, id DESC LIMIT ?"
	wantArgs := []any{true, "2026-01-01", "2026-01-01", "7", 11}
	if statement != want || !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("sql: %q args=%#v", statement, args)
	}
}

func TestNextCursorTrimsProbeRow(t *testing.T) {
	result := Result{PerPage: 3}
	page, next := NextCursor(result, []string{"a", "b", "c", "d"}, func(item string) (string, string) {
		return "v-" + item, "id-" + item
	})
	if !reflect.DeepEqual(page, []string{"a", "b", "c"}) {
		t.Fatalf("page = %v", page)
	}
	cursor, err := DecodeCursor(next)
	if err != nil || cursor.Value != "v-c" || cursor.ID != "id-c" {
		t.Fatalf("next cursor = %+v, err = %v", cursor, err)
	}
}

func TestNextCursorShortPageEndsWalk(t *testing.T) {
	result := Result{PerPage: 3}
	page, next := NextCursor(result, []string{"a", "b"}, func(item string) (string, string) {
		return item, item
	})
	if next != "" || !reflect.DeepEqual(page, []string{"a", "b"}) {
		t.Fatalf("page = %v, next = %q", page, next)
	}
}

func TestCursorMeta(t *testing.T) {
	result := Result{PerPage: 10}

	meta := result.CursorMeta("")
	if meta.NextCursor != nil || meta.PerPage != 10 {
		t.Fatalf("last-page meta = %+v", meta)
	}
	payload, err := json.Marshal(meta)
	if err != nil || string(payload) != `{"next_cursor":null,"per_page":10}` {
		t.Fatalf("json = %s, err = %v", payload, err)
	}

	meta = result.CursorMeta("token")
	if meta.NextCursor == nil || *meta.NextCursor != "token" {
		t.Fatalf("meta = %+v", meta)
	}
}
