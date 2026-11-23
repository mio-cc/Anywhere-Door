package main

import (
	"reflect"
	"strings"
	"testing"
)

var (
	wantAPICodes = []string{"bad_request", "unauthorized", "forbidden", "node_locked", "origin_rejected", "not_found",
		"graph_conflict", "idempotency_mismatch", "proposal_stale", "approval_closed", "run_state", "too_large",
		"invalid_graph", "extension_invalid", "estopped", "rate_limited", "internal", "provider_error", "provider_unavailable"}
	wantFailureClasses = []string{"transient", "rate_limited", "format_invalid", "budget_exceeded", "auth_denied",
		"user_input", "structural", "timeout", "cancelled", "internal"}
	// 诊断码 → 允许的级别（C.11 §5 表）
	wantDiagLevels = map[string][]string{
		"V01": {"error"}, "V02": {"error"}, "V03": {"error"}, "V04": {"error"}, "V05": {"error"}, "V06": {"error"},
		"V07": {"error"}, "V08": {"error"}, "V09": {"error"}, "V10": {"error"}, "V11": {"error"}, "V12": {"error", "warn"},
		"V13": {"error"}, "V14": {"error"}, "V15": {"warn"}, "V16": {"error"}, "V17": {"warn"}, "V18": {"warn"},
	}
)

func TestErrorEnums(t *testing.T) {
	t.Run("C11-T1 枚举", func(t *testing.T) {
		s := repoSet(t)
		if got := enumOf(t, s, "errors.schema.json", "ApiErrorCode"); !reflect.DeepEqual(got, wantAPICodes) {
			t.Errorf("ApiErrorCode：%v\n应为 C.11 §3：%v", got, wantAPICodes)
		}
		if got := enumOf(t, s, "errors.schema.json", "FailureClass"); !reflect.DeepEqual(got, wantFailureClasses) {
			t.Errorf("FailureClass：%v\n应为 C.11 §4：%v", got, wantFailureClasses)
		}
		if got := enumOf(t, s, "errors.schema.json", "DiagCode"); !reflect.DeepEqual(got, sortedKeys(wantDiagLevels)) {
			t.Errorf("DiagCode：%v\n应为 C.11 §5：%v", got, sortedKeys(wantDiagLevels))
		}
		if got := enumOf(t, s, "errors.schema.json", "DiagLevel"); !reflect.DeepEqual(got, []string{"error", "warn"}) {
			t.Errorf("DiagLevel：%v", got)
		}
	})
}

func TestErrorMessages(t *testing.T) {
	t.Run("C11-T2 错误文案", func(t *testing.T) {
		var msgs map[string]string
		readYAML(t, "schema/builtin/messages/errors.zh.yaml", &msgs)
		want := append([]string(nil), wantAPICodes...)
		if got := sortedKeys(msgs); !reflect.DeepEqual(got, sortedStrings(want)) {
			t.Errorf("errors.zh.yaml 的键：%v\n应当恰好是每个 ApiErrorCode：%v", got, sortedStrings(want))
		}
		for code, msg := range msgs {
			if strings.TrimSpace(msg) == "" {
				t.Errorf("%s 的文案是空的", code)
			}
		}
	})
	t.Run("C11-T3 诊断文案", func(t *testing.T) {
		var msgs map[string]map[string]string
		readYAML(t, "schema/builtin/messages/diags.zh.yaml", &msgs)
		if got := sortedKeys(msgs); !reflect.DeepEqual(got, sortedKeys(wantDiagLevels)) {
			t.Errorf("diags.zh.yaml 的键：%v\n应当恰好是每个 DiagCode", got)
		}
		for _, code := range sortedKeys(wantDiagLevels) {
			if got := sortedKeys(msgs[code]); !reflect.DeepEqual(got, wantDiagLevels[code]) {
				t.Errorf("%s 的级别：%v，应为 %v（C.11 §5）", code, got, wantDiagLevels[code])
			}
			for level, msg := range msgs[code] {
				if strings.TrimSpace(msg) == "" {
					t.Errorf("%s.%s 的文案是空的", code, level)
				}
			}
		}
	})
}

func sortedStrings(in []string) []string {
	m := map[string]bool{}
	for _, s := range in {
		m[s] = true
	}
	return sortedKeys(m)
}

func TestErrorExamples(t *testing.T) {
	t.Run("C11-T4 示例", func(t *testing.T) {
		s := repoSet(t)
		for file, ref := range map[string]string{
			"errors/graph_conflict.json":      "errors.schema.json#/$defs/ErrorResponse",
			"errors/diag_v12_warn.json":       "errors.schema.json#/$defs/Diag",
			"invalid/error_unknown_code.json": "errors.schema.json#/$defs/ErrorResponse",
			"invalid/diag_code_v99.json":      "errors.schema.json#/$defs/Diag",
		} {
			if e := manifestEntry(t, s, file); e.Schema != ref {
				t.Errorf("清单里 %s 按 %s 校验，应为 %s", file, e.Schema, ref)
			}
		}
		const resp = "errors.schema.json#/$defs/ErrorResponse"
		const req = `"request_id": "01JDREQ0000000000000000001"`
		mustValid(t, s, resp, `{"error": {"code": "internal", "message": "出错了", `+req+`, "details": {"anything": [1, {"x": 2}]}}}`)
		mustInvalid(t, s, resp, `{"error": {"code": "internal", "message": "出错了", `+req+`, "stack": "…"}}`, "/error")
		mustInvalid(t, s, resp, `{"error": {"code": "internal", "message": "出错了"}}`, "/error")
		mustInvalid(t, s, resp, `{"error": {"code": "internal", "message": "出错了", `+req+`}, "extra": 1}`, "")
		const diag = "errors.schema.json#/$defs/Diag"
		mustValid(t, s, diag, `{"level": "error", "code": "V05", "node": "01JDN0000000000000000000A1", "port": "in", "msg": "连线的端口不对"}`)
		mustInvalid(t, s, diag, `{"level": "info", "code": "V05", "msg": "x"}`, "/level")
		mustInvalid(t, s, diag, `{"level": "error", "code": "V05", "msg": "x", "hint": "y"}`, "")
	})
}
