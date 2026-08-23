package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/net/html"
)

const productSearchURL = "https://echonet.jp/product/echonet-lite/"

// maxScanPages bounds how many result pages (12 products/page) are fetched
// while collecting up to `limit` matches for maker/category filters that
// echonet.jp's own query parameters (con_manufacturer / con_product_type) do
// not reliably apply server-side (see fetchProductPage).
const maxScanPages = 5

func registerProductTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_certified_products",
		Description: "ECHONET Lite認証登録製品をechonet.jpで検索します。メーカー名・機種(カテゴリ)・キーワードで絞り込み可能。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, searchCertifiedProducts)
}

type searchCertifiedProductsParams struct {
	Maker    string `json:"maker,omitempty"    jsonschema:"メーカー名（部分一致）。例: パナソニック, Panasonic"`
	Category string `json:"category,omitempty" jsonschema:"機種カテゴリ。例: 家庭用エアコン, 低圧スマート電力量メータ"`
	Keyword  string `json:"keyword,omitempty"  jsonschema:"フリーワード検索。例: エアコン, V2H"`
	Limit    int    `json:"limit,omitempty"    jsonschema:"最大取得件数(デフォルト20, 最大100)"`
}

type certifiedProduct struct {
	Name        string `json:"name"`
	Maker       string `json:"maker"`
	CertNumber  string `json:"cert_number"`
	AppendixVer string `json:"appendix_version,omitempty"`
	CertDate    string `json:"cert_date,omitempty"`
}

func searchCertifiedProducts(_ context.Context, _ *mcp.CallToolRequest, params *searchCertifiedProductsParams) (*mcp.CallToolResult, any, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	products, err := collectProducts(params.Maker, params.Category, params.Keyword, limit)
	if err != nil {
		return errorResult(fmt.Sprintf("製品検索エラー: %v", err)), nil, nil
	}

	if len(products) == 0 {
		return textResult("検索条件に一致する認証製品が見つかりませんでした。"), nil, nil
	}
	return jsonResult(products)
}

// collectProducts fetches search result pages until `limit` matching
// products are collected or maxScanPages is exhausted. echonet.jp's search
// form only reliably filters by keyword (pro_num) server-side; maker and
// category are additionally filtered client-side against the parsed results
// so the tool still narrows results even when the site ignores those query
// parameters.
func collectProducts(maker, category, keyword string, limit int) ([]certifiedProduct, error) {
	var results []certifiedProduct
	for page := 1; page <= maxScanPages && len(results) < limit; page++ {
		body, err := fetchProductPage(page, maker, category, keyword)
		if err != nil {
			return nil, err
		}

		raw, err := parseProductHTML(body, 100)
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			break
		}

		for _, p := range raw {
			if !matchesFilter(p, maker, category) {
				continue
			}
			results = append(results, p)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func matchesFilter(p certifiedProduct, maker, category string) bool {
	if maker != "" && !containsFold(p.Maker, maker) {
		return false
	}
	if category != "" && !containsFold(p.Name, category) {
		return false
	}
	return true
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// fetchProductPage issues a GET request against echonet.jp's product search
// form (method="get", action="https://echonet.jp/product/echonet-lite/"),
// matching its actual field names: con_manufacturer (maker), con_product_type
// (category), pro_num (keyword, despite the name). Pagination uses the
// site's own /page/N/ path segment; there is no per-page-count parameter.
func buildProductPageURL(page int, maker, category, keyword string) string {
	reqURL := productSearchURL
	if page > 1 {
		reqURL = fmt.Sprintf("%spage/%d/", productSearchURL, page)
	}

	q := url.Values{}
	if maker != "" {
		q.Set("con_manufacturer", maker)
	}
	if category != "" {
		q.Set("con_product_type", category)
	}
	if keyword != "" {
		q.Set("pro_num", keyword)
	}
	if len(q) > 0 {
		reqURL += "?" + q.Encode()
	}
	return reqURL
}

func fetchProductPage(page int, maker, category, keyword string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(buildProductPageURL(page, maker, category, keyword))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2MB limit
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseProductHTML extracts product entries from the search result HTML.
// Each product is in a <div class="col-sm-4"> containing:
//   - <h3 class="name"> with product name text and <small> for maker
//   - <p><small> for cert number, appendix version
//   - <p> for cert date
func parseProductHTML(body string, limit int) ([]certifiedProduct, error) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	var products []certifiedProduct
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(products) >= limit {
			return
		}
		if isElement(n, "div") && hasClass(n, "col-sm-4") {
			p := extractProduct(n)
			if p.CertNumber != "" {
				products = append(products, p)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return products, nil
}

func extractProduct(n *html.Node) certifiedProduct {
	var p certifiedProduct

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if isElement(node, "h3") && hasClass(node, "name") {
			// first direct text node = product name, <small> = maker
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					if t := strings.TrimSpace(c.Data); t != "" && p.Name == "" {
						p.Name = t
					}
				}
				if isElement(c, "small") {
					if t := strings.TrimSpace(textContent(c)); t != "" {
						p.Maker = t
					}
				}
			}
		}
		if isElement(node, "p") {
			t := strings.TrimSpace(textContent(node))
			switch {
			case strings.Contains(t, "ENL認証登録番号"):
				p.CertNumber = strings.TrimSpace(strings.SplitN(t, ":", 2)[1])
			case strings.Contains(t, "Appendixバージョン"):
				p.AppendixVer = strings.TrimSpace(strings.SplitN(t, ":", 2)[1])
			case strings.Contains(t, "ENL認証登録日") || strings.Contains(t, "認証登録日"):
				p.CertDate = strings.TrimSpace(strings.SplitN(t, ":", 2)[1])
			case p.Name == "" && !strings.Contains(t, ":") && len(t) > 3:
				p.Name = t
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)

	return p
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func collectTexts(n *html.Node) []string {
	var texts []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			t := strings.TrimSpace(node.Data)
			if t != "" {
				texts = append(texts, t)
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return texts
}

func isElement(n *html.Node, tag string) bool {
	return n.Type == html.ElementNode && n.Data == tag
}

func hasClass(n *html.Node, class string) bool {
	if n.Type != html.ElementNode {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}
