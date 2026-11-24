package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mio-cc/anywhere-door/gen"
)

const schemaBase = "https://anydoor.local/schema/"

// errorSchema 编译 schema/errors.schema.json#/$defs/ErrorResponse（跨文件引用只从本地 schema/ 解析）。
func errorSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "schema", "*.schema.json"))
	must(t, err)
	if len(paths) == 0 {
		t.Fatal("找不到 schema/*.schema.json")
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	for _, p := range paths {
		b, err := os.ReadFile(p)
		must(t, err)
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		must(t, err)
		must(t, c.AddResource(schemaBase+filepath.Base(p), doc))
	}
	sch, err := c.Compile(schemaBase + "errors.schema.json#/$defs/ErrorResponse")
	must(t, err)
	return sch
}

func TestErrors(t *testing.T) {
	t.Run("B0305-T8 错误响应", func(t *testing.T) {
		sch := errorSchema(t)
		msgs, err := loadMessages()
		must(t, err)
		f := newFixture(t, false)
		cookie := f.login()
		cases := []struct {
			name string
			rec  *httptest.ResponseRecorder
			code gen.ApiErrorCode
		}{
			{"400", f.do(http.MethodGet, "/api/v1/test/bad", nil, withCookie(cookie)), gen.ErrCodeBadRequest},
			{"401", f.do(http.MethodGet, "/api/v1/test/ping", nil), gen.ErrCodeUnauthorized},
			{"403", f.do(http.MethodPost, "/api/v1/test/echo", nil, withCookie(cookie), withOrigin("http://evil.example")), gen.ErrCodeOriginRejected},
			{"403 forbidden", f.do(http.MethodGet, "/api/v1/test/forbidden", nil, withCookie(cookie)), gen.ErrCodeForbidden},
			{"404", f.do(http.MethodGet, "/api/v1/nope", nil, withCookie(cookie)), gen.ErrCodeNotFound},
			{"413", f.do(http.MethodPost, "/api/v1/test/echo", make([]byte, maxBody+1), withCookie(cookie), withOrigin(localOrigin)), gen.ErrCodeTooLarge},
			{"500", f.do(http.MethodGet, "/api/v1/test/panic", nil, withCookie(cookie)), gen.ErrCodeInternal},
		}
		wantMsg := map[gen.ApiErrorCode]string{
			gen.ErrCodeBadRequest: strings.ReplaceAll(msgs[gen.ErrCodeBadRequest], "{errors}", "/name：缺少 name；/limit：limit 太大"),
			gen.ErrCodeForbidden:  strings.ReplaceAll(msgs[gen.ErrCodeForbidden], "{reasons}", "只有用户能做；测试"),
			gen.ErrCodeTooLarge:   strings.ReplaceAll(msgs[gen.ErrCodeTooLarge], "{limit}", "1MB"),
			gen.ErrCodeNotFound:   strings.ReplaceAll(msgs[gen.ErrCodeNotFound], "{resource}", "接口"),
		}
		for _, c := range cases {
			if c.rec.Code != statusOf[c.code] {
				t.Errorf("%s：状态码 %d", c.name, c.rec.Code)
			}
			if ct := c.rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Errorf("%s：Content-Type 是 %q", c.name, ct)
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(c.rec.Body.Bytes()))
			must(t, err)
			if err := sch.Validate(doc); err != nil {
				t.Errorf("%s：不符合 ErrorResponse：%v\n%s", c.name, err, c.rec.Body.String())
			}
			e := errOf(t, c.rec)
			want, ok := wantMsg[c.code]
			if !ok {
				want = msgs[c.code]
			}
			if e.Code != c.code || e.Message != want {
				t.Errorf("%s：得到 %s「%s」，应当是 %s「%s」", c.name, e.Code, e.Message, c.code, want)
			}
			if strings.Contains(e.Message, "{") {
				t.Errorf("%s：文案里还有没替换的变量：%s", c.name, e.Message)
			}
			if e.RequestID != c.rec.Header().Get("X-Request-Id") {
				t.Errorf("%s：request_id %q 与响应头 %q 不同", c.name, e.RequestID, c.rec.Header().Get("X-Request-Id"))
			}
		}
		var nf struct {
			Error struct {
				Details map[string]string `json:"details"`
			} `json:"error"`
		}
		must(t, json.Unmarshal(cases[4].rec.Body.Bytes(), &nf))
		if d := nf.Error.Details; d["resource"] != "接口" || d["id"] != "/api/v1/nope" {
			t.Errorf("404 的 details 应当是 resource、id（C.11 §3），得到 %v", d)
		}
		// details 的形状照 C.11 §3（JSON Schema 不约束 details，这里逐个查）
		detailsOf := func(rec *httptest.ResponseRecorder) map[string]json.RawMessage {
			var v struct {
				Error struct {
					Details map[string]json.RawMessage `json:"details"`
				} `json:"error"`
			}
			must(t, json.Unmarshal(rec.Body.Bytes(), &v))
			return v.Error.Details
		}
		var fes []FieldError
		must(t, json.Unmarshal(detailsOf(cases[0].rec)["errors"], &fes))
		if len(fes) != 2 || fes[0] != (FieldError{Path: "/name", Message: "缺少 name"}) || fes[1].Path != "/limit" {
			t.Errorf("400 的 details.errors 应当是 [{path, message}]，得到 %+v", fes)
		}
		fd := detailsOf(cases[3].rec)
		var reasons []string
		must(t, json.Unmarshal(fd["reasons"], &reasons))
		if string(fd["decision"]) != `"deny"` || string(fd["action"]) != `"GET /api/v1/test/forbidden"` ||
			string(fd["target"]) != `"anydoor"` || len(reasons) != 2 || len(fd) != 4 {
			t.Errorf("403 forbidden 的 details 应当是 decision、reasons、action、target，得到 %v", fd)
		}
		for _, i := range []int{1, 2} {
			if d := detailsOf(cases[i].rec); d != nil {
				t.Errorf("%s 不应当有 details：%v", cases[i].name, d)
			}
		}
		last := cases[len(cases)-1].rec
		var raw map[string]map[string]json.RawMessage
		must(t, json.Unmarshal(last.Body.Bytes(), &raw))
		if _, ok := raw["error"]["details"]; ok {
			t.Error("500 不应当有 details")
		}
		if strings.Contains(last.Body.String(), panicText) {
			t.Error("500 的响应里不能有 panic 的原文")
		}
		if got := render("上限 {limit}，还有 {unknown}", map[string]string{"limit": "1MB"}); got != "上限 1MB，还有 {unknown}" {
			t.Errorf("render 得到 %q", got)
		}
	})
}
