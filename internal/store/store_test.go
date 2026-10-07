package store

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
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

func newUser(t *testing.T, s *Store, subject string) int64 {
	t.Helper()
	u, err := s.UpsertUser(context.Background(), Identity{Issuer: "https://idp.test", Subject: subject})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func snapshot(t *testing.T, s *Store, userID int64) Snapshot {
	t.Helper()
	snap, err := s.Snapshot(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func defaultGroup(t *testing.T, s *Store, userID int64) int64 {
	t.Helper()
	return snapshot(t, s, userID).Groups[0].ID
}

func TestNewUserHasOnlyDefaultGroup(t *testing.T) {
	s := openTest(t)
	snap := snapshot(t, s, newUser(t, s, "alice"))
	if len(snap.Groups) != 1 || !snap.Groups[0].IsDefault {
		t.Fatalf("groups = %+v", snap.Groups)
	}
	if len(snap.Sites) != 0 || len(snap.Domains) != 0 {
		t.Fatalf("unexpected data: %+v", snap)
	}
}

func TestUpsertUserKeepsIdentityAndRefreshesProfile(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	first, err := s.UpsertUser(ctx, Identity{Issuer: "i", Subject: "s", Name: "Old", Email: "old@x"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.UpsertUser(ctx, Identity{Issuer: "i", Subject: "s", Name: "New", Email: "new@x"})
	if err != nil {
		t.Fatal(err)
	}
	if second != (User{ID: first.ID, Name: "New", Email: "new@x"}) {
		t.Fatalf("second = %+v, first = %+v", second, first)
	}
	if n := len(snapshot(t, s, first.ID).Groups); n != 1 {
		t.Fatalf("relogin created extra groups: %d", n)
	}
	other, _ := s.UpsertUser(ctx, Identity{Issuer: "other", Subject: "s"})
	if other.ID == first.ID {
		t.Fatal("same subject from another issuer must be another user")
	}
}

func TestSessionsLastUntilDeleted(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	uid := newUser(t, s, "alice")
	token, err := s.CreateSession(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	u, _, err := s.SessionUser(ctx, token)
	if err != nil || u.ID != uid {
		t.Fatalf("user = %+v, err = %v", u, err)
	}
	if err := s.TouchSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SessionUser(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if _, _, err := s.SessionUser(ctx, "made-up"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUsersCannotSeeOrTouchEachOthersData(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	alice, bob := newUser(t, s, "alice"), newUser(t, s, "bob")
	site, _ := s.CreateSite(ctx, alice, SiteInput{Title: "a", URL: "a.com"})
	group, _ := s.CreateGroup(ctx, alice, "g")
	s.SetDomainIcon(ctx, alice, "a.com", pngHeader)

	if snap := snapshot(t, s, bob); len(snap.Sites) != 0 || len(snap.Domains) != 0 || len(snap.Groups) != 1 {
		t.Fatalf("bob sees %+v", snap)
	}
	for name, err := range map[string]error{
		"update site":   s.UpdateSite(ctx, bob, site.ID, SiteInput{Title: "x", URL: "x.com"}),
		"delete site":   s.DeleteSite(ctx, bob, site.ID),
		"move site":     s.MoveSite(ctx, bob, site.ID, 0),
		"rename group":  s.RenameGroup(ctx, bob, group.ID, "x"),
		"hide group":    s.SetGroupHiddenAtWork(ctx, bob, group.ID, true),
		"delete group":  s.DeleteGroup(ctx, bob, group.ID),
		"clear icon":    s.ClearDomainIcon(ctx, bob, "a.com"),
		"delete domain": s.DeleteDomain(ctx, bob, "a.com"),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
	if _, err := s.CreateSite(ctx, bob, SiteInput{Title: "x", URL: "x.com", GroupID: group.ID}); !errors.Is(err, ErrInvalid) {
		t.Errorf("site into alice's group: err = %v, want ErrInvalid", err)
	}
	if err := s.ReorderGroups(ctx, bob, []int64{group.ID}); !errors.Is(err, ErrInvalid) {
		t.Errorf("reorder alice's group: err = %v, want ErrInvalid", err)
	}
	if snap := snapshot(t, s, alice); len(snap.Sites) != 1 || snap.Domains[0].Icon == nil || len(snap.Groups) != 2 {
		t.Fatalf("alice's data changed: %+v", snap)
	}
}

func TestReopenKeepsData(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	uid := newUser(t, s, "alice")
	if _, err := s.CreateSite(context.Background(), uid, SiteInput{Title: "Go", URL: "go.dev"}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n := len(snapshot(t, s, uid).Sites); n != 1 {
		t.Fatalf("sites = %d, want 1", n)
	}
}

func TestCreateSiteNormalizesURLAndRegistersDomain(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	site, err := s.CreateSite(context.Background(), uid, SiteInput{Title: " GitHub ", URL: "GitHub.com/golang"})
	if err != nil {
		t.Fatal(err)
	}
	want := Site{ID: site.ID, Title: "GitHub", URL: "https://GitHub.com/golang", Domain: "github.com", GroupID: defaultGroup(t, s, uid), SortKey: "a0", Links: []SiteLink{}}
	if !reflect.DeepEqual(site, want) {
		t.Fatalf("site = %+v, want %+v", site, want)
	}
	snap := snapshot(t, s, uid)
	if len(snap.Domains) != 1 || snap.Domains[0] != (Domain{Domain: "github.com", SiteCount: 1}) {
		t.Fatalf("domains = %+v", snap.Domains)
	}
}

func TestCreateSiteRejectsBadInput(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	for _, in := range []SiteInput{
		{Title: "", URL: "https://a.com"},
		{Title: "a", URL: ""},
		{Title: "a", URL: "ftp://a.com"},
		{Title: "a", URL: "https://"},
		{Title: "a", URL: "https://a.com", GroupID: 99},
	} {
		if _, err := s.CreateSite(ctx, uid, in); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: err = %v, want ErrInvalid", in, err)
		}
	}
}

func TestDeletingSiteKeepsDomainAndIcon(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	site, _ := s.CreateSite(ctx, uid, SiteInput{Title: "a", URL: "https://a.com"})
	icon, err := s.SetDomainIcon(ctx, uid, "a.com", pngHeader)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSite(ctx, uid, site.ID); err != nil {
		t.Fatal(err)
	}
	snap := snapshot(t, s, uid)
	if len(snap.Domains) != 1 || snap.Domains[0] != (Domain{Domain: "a.com", Icon: snap.Domains[0].Icon, SiteCount: 0}) || *snap.Domains[0].Icon != icon {
		t.Fatalf("domains = %+v", snap.Domains)
	}
}

func TestSetDomainIconNamesFileByHashAndCleansUp(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	s.CreateSite(ctx, uid, SiteInput{Title: "a", URL: "https://a.com"})

	first, err := s.SetDomainIcon(ctx, uid, "a.com", pngHeader)
	if err != nil {
		t.Fatal(err)
	}
	if !iconName.MatchString(first) {
		t.Fatalf("name %q does not look hashed", first)
	}
	firstPath, _ := s.IconPath(first)

	second, err := s.SetDomainIcon(ctx, uid, "a.com", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if second == first || second[len(second)-4:] != ".svg" {
		t.Fatalf("second = %q", second)
	}
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("old icon still on disk: %v", err)
	}

	if err := s.ClearDomainIcon(ctx, uid, "a.com"); err != nil {
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
	alice, bob := newUser(t, s, "alice"), newUser(t, s, "bob")
	s.CreateSite(ctx, alice, SiteInput{Title: "a", URL: "https://a.com"})
	s.CreateSite(ctx, bob, SiteInput{Title: "a", URL: "https://a.com"})
	name, _ := s.SetDomainIcon(ctx, alice, "a.com", pngHeader)
	s.SetDomainIcon(ctx, bob, "a.com", pngHeader)
	s.ClearDomainIcon(ctx, alice, "a.com")
	path, _ := s.IconPath(name)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("icon still used by another user was removed: %v", err)
	}
}

func TestSetDomainIconRejectsUnknownFormatsAndDomains(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	s.CreateSite(ctx, uid, SiteInput{Title: "a", URL: "https://a.com"})
	if _, err := s.SetDomainIcon(ctx, uid, "a.com", []byte("hello")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if _, err := s.SetDomainIcon(ctx, uid, "nope.com", pngHeader); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteDomainOnlyWhenUnused(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	site, _ := s.CreateSite(ctx, uid, SiteInput{Title: "a", URL: "https://a.com"})
	if err := s.DeleteDomain(ctx, uid, "a.com"); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	s.DeleteSite(ctx, uid, site.ID)
	if err := s.DeleteDomain(ctx, uid, "a.com"); err != nil {
		t.Fatal(err)
	}
	if n := len(snapshot(t, s, uid).Domains); n != 0 {
		t.Fatalf("domains = %d", n)
	}
}

func TestDeleteGroupMovesSitesToDefault(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	def := defaultGroup(t, s, uid)
	s.CreateSite(ctx, uid, SiteInput{Title: "d1", URL: "https://d.com"})
	g, err := s.CreateGroup(ctx, uid, "工作")
	if err != nil {
		t.Fatal(err)
	}
	s.CreateSite(ctx, uid, SiteInput{Title: "g1", URL: "https://g.com", GroupID: g.ID})
	s.CreateSite(ctx, uid, SiteInput{Title: "g2", URL: "https://g.com/2", GroupID: g.ID})

	if err := s.DeleteGroup(ctx, uid, g.ID); err != nil {
		t.Fatal(err)
	}
	snap := snapshot(t, s, uid)
	if len(snap.Groups) != 1 {
		t.Fatalf("groups = %+v", snap.Groups)
	}
	var titles []string
	for _, st := range snap.Sites {
		if st.GroupID != def {
			t.Fatalf("site %+v not moved", st)
		}
		titles = append(titles, st.Title)
	}
	if want := []string{"d1", "g1", "g2"}; !slices.Equal(titles, want) {
		t.Fatalf("order = %v, want %v", titles, want)
	}
}

func siteTitles(t *testing.T, s *Store, userID int64) []string {
	t.Helper()
	var titles []string
	for _, st := range snapshot(t, s, userID).Sites {
		titles = append(titles, st.Title)
	}
	return titles
}

func TestMoveSite(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	var ids []int64
	for _, title := range []string{"a", "b", "c"} {
		site, err := s.CreateSite(ctx, uid, SiteInput{Title: title, URL: title + ".com"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, site.ID)
	}
	a, b, c := ids[0], ids[1], ids[2]
	other, _ := s.CreateGroup(ctx, uid, "g")
	elsewhere, _ := s.CreateSite(ctx, uid, SiteInput{Title: "x", URL: "x.com", GroupID: other.ID})

	for _, step := range []struct {
		id, after int64
		want      []string
	}{
		{c, 0, []string{"c", "a", "b", "x"}},
		{a, b, []string{"c", "b", "a", "x"}},
		{b, c, []string{"c", "b", "a", "x"}}, // already there
		{c, a, []string{"b", "a", "c", "x"}},
		{a, b, []string{"b", "a", "c", "x"}},
		{c, b, []string{"b", "c", "a", "x"}},
	} {
		if err := s.MoveSite(ctx, uid, step.id, step.after); err != nil {
			t.Fatal(err)
		}
		if got := siteTitles(t, s, uid); !slices.Equal(got, step.want) {
			t.Fatalf("after moving %d behind %d: order = %v, want %v", step.id, step.after, got, step.want)
		}
	}

	for name, after := range map[string]int64{"itself": a, "other group": elsewhere.ID, "missing": 999} {
		if err := s.MoveSite(ctx, uid, a, after); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	if err := s.MoveSite(ctx, uid, 999, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing site: err = %v, want ErrNotFound", err)
	}
}

func TestDefaultGroupIsProtected(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	def := defaultGroup(t, s, uid)
	if err := s.DeleteGroup(ctx, uid, def); !errors.Is(err, ErrInvalid) {
		t.Fatalf("delete: %v", err)
	}
	if err := s.RenameGroup(ctx, uid, def, "x"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("rename: %v", err)
	}
}

func TestGroupHiddenAtWork(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	def := defaultGroup(t, s, uid)
	g, _ := s.CreateGroup(ctx, uid, "g")
	if snapshot(t, s, uid).Groups[1].HiddenAtWork {
		t.Fatal("new group is hidden at work")
	}
	if err := s.SetGroupHiddenAtWork(ctx, uid, g.ID, true); err != nil {
		t.Fatal(err)
	}
	if !snapshot(t, s, uid).Groups[1].HiddenAtWork {
		t.Fatal("group not hidden at work")
	}
	if err := s.SetGroupHiddenAtWork(ctx, uid, def, true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("hiding default group: err = %v, want ErrInvalid", err)
	}
}

func TestReorderGroups(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	def := defaultGroup(t, s, uid)
	a, _ := s.CreateGroup(ctx, uid, "a")
	b, _ := s.CreateGroup(ctx, uid, "b")
	if err := s.ReorderGroups(ctx, uid, []int64{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	snap := snapshot(t, s, uid)
	if snap.Groups[0].ID != def || snap.Groups[1].ID != b.ID || snap.Groups[2].ID != a.ID {
		t.Fatalf("groups = %+v", snap.Groups)
	}
	for _, ids := range [][]int64{{a.ID}, {a.ID, a.ID}, {a.ID, def}} {
		if err := s.ReorderGroups(ctx, uid, ids); !errors.Is(err, ErrInvalid) {
			t.Errorf("%v: err = %v, want ErrInvalid", ids, err)
		}
	}
}

func TestUpdateSiteMovesToEndOfNewGroup(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	g, _ := s.CreateGroup(ctx, uid, "g")
	s.CreateSite(ctx, uid, SiteInput{Title: "g1", URL: "https://g.com", GroupID: g.ID})
	site, _ := s.CreateSite(ctx, uid, SiteInput{Title: "d1", URL: "https://d.com"})
	if err := s.UpdateSite(ctx, uid, site.ID, SiteInput{Title: "moved", URL: "https://new.com", GroupID: g.ID}); err != nil {
		t.Fatal(err)
	}
	snap := snapshot(t, s, uid)
	last := snap.Sites[len(snap.Sites)-1]
	if !reflect.DeepEqual(last, Site{ID: site.ID, Title: "moved", URL: "https://new.com", Domain: "new.com", GroupID: g.ID, SortKey: "a1", Links: []SiteLink{}}) {
		t.Fatalf("sites = %+v", snap.Sites)
	}
	if len(snap.Domains) != 3 {
		t.Fatalf("domains = %+v", snap.Domains)
	}
	if err := s.UpdateSite(ctx, uid, 999, SiteInput{Title: "x", URL: "x.com"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSiteLinksResolveAgainstTheSite(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	site, err := s.CreateSite(context.Background(), uid, SiteInput{Title: "GitHub", URL: "github.com", Links: []SiteLink{
		{URL: "eric/palace"},
		{URL: "/eric/homepage/pulls", Title: " PRs "},
		{URL: "GitHub.com/golang/go"},
		{URL: "https://github.com/a/b/c?tab=readme#top"},
		{URL: "github.com"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := []SiteLink{
		{Title: "eric/palace", URL: "https://github.com/eric/palace"},
		{Title: "PRs", URL: "https://github.com/eric/homepage/pulls"},
		{Title: "golang/go", URL: "https://GitHub.com/golang/go"},
		{Title: "b/c", URL: "https://github.com/a/b/c?tab=readme#top"},
		{Title: "github.com", URL: "https://github.com"},
	}
	if !reflect.DeepEqual(site.Links, want) {
		t.Fatalf("links = %+v\nwant    %+v", site.Links, want)
	}
	if got := snapshot(t, s, uid).Sites[0].Links; !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot links = %+v", got)
	}
}

func TestSiteLinksMustStayOnTheDomain(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	for _, link := range []SiteLink{
		{URL: "https://gitlab.com/eric/palace"},
		{URL: "https://api.github.com/repos"},
		{URL: "ftp://github.com/x"},
		{URL: " "},
	} {
		in := SiteInput{Title: "GitHub", URL: "github.com", Links: []SiteLink{{URL: "ok"}, link}}
		if _, err := s.CreateSite(ctx, uid, in); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: err = %v, want ErrInvalid", link, err)
		}
	}
	if n := len(snapshot(t, s, uid).Sites); n != 0 {
		t.Fatalf("rejected input created %d sites", n)
	}

	// Moving the site to another domain must not strand its links.
	site, _ := s.CreateSite(ctx, uid, SiteInput{Title: "GitHub", URL: "github.com", Links: []SiteLink{{URL: "https://github.com/x"}}})
	err := s.UpdateSite(ctx, uid, site.ID, SiteInput{Title: "GitLab", URL: "gitlab.com", Links: []SiteLink{{URL: "https://github.com/x"}}})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestUpdateSiteReplacesLinksAndDeleteRemovesThem(t *testing.T) {
	s := openTest(t)
	uid := newUser(t, s, "alice")
	ctx := context.Background()
	site, _ := s.CreateSite(ctx, uid, SiteInput{Title: "GitHub", URL: "github.com", Links: []SiteLink{{URL: "a/1"}, {URL: "b/2"}}})
	other, _ := s.CreateSite(ctx, uid, SiteInput{Title: "Go", URL: "go.dev", Links: []SiteLink{{URL: "doc"}}})

	if err := s.UpdateSite(ctx, uid, site.ID, SiteInput{Title: "GitHub", URL: "github.com", Links: []SiteLink{{URL: "b/2"}, {URL: "c/3"}}}); err != nil {
		t.Fatal(err)
	}
	got := snapshot(t, s, uid).Sites
	if titles := linkTitles(got[0].Links); !slices.Equal(titles, []string{"b/2", "c/3"}) {
		t.Fatalf("links = %v", titles)
	}

	if err := s.DeleteSite(ctx, uid, site.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM site_links WHERE site_id = ?`, site.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d links left behind", n)
	}
	if got := snapshot(t, s, uid).Sites; len(got) != 1 || got[0].ID != other.ID || len(got[0].Links) != 1 {
		t.Fatalf("other site's links changed: %+v", got)
	}
}

func TestOtherUsersCannotReplaceLinks(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	alice, bob := newUser(t, s, "alice"), newUser(t, s, "bob")
	site, _ := s.CreateSite(ctx, alice, SiteInput{Title: "GitHub", URL: "github.com", Links: []SiteLink{{URL: "a/1"}}})
	err := s.UpdateSite(ctx, bob, site.ID, SiteInput{Title: "GitHub", URL: "github.com", Links: []SiteLink{{URL: "evil"}}})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if got := snapshot(t, s, alice).Sites[0].Links; len(got) != 1 || got[0].Title != "a/1" {
		t.Fatalf("alice's links = %+v", got)
	}
}

func linkTitles(links []SiteLink) []string {
	titles := make([]string, len(links))
	for i, l := range links {
		titles[i] = l.Title
	}
	return titles
}
