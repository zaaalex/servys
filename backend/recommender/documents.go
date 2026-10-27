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
func (f *HTTPDocumentFetcher) Fetch(ctx context.Context, s SearchResult) (FetchedDocument, error) {
	u, err := url.Parse(s.URL)
	if err != nil || u.Hostname() == "" || u.User != nil {
		return FetchedDocument{}, ErrUnsafeDocumentURL
	}
	if u.Scheme != "https" && !(f.AllowHTTP && u.Scheme == "http") {
		return FetchedDocument{}, ErrUnsafeDocumentURL
	}
	if !f.AllowPrivate {
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, u.Hostname())
		if e != nil {
			return FetchedDocument{}, ErrUnsafeDocumentURL
		}
		for _, ip := range ips {
			if ip.IP.IsPrivate() || ip.IP.IsLoopback() {
				return FetchedDocument{}, ErrUnsafeDocumentURL
			}
		}
	}
	c := f.Client
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := c.Do(req)
	if err != nil {
		return FetchedDocument{}, err
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
	return FetchedDocument{URL: s.URL, Title: s.Title, Text: NormalizeHTML(decodePage(b, resp.Header.Get("Content-Type"))), ContentType: resp.Header.Get("Content-Type")}, nil
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
