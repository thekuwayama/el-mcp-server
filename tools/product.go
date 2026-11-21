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

const (
	// productsPerPage is the number of products echonet.jp shows per result
	// page (its default grid view).
	productsPerPage = 12
	// maxProductLimit is the upper bound of the `limit` parameter.
	maxProductLimit = 100
	// maxScanPages is the number of result pages needed to collect
	// maxProductLimit products, so that every allowed `limit` is reachable.
	maxScanPages = (maxProductLimit + productsPerPage - 1) / productsPerPage
)

// productSearchFormFields lists every field of echonet.jp's product search
// form. The site only applies con_manufacturer / con_product_type when the
// request carries the whole form (as a browser submit does); sending just the
// non-empty fields makes it silently ignore the maker and category filters.
var productSearchFormFields = []string{
	"con_manufacturer",
	"con_product_type",
	"con_num",
	"con_year_from",
	"con_month_from",
	"con_day_from",
	"con_year_to",
	"con_month_to",
	"con_day_to",
	"pro_num",
}

func registerProductTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_certified_products",
		Description: "ECHONET Lite認証登録製品をechonet.jpで検索します。メーカー名・機種(カテゴリ)・キーワードで絞り込み可能。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, searchCertifiedProducts)
}

type searchCertifiedProductsParams struct {
	Maker    string `json:"maker,omitempty"    jsonschema:"メーカー名（echonet.jp上の日本語表記に部分一致）。例: パナソニック, ダイキン"`
	Category string `json:"category,omitempty" jsonschema:"echonet.jpの機種カテゴリ名（完全一致を優先し、なければ部分一致）。例: 家庭用エアコン, 低圧スマート電力量メータ, 蓄電池。複数の分類にある名前は「住宅設備機器/その他」のように分類名を付ける"`
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
	if limit > maxProductLimit {
		limit = maxProductLimit
	}

	categoryValue := ""
	if params.Category != "" {
		v, err := resolveCategory(params.Category)
		if err != nil {
			return errorResult(fmt.Sprintf("製品検索エラー: %v", err)), nil, nil
		}
		categoryValue = v
	}

	products, err := collectProducts(params.Maker, categoryValue, params.Keyword, limit)
	if err != nil {
		return errorResult(fmt.Sprintf("製品検索エラー: %v", err)), nil, nil
	}

	if len(products) == 0 {
		return textResult("検索条件に一致する認証製品が見つかりませんでした。"), nil, nil
	}
	return jsonResult(products)
}

// collectProducts fetches search result pages until `limit` products are
// collected, the last page is reached, or maxScanPages is exhausted. maker,
// categoryValue (a con_product_type option value) and keyword are all applied
// server-side by echonet.jp, so every parsed product is a match.
func collectProducts(maker, categoryValue, keyword string, limit int) ([]certifiedProduct, error) {
	var results []certifiedProduct
	for page := 1; page <= maxScanPages && len(results) < limit; page++ {
		body, err := fetchProductPage(page, maker, categoryValue, keyword)
		if err != nil {
			return nil, err
		}

		raw, err := parseProductHTML(body, limit-len(results))
		if err != nil {
			return nil, err
		}
		results = append(results, raw...)
		if len(raw) < productsPerPage {
			break
		}
	}
	return results, nil
}

// categoryOption is one <option> of the search form's con_product_type
// <select>. Value is what echonet.jp expects as con_product_type (e.g.
// "air_prdct_sprtr_%e5%ae%b6..."), Label is the displayed category name and
// Group is the enclosing <optgroup> label (e.g. "空調機器").
type categoryOption struct {
	Group string
	Label string
	Value string
}

func (o categoryOption) qualifiedName() string {
	if o.Group == "" {
		return o.Label
	}
	return o.Group + "/" + o.Label
}

// resolveCategory maps a human-readable category name to echonet.jp's
// con_product_type option value. echonet.jp ignores con_product_type unless
// it is one of the form's option values, so the options are read from the
// search page itself rather than hard-coded.
func resolveCategory(category string) (string, error) {
	body, err := fetchProductPage(1, "", "", "")
	if err != nil {
		return "", err
	}
	opts, err := parseCategoryOptions(body)
	if err != nil {
		return "", err
	}
	if len(opts) == 0 {
		return "", fmt.Errorf("echonet.jpの機種カテゴリ一覧を取得できませんでした")
	}
	opt, err := matchCategory(opts, category)
	if err != nil {
		return "", err
	}
	return opt.Value, nil
}

