package codegen

import "testing"

func TestGoName(t *testing.T) {
	cases := map[string]string{
		"task_id":     "TaskID",  // initialism upper-cased
		"user_id":     "UserID",  // initialism upper-cased
		"created_at":  "CreatedAt",
		"api_url":     "APIURL",  // consecutive initialisms
		"name":        "Name",
		"_user_id":    "UserID",  // leading underscore skipped
		"http_status": "HTTPStatus",
	}
	for in, want := range cases {
		if got := goName(in); got != want {
			t.Errorf("goName(%q) = %q, want %q", in, got, want)
		}
	}
}

// lowerCamel must NOT apply initialisms and must lower-case the first word, so
// query argument names stay stable (user_id -> userId, not userID).
func TestLowerCamel(t *testing.T) {
	cases := map[string]string{
		"user_id":    "userId",
		"task_id":    "taskId",
		"created_at": "createdAt",
		"email":      "email",
		"_user_id":   "userId", // leading underscore: first real word still lowered
		"api_url":    "apiUrl",
	}
	for in, want := range cases {
		if got := lowerCamel(in); got != want {
			t.Errorf("lowerCamel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLowerFirst(t *testing.T) {
	cases := map[string]string{
		"GetUser": "getUser",
		"ID":      "iD",
		"":        "",
		"a":       "a",
	}
	for in, want := range cases {
		if got := lowerFirst(in); got != want {
			t.Errorf("lowerFirst(%q) = %q, want %q", in, got, want)
		}
	}
}
