package store

import (
	"context"
	"errors"
	"os"
	"testing"
)

var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func snapshot(t *testing.T, s *Store) Snapshot {
	t.Helper()
	snap, err := s.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestFreshStoreHasOnlyDefaultGroup(t *testing.T) {
	snap := snapshot(t, openTest(t))
	if len(snap.Groups) != 1 || !snap.Groups[0].IsDefault || snap.Groups[0].ID != DefaultGroupID {
		t.Fatalf("groups = %+v", snap.Groups)
	}
	if len(snap.Sites) != 0 || len(snap.Domains) != 0 {
		t.Fatalf("unexpected data: %+v", snap)
	}
}

func TestReopenKeepsData(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSite(context.Background(), SiteInput{Title: "Go", URL: "go.dev"}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n := len(snapshot(t, s).Sites); n != 1 {
		t.Fatalf("sites = %d, want 1", n)
	}
}

func TestCreateSiteNormalizesURLAndRegistersDomain(t *testing.T) {
	s := openTest(t)
	site, err := s.CreateSite(context.Background(), SiteInput{Title: " GitHub ", URL: "GitHub.com/golang"})
	if err != nil {
		t.Fatal(err)
	}
	if site.URL != "https://GitHub.com/golang" || site.Domain != "github.com" || site.Title != "GitHub" {
		t.Fatalf("site = %+v", site)
	}
	if site.GroupID != DefaultGroupID {
		t.Fatalf("group = %d, want default", site.GroupID)
	}
	snap := snapshot(t, s)
	if len(snap.Domains) != 1 || snap.Domains[0].Domain != "github.com" || snap.Domains[0].SiteCount != 1 {
		t.Fatalf("domains = %+v", snap.Domains)
	}
}

func TestCreateSiteRejectsBadInput(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	for _, in := range []SiteInput{
		{Title: "", URL: "https://a.com"},
		{Title: "a", URL: ""},
		{Title: "a", URL: "ftp://a.com"},
		{Title: "a", URL: "https://"},
		{Title: "a", URL: "https://a.com", GroupID: 99},
	} {
		if _, err := s.CreateSite(ctx, in); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: err = %v, want ErrInvalid", in, err)
		}
	}
}

func TestDeletingSiteKeepsDomainAndIcon(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	site, _ := s.CreateSite(ctx, SiteInput{Title: "a", URL: "https://a.com"})
	icon, err := s.SetDomainIcon(ctx, "a.com", pngHeader)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSite(ctx, site.ID); err != nil {
		t.Fatal(err)
	}
	snap := snapshot(t, s)
	if len(snap.Domains) != 1 || snap.Domains[0].Icon == nil || *snap.Domains[0].Icon != icon {
		t.Fatalf("domains = %+v", snap.Domains)
	}
	if snap.Domains[0].SiteCount != 0 {
		t.Fatalf("site count = %d", snap.Domains[0].SiteCount)
	}
}

func TestSetDomainIconNamesFileByHashAndCleansUp(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	s.CreateSite(ctx, SiteInput{Title: "a", URL: "https://a.com"})

	first, err := s.SetDomainIcon(ctx, "a.com", pngHeader)
	if err != nil {
		t.Fatal(err)
	}
	if !iconName.MatchString(first) {
		t.Fatalf("name %q does not look hashed", first)
	}
	firstPath, _ := s.IconPath(first)

	second, err := s.SetDomainIcon(ctx, "a.com", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if second == first || second[len(second)-4:] != ".svg" {
		t.Fatalf("second = %q", second)
	}
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("old icon still on disk: %v", err)
	}

	if err := s.ClearDomainIcon(ctx, "a.com"); err != nil {
		t.Fatal(err)
	}
	secondPath, _ := s.IconPath(second)
	if _, err := os.Stat(secondPath); !os.IsNotExist(err) {
		t.Fatalf("cleared icon still on disk: %v", err)
	}
}

func TestSharedIconFileSurvivesWhileReferenced(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	s.CreateSite(ctx, SiteInput{Title: "a", URL: "https://a.com"})
	s.CreateSite(ctx, SiteInput{Title: "b", URL: "https://b.com"})
	name, _ := s.SetDomainIcon(ctx, "a.com", pngHeader)
	s.SetDomainIcon(ctx, "b.com", pngHeader)
	s.ClearDomainIcon(ctx, "a.com")
	path, _ := s.IconPath(name)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("shared icon removed: %v", err)
	}
}

