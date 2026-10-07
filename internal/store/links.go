package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

const maxLinks = 50

// normalizeLinks validates a site's links. Each one may be a full URL, a
// URL without a scheme, or a path on the site such as "eric/palace", and
// must end up on the site's domain. An empty title is derived from the path.
func normalizeLinks(links []SiteLink, site *url.URL, domain string) ([]SiteLink, error) {
	if len(links) > maxLinks {
		return nil, invalid(fmt.Sprintf("子链接最多 %d 个", maxLinks))
	}
	out := make([]SiteLink, 0, len(links))
	for _, l := range links {
		title := strings.TrimSpace(l.Title)
		raw := strings.TrimSpace(l.URL)
		if raw == "" {
			return nil, invalid("子链接的链接不能为空")
		}
		if len(raw) > maxURLLen {
			return nil, invalid("子链接的链接过长")
		}
		if utf8.RuneCountInString(title) > maxTitleLen {
			return nil, invalid("子链接的标题过长")
		}
		u, err := url.Parse(resolveLink(raw, site, domain))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
			return nil, invalid("子链接格式不正确：" + raw)
		}
		if strings.ToLower(u.Hostname()) != domain {
			return nil, invalid(fmt.Sprintf("子链接必须与网站同域名（%s）：%s", domain, raw))
		}
		if title == "" {
			title = linkTitle(u, domain)
		}
		out = append(out, SiteLink{Title: title, URL: u.String()})
	}
	return out, nil
}

// resolveLink turns user input into an absolute URL string. Input that
// starts with the site's domain only lacks a scheme; anything else without
// a scheme is a path on the site.
func resolveLink(raw string, site *url.URL, domain string) string {
	if strings.Contains(raw, "://") {
		return raw
	}
	end := strings.IndexAny(raw, "/?#")
	if end < 0 {
		end = len(raw)
	}
	if host := strings.ToLower(raw[:end]); host == domain || strings.HasPrefix(host, domain+":") {
		return site.Scheme + "://" + raw
	}
	return site.Scheme + "://" + site.Host + "/" + strings.TrimLeft(raw, "/")
}

// linkTitle names a link by the last two segments of its path, which is
// "owner/repo" on code hosts. A link to the site root is named by domain.
// The frontend mirrors this in defaultLinkTitle.
func linkTitle(u *url.URL, domain string) string {
	segs := strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' })
	if len(segs) == 0 {
		return domain
	}
	if len(segs) > 2 {
		segs = segs[len(segs)-2:]
	}
	title := strings.Join(segs, "/")
	if utf8.RuneCountInString(title) > maxTitleLen {
		title = string([]rune(title)[:maxTitleLen])
	}
	return title
}

func replaceLinks(ctx context.Context, tx *sql.Tx, siteID int64, links []SiteLink) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM site_links WHERE site_id = ?`, siteID); err != nil {
		return err
	}
	for i, l := range links {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO site_links (site_id, title, url, position) VALUES (?, ?, ?, ?)`,
			siteID, l.Title, l.URL, i+1); err != nil {
			return err
		}
	}
	return nil
}
