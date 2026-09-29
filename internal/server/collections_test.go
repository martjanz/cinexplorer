package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"cinexplorer/internal/catalog"
	"cinexplorer/internal/grouping"
	"cinexplorer/internal/nameparse"
	"cinexplorer/internal/store"
)

const kubrick = "../cine/Collections/Kubrick"

// sendRec sends an HTTP request and returns the response recorder.
func sendRec(t *testing.T, s *Server, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	return request(s.Handler(), method, url, body, "application/json", "127.0.0.1")
}

// putKubrick leaves Amarcord (f1) plus n films in ../cine/Collections/Kubrick
// (fingerprints k1…kn) in the catalog.
func putKubrick(t *testing.T, s *Server, n int) {
	t.Helper()
	rows := []store.FileRow{{Path: moviePath, Size: 10, MTime: 1, Fingerprint: "f1", Kind: "video"}}
	vs := []grouping.Version{{Dir: "../cine", Parsed: nameparse.Parsed{Title: "Amarcord", Year: 1973}, Size: 10, Parts: 1,
		Members: []grouping.Member{{Path: moviePath, Role: grouping.RoleMain}}}}
	for i := 1; i <= n; i++ {
		p := fmt.Sprintf("%s/Film %d.mkv", kubrick, i)
		rows = append(rows, store.FileRow{Path: p, Size: 10, MTime: 1, Fingerprint: fmt.Sprintf("k%d", i), Kind: "video"})
		vs = append(vs, grouping.Version{Dir: kubrick, Parsed: nameparse.Parsed{Title: fmt.Sprintf("Film %d", i)}, Size: 10, Parts: 1,
			Members: []grouping.Member{{Path: p, Role: grouping.RoleMain}}})
	}
	if err := s.Store.SyncFiles(rows, []string{"../cine"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.ReplaceVersions(vs); err != nil {
		t.Fatal(err)
	}
}

func folders(t *testing.T, s *Server) []catalog.CollectionFolder {
	t.Helper()
	var fs []catalog.CollectionFolder
	if code := getJSON(t, s, "/api/collections", &fs); code != 200 {
		t.Fatalf("collections %d", code)
	}
	return fs
}

func listCount(t *testing.T, s *Server) int {
	t.Helper()
	var cards []catalog.ListCard
	getJSON(t, s, "/api/lists", &cards)
	if len(cards) != 1 {
		t.Fatalf("lists %+v", cards)
	}
	return cards[0].Count
}

func TestCollectionsEndpoints(t *testing.T) {
	s, _ := newServer(t)
	putKubrick(t, s, 2)
	fs := folders(t, s)
	if len(fs) != 1 || fs[0].Path != kubrick || fs[0].Name != "Kubrick" || fs[0].New != 2 || len(fs[0].Preview) != 2 || fs[0].List != nil {
		t.Fatalf("folders %+v", fs)
	}
	for body, want := range map[string]int{
		`{"path":"` + kubrick + `"}`:                        http.StatusBadRequest, // neither a name nor a list
		`{"path":"` + kubrick + `","name":"K","listId":1}`:  http.StatusBadRequest, // both
		`{"path":"../cine/Collections/Nada","name":"Nada"}`: http.StatusNotFound,
		`{"path":"` + kubrick + `","listId":999}`:           http.StatusNotFound,
		`{"path":"` + kubrick + `","name":" "}`:             http.StatusBadRequest, // a blank name
	} {
		if code := sendRec(t, s, "POST", "/api/collections/import", body).Code; code != want {
			t.Errorf("import %s: %d, want %d", body, code, want)
		}
	}
	rec := sendRec(t, s, "POST", "/api/collections/import", `{"path":"`+kubrick+`","name":"Kubrick"}`)
	var res struct {
		ListID int64 `json:"listId"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &res) != nil || res.ListID == 0 {
		t.Fatalf("import %d: %s", rec.Code, rec.Body)
	}
	if n := listCount(t, s); n != 2 {
		t.Fatalf("count %d", n)
	}
	if fs := folders(t, s); len(fs) != 0 {
		t.Fatalf("still pending %+v", fs)
	}

	// A new film in the folder is offered as an addition to the list.
	putKubrick(t, s, 3)
	fs = folders(t, s)
	if len(fs) != 1 || fs[0].New != 1 || fs[0].Total != 3 || fs[0].List == nil || fs[0].List.ID != res.ListID {
		t.Fatalf("folders %+v", fs)
	}
	if code := sendRec(t, s, "POST", "/api/collections/import", `{"path":"`+kubrick+`","name":"kubrick"}`).Code; code != http.StatusConflict {
		t.Fatalf("taken name %d", code)
	}
	body := fmt.Sprintf(`{"path":%q,"listId":%d}`, kubrick, res.ListID)
	if code := sendRec(t, s, "POST", "/api/collections/import", body).Code; code != 200 {
		t.Fatalf("append %d", code)
	}
	if n := listCount(t, s); n != 3 {
		t.Fatalf("count %d", n)
	}

	// Declining the next one keeps the list as it is.
	putKubrick(t, s, 4)
	if code := sendRec(t, s, "POST", "/api/collections/dismiss", `{"path":"`+kubrick+`"}`).Code; code != http.StatusNoContent {
		t.Fatalf("dismiss %d", code)
	}
	if fs := folders(t, s); len(fs) != 0 {
		t.Fatalf("still pending %+v", fs)
	}
	if n := listCount(t, s); n != 3 {
		t.Fatalf("count %d", n)
	}
	if code := sendRec(t, s, "POST", "/api/collections/dismiss", `{"path":"`+kubrick+`"}`).Code; code != http.StatusNotFound {
		t.Fatalf("dismiss twice %d", code)
	}
}

func TestCollectionsReadOnly(t *testing.T) {
	s, _ := newServer(t)
	putKubrick(t, s, 1)
	s.ReadOnly = true
	for _, url := range []string{"/api/collections/import", "/api/collections/dismiss"} {
		if code := sendRec(t, s, "POST", url, `{"path":"`+kubrick+`","name":"K"}`).Code; code != http.StatusConflict {
			t.Errorf("%s: %d", url, code)
		}
	}
	if fs := folders(t, s); len(fs) != 1 {
		t.Fatalf("folders %+v", fs)
	}
}