func TestSetDomainIconRejectsUnknownFormatsAndDomains(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	s.CreateSite(ctx, SiteInput{Title: "a", URL: "https://a.com"})
	if _, err := s.SetDomainIcon(ctx, "a.com", []byte("hello")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if _, err := s.SetDomainIcon(ctx, "nope.com", pngHeader); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteDomainOnlyWhenUnused(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	site, _ := s.CreateSite(ctx, SiteInput{Title: "a", URL: "https://a.com"})
	if err := s.DeleteDomain(ctx, "a.com"); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	s.DeleteSite(ctx, site.ID)
	if err := s.DeleteDomain(ctx, "a.com"); err != nil {
		t.Fatal(err)
	}
	if n := len(snapshot(t, s).Domains); n != 0 {
		t.Fatalf("domains = %d", n)
	}
}

func TestDeleteGroupMovesSitesToDefault(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	s.CreateSite(ctx, SiteInput{Title: "d1", URL: "https://d.com"})
	g, err := s.CreateGroup(ctx, "工作")
	if err != nil {
		t.Fatal(err)
	}
	s.CreateSite(ctx, SiteInput{Title: "g1", URL: "https://g.com", GroupID: g.ID})
	s.CreateSite(ctx, SiteInput{Title: "g2", URL: "https://g.com/2", GroupID: g.ID})

	if err := s.DeleteGroup(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	snap := snapshot(t, s)
	if len(snap.Groups) != 1 {
		t.Fatalf("groups = %+v", snap.Groups)
	}
	var titles []string
	for _, st := range snap.Sites {
		if st.GroupID != DefaultGroupID {
			t.Fatalf("site %+v not moved", st)
		}
		titles = append(titles, st.Title)
	}
	if want := []string{"d1", "g1", "g2"}; len(titles) != 3 || titles[0] != want[0] || titles[1] != want[1] || titles[2] != want[2] {
		t.Fatalf("order = %v, want %v", titles, want)
	}
}

func TestDefaultGroupIsProtected(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if err := s.DeleteGroup(ctx, DefaultGroupID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("delete: %v", err)
	}
	if err := s.RenameGroup(ctx, DefaultGroupID, "x"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("rename: %v", err)
	}
}

func TestReorderGroups(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	a, _ := s.CreateGroup(ctx, "a")
	b, _ := s.CreateGroup(ctx, "b")
	if err := s.ReorderGroups(ctx, []int64{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	snap := snapshot(t, s)
	if snap.Groups[0].ID != DefaultGroupID || snap.Groups[1].ID != b.ID || snap.Groups[2].ID != a.ID {
		t.Fatalf("groups = %+v", snap.Groups)
	}
	for _, ids := range [][]int64{{a.ID}, {a.ID, a.ID}, {a.ID, DefaultGroupID}} {
		if err := s.ReorderGroups(ctx, ids); !errors.Is(err, ErrInvalid) {
			t.Errorf("%v: err = %v, want ErrInvalid", ids, err)
		}
	}
}

func TestUpdateSiteMovesToEndOfNewGroup(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	g, _ := s.CreateGroup(ctx, "g")
	s.CreateSite(ctx, SiteInput{Title: "g1", URL: "https://g.com", GroupID: g.ID})
	site, _ := s.CreateSite(ctx, SiteInput{Title: "d1", URL: "https://d.com"})
	if err := s.UpdateSite(ctx, site.ID, SiteInput{Title: "moved", URL: "https://new.com", GroupID: g.ID}); err != nil {
		t.Fatal(err)
	}
	snap := snapshot(t, s)
	last := snap.Sites[len(snap.Sites)-1]
	if last.ID != site.ID || last.GroupID != g.ID || last.Domain != "new.com" {
		t.Fatalf("sites = %+v", snap.Sites)
	}
	if len(snap.Domains) != 3 {
		t.Fatalf("domains = %+v", snap.Domains)
	}
	if err := s.UpdateSite(ctx, 999, SiteInput{Title: "x", URL: "x.com"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
