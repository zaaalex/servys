package recommender

import (
	"context"
	"errors"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var ErrUnsafeDocumentURL = errors.New("unsafe document URL")

type HTTPDocumentFetcher struct {
	Client                  *http.Client
	MaxBodyBytes            int64
	AllowHTTP, AllowPrivate bool
}

func NewDocumentFetcher() *HTTPDocumentFetcher { return &HTTPDocumentFetcher{MaxBodyBytes: 4 << 20} }

// Go 1.27 считает частные адреса и CGNAT глобальными, поэтому их закрываем отдельно.
func blockedAddress(ip net.IP) bool {
	if ip == nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() || !ip.IsGlobalUnicast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, n := range extraBlocked {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

var extraBlocked = mustNets(
	"0.0.0.0/8",
	"100.64.0.0/10",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"240.0.0.0/4",
	"2001:db8::/32",
)

func mustNets(cidrs ...string) []*net.IPNet {
	nets := make([]*net.IPNet, len(cidrs))
	for i, cidr := range cidrs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		nets[i] = n
	}
	return nets
}

func (f *HTTPDocumentFetcher) admit(ctx context.Context, raw string) (*url.URL, net.IP, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil {
		return nil, nil, ErrUnsafeDocumentURL
	}
	if u.Scheme != "https" && !(f.AllowHTTP && u.Scheme == "http") {
		return nil, nil, ErrUnsafeDocumentURL
	}
	lookup, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(lookup, u.Hostname())
	if err != nil || len(ips) == 0 {
		return nil, nil, ErrUnsafeDocumentURL
	}
	if !f.AllowPrivate {
		for _, ip := range ips {
			if blockedAddress(ip.IP) {
				return nil, nil, ErrUnsafeDocumentURL
			}
		}
	}
	return u, ips[0].IP, nil
}

func (f *HTTPDocumentFetcher) pinnedClient(ip net.IP) *http.Client {
	timeout := 15 * time.Second
	if f.Client != nil && f.Client.Timeout > 0 {
		timeout = f.Client.Timeout
	}
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				_, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				return dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
			},
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func (f *HTTPDocumentFetcher) Fetch(ctx context.Context, s SearchResult) (FetchedDocument, error) {
	current := s.URL
	for hop := 0; hop < 5; hop++ {
		u, ip, err := f.admit(ctx, current)
		if err != nil {
			return FetchedDocument{}, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return FetchedDocument{}, ErrUnsafeDocumentURL
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		client := f.pinnedClient(ip)
		resp, err := client.Do(req)
		client.CloseIdleConnections()
		if err != nil {
			return FetchedDocument{}, err
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc, locErr := resp.Location()
			resp.Body.Close()
			if locErr != nil {
				return FetchedDocument{}, ErrUnsafeDocumentURL
			}
			current = loc.String()
			continue
		}
		defer resp.Body.Close()
		limit := f.MaxBodyBytes
		if limit <= 0 {
			limit = 4 << 20
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		if err != nil || int64(len(b)) > limit {
			return FetchedDocument{}, errors.New("document read failed")
		}
		return FetchedDocument{URL: u.String(), Title: s.Title, Text: NormalizeHTML(decodePage(b, resp.Header.Get("Content-Type"))), ContentType: resp.Header.Get("Content-Type")}, nil
	}
	return FetchedDocument{}, ErrUnsafeDocumentURL
}

func decodePage(b []byte, contentType string) string {
	switch pageCharset(contentType, b) {
	case "windows-1251", "cp1251", "x-cp1251":
		return decode1251(b)
	default:
		return string(b)
	}
}

func pageCharset(contentType string, body []byte) string {
	if _, params, err := mime.ParseMediaType(contentType); err == nil {
		if cs := strings.TrimSpace(params["charset"]); cs != "" {
			return strings.ToLower(cs)
		}
	}
	head := body
	if len(head) > 2048 {
		head = head[:2048]
	}
	low := strings.ToLower(string(head))
	i := strings.Index(low, "charset=")
	if i < 0 {
		return "utf-8"
	}
	rest := strings.Trim(low[i+len("charset="):], "\"' ")
	for j, r := range rest {
		if r == '"' || r == '\'' || r == ' ' || r == '>' || r == ';' {
			return rest[:j]
		}
	}
	return rest
}

func decode1251(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		if c < 0x80 {
			sb.WriteByte(c)
			continue
		}
		sb.WriteRune(cp1251[c-0x80])
	}
	return sb.String()
}

var cp1251 = []rune{
	0x0402, 0x0403, 0x201A, 0x0453, 0x201E, 0x2026, 0x2020, 0x2021, 0x20AC, 0x2030, 0x0409, 0x2039, 0x040A, 0x040C, 0x040B, 0x040F,
	0x0452, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0xFFFD, 0x2122, 0x0459, 0x203A, 0x045A, 0x045C, 0x045B, 0x045F,
	0x00A0, 0x040E, 0x045E, 0x0408, 0x00A4, 0x0490, 0x00A6, 0x00A7, 0x0401, 0x00A9, 0x0404, 0x00AB, 0x00AC, 0x00AD, 0x00AE, 0x0407,
	0x00B0, 0x00B1, 0x0406, 0x0456, 0x0491, 0x00B5, 0x00B6, 0x00B7, 0x0451, 0x2116, 0x0454, 0x00BB, 0x0458, 0x0405, 0x0455, 0x0457,
	0x0410, 0x0411, 0x0412, 0x0413, 0x0414, 0x0415, 0x0416, 0x0417, 0x0418, 0x0419, 0x041A, 0x041B, 0x041C, 0x041D, 0x041E, 0x041F,
	0x0420, 0x0421, 0x0422, 0x0423, 0x0424, 0x0425, 0x0426, 0x0427, 0x0428, 0x0429, 0x042A, 0x042B, 0x042C, 0x042D, 0x042E, 0x042F,
	0x0430, 0x0431, 0x0432, 0x0433, 0x0434, 0x0435, 0x0436, 0x0437, 0x0438, 0x0439, 0x043A, 0x043B, 0x043C, 0x043D, 0x043E, 0x043F,
	0x0440, 0x0441, 0x0442, 0x0443, 0x0444, 0x0445, 0x0446, 0x0447, 0x0448, 0x0449, 0x044A, 0x044B, 0x044C, 0x044D, 0x044E, 0x044F,
}

var dropHTML = regexp.MustCompile(`(?is)<(?:script|style|head)(?:[\s>/])[^>]*>.*?</(?:script|style|head)>`)
var tagHTML = regexp.MustCompile(`(?s)<[^>]*>`)

func NormalizeHTML(s string) string {
	return normalizeSpace(html.UnescapeString(tagHTML.ReplaceAllString(dropHTML.ReplaceAllString(s, " "), " ")))
}
