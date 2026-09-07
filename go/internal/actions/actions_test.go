package actions

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

func TestConnectPrintOnlyLines(t *testing.T) {
	cases := []struct {
		name string
		a    Connect
		want []string
	}{
		{"plain", Connect{Alias: "web"}, []string{"connect", "web"}},
		{"recording", Connect{Alias: "web", RecordTo: "/tmp/web-20260906T120000.log"},
			[]string{"record", "web", "/tmp/web-20260906T120000.log"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.PrintOnlyLines(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("PrintOnlyLines() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSftpPrintOnlyLines(t *testing.T) {
	got := Sftp{Alias: "web"}.PrintOnlyLines()
	want := []string{"sftp", "web"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PrintOnlyLines() = %v, want %v", got, want)
	}
}

func TestScpPrintOnlyLines(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}

	cases := []struct {
		name string
		a    Scp
		want []string
	}{
		{
			name: "upload expands tilde",
			a:    Scp{Alias: "web", LocalPath: "~/file.txt", RemotePath: "/remote/file.txt", Upload: true},
			want: []string{"scp", "upload", "web", filepath.Join(home, "file.txt"), "/remote/file.txt"},
		},
		{
			name: "download leaves absolute path alone",
			a:    Scp{Alias: "web", LocalPath: "/tmp/file.txt", RemotePath: "/remote/file.txt", Upload: false},
			want: []string{"scp", "download", "web", "/tmp/file.txt", "/remote/file.txt"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.PrintOnlyLines(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("PrintOnlyLines() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSessionLogPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}
	path, err := SessionLogPath("web")
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(home, "Documents", "sshtui")
	if filepath.Dir(path) != wantDir {
		t.Errorf("SessionLogPath dir = %q, want %q", filepath.Dir(path), wantDir)
	}
	name := filepath.Base(path)
	if !regexp.MustCompile(`^web-\d{8}T\d{6}\.log$`).MatchString(name) {
		t.Errorf("SessionLogPath name = %q, doesn't match expected pattern", name)
	}
}