// parseCategoryOptions extracts the non-empty options of the
// <select name="con_product_type"> in the search form.
func parseCategoryOptions(body string) ([]categoryOption, error) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	var opts []categoryOption
	var walk func(n *html.Node, inSelect bool, group string)
	walk = func(n *html.Node, inSelect bool, group string) {
		if isElement(n, "select") {
			inSelect = attr(n, "name") == "con_product_type"
		}
		if inSelect && isElement(n, "optgroup") {
			group = strings.TrimSpace(attr(n, "label"))
		}
		if inSelect && isElement(n, "option") {
			if v := attr(n, "value"); v != "" {
				opts = append(opts, categoryOption{
					Group: group,
					Label: strings.TrimSpace(textContent(n)),
					Value: v,
				})
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inSelect, group)
		}
	}
	walk(doc, false, "")

	return opts, nil
}

// matchCategory picks the option for query. It accepts, in order of
// preference: an option value as-is, an exact category name ("家庭用エアコン")
// or "分類/名前" ("住宅設備機器/その他"), and finally a unique partial match
// of the name. Ambiguous or unknown queries return an error listing the
// candidates so the caller can retry with a precise name.
func matchCategory(opts []categoryOption, query string) (categoryOption, error) {
	q := strings.TrimSpace(query)

	for _, o := range opts {
		if o.Value == q {
			return o, nil
		}
	}

	var exact []categoryOption
	for _, o := range opts {
		if strings.EqualFold(o.Label, q) || strings.EqualFold(o.qualifiedName(), q) {
			exact = append(exact, o)
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return categoryOption{}, fmt.Errorf("機種カテゴリ %q は複数の分類にあります。分類名を付けて指定してください: %s", query, joinCategoryNames(exact))
	}

	var partial []categoryOption
	for _, o := range opts {
		if containsFold(o.qualifiedName(), q) {
			partial = append(partial, o)
		}
	}
	switch len(partial) {
	case 1:
		return partial[0], nil
	case 0:
		return categoryOption{}, fmt.Errorf("機種カテゴリ %q はechonet.jpにありません。指定できるカテゴリ: %s", query, joinCategoryNames(opts))
	default:
		return categoryOption{}, fmt.Errorf("機種カテゴリ %q に一致するカテゴリが複数あります。いずれかを指定してください: %s", query, joinCategoryNames(partial))
	}
}

func joinCategoryNames(opts []categoryOption) string {
	names := make([]string, len(opts))
	for i, o := range opts {
		names[i] = o.qualifiedName()
	}
	return strings.Join(names, ", ")
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// buildProductPageURL builds a GET request URL against echonet.jp's product
// search form (method="get", action="https://echonet.jp/product/echonet-lite"),
// matching its actual field names: con_manufacturer (maker), con_product_type
// (category option value), pro_num (keyword, despite the name). When any
// filter is set, every form field is sent (empty ones included), because
// echonet.jp ignores the maker/category filters otherwise. Pagination uses the
// site's own /page/N/ path segment; there is no per-page-count parameter.
func buildProductPageURL(page int, maker, categoryValue, keyword string) string {
	reqURL := productSearchURL
	if page > 1 {
		reqURL = fmt.Sprintf("%spage/%d/", productSearchURL, page)
	}
	if maker == "" && categoryValue == "" && keyword == "" {
		return reqURL
	}

	values := map[string]string{
		"con_manufacturer": maker,
		"con_product_type": categoryValue,
		"pro_num":          keyword,
	}
	parts := make([]string, len(productSearchFormFields))
	for i, f := range productSearchFormFields {
		parts[i] = f + "=" + url.QueryEscape(values[f])
	}
	return reqURL + "?" + strings.Join(parts, "&")
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

func isElement(n *html.Node, tag string) bool {
	return n.Type == html.ElementNode && n.Data == tag
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
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
