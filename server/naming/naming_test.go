package naming

import (
	"reflect"
	"testing"
)

func TestWords(t *testing.T) {
	tests := map[string][]string{
		"ID":                   {"ID"},
		"UserID":               {"User", "ID"},
		"userID":               {"user", "ID"},
		"HTTPServer":           {"HTTP", "Server"},
		"AccessTokenExpiresAt": {"Access", "Token", "Expires", "At"},
		"UserIDs":              {"User", "IDs"},
		"URLs":                 {"URLs"},
		"Address2":             {"Address2"},
		"V2API":                {"V2", "API"},
		"created_at":           {"created", "at"},
		"Name":                 {"Name"},
		"x":                    {"x"},
		"APIKey":               {"API", "Key"},
		"IsAdmin":              {"Is", "Admin"},
	}
	for in, want := range tests {
		if got := Words(in); !reflect.DeepEqual(got, want) {
			t.Errorf("Words(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		name string
		want map[Style]string
	}{
		{"UserID", map[Style]string{
			Snake: "user_id", Camel: "userId", Pascal: "UserId",
			ScreamingSnake: "USER_ID", Kebab: "user-id", Header: "User-Id",
		}},
		{"HTTPServer", map[Style]string{
			Snake: "http_server", Camel: "httpServer", Pascal: "HttpServer",
			ScreamingSnake: "HTTP_SERVER", Kebab: "http-server", Header: "Http-Server",
		}},
		{"ID", map[Style]string{
			Snake: "id", Camel: "id", Pascal: "Id",
			ScreamingSnake: "ID", Kebab: "id", Header: "Id",
		}},
		{"AccessTokenExpiresAt", map[Style]string{
			Snake: "access_token_expires_at", Camel: "accessTokenExpiresAt",
			ScreamingSnake: "ACCESS_TOKEN_EXPIRES_AT", Header: "Access-Token-Expires-At",
		}},
		{"UserIDs", map[Style]string{Snake: "user_ids", Camel: "userIds"}},
	}
	for _, tt := range tests {
		for style, want := range tt.want {
			if got := Apply(style, tt.name); got != want {
				t.Errorf("Apply(%s, %q) = %q, want %q", style, tt.name, got, want)
			}
		}
	}
}
