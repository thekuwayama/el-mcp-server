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
	Name       string `json:"name"`
	Maker      string `json:"maker"`
	CertNumber string `json:"cert_number"`
	AppendixVer string `json:"appendix_version,omitempty"`
	CertDate   string `json:"cert_date,omitempty"`
}

func searchCertifiedProducts(_ context.Context, _ *mcp.CallToolRequest, params *searchCertifiedProductsParams) (*mcp.CallToolResult, any, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	body, err := fetchProductPage(params.Maker, params.Category, params.Keyword, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("製品ページ取得エラー: %w", err)
	}

	products, err := parseProductHTML(body, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("HTMLパースエラー: %w", err)
	}

	if len(products) == 0 {
		return textResult("検索条件に一致する認証製品が見つかりませんでした。"), nil, nil
	}
	return jsonResult(products)
}

func fetchProductPage(maker, category, keyword string, limit int) (string, error) {
	perPage := limit
	if perPage > 48 {
		perPage = 48
	}

	form := url.Values{}
	if maker != "" {
		form.Set("maker", maker)
	}
	if category != "" {
		form.Set("category", category)
	}
	if keyword != "" {
		form.Set("keyword", keyword)
	}
	form.Set("per_page", fmt.Sprintf("%d", perPage))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.PostForm(productSearchURL, form)
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
// The page uses a card-based layout; we collect text blocks and heuristically extract fields.
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
		if isElement(n, "article") || hasClass(n, "product-item") || hasClass(n, "product_item") || hasClass(n, "item") {
			p := extractProduct(n)
			if p.CertNumber != "" || p.Name != "" {
				products = append(products, p)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// fallback: if nothing found via structured walk, extract all text with cert number patterns
	if len(products) == 0 {
		products = extractByPattern(doc, limit)
	}

	return products, nil
}

func extractProduct(n *html.Node) certifiedProduct {
	var p certifiedProduct
	texts := collectTexts(n)
	for _, t := range texts {
		t = strings.TrimSpace(t)
		if strings.HasPrefix(t, "ENL認証登録番号") || strings.HasPrefix(t, "認証登録番号") {
			parts := strings.SplitN(t, ":", 2)
			if len(parts) == 2 {
				p.CertNumber = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(t, "Appendixバージョン") {
			parts := strings.SplitN(t, ":", 2)
			if len(parts) == 2 {
				p.AppendixVer = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(t, "ENL認証登録日") || strings.HasPrefix(t, "認証登録日") {
			parts := strings.SplitN(t, ":", 2)
			if len(parts) == 2 {
				p.CertDate = strings.TrimSpace(parts[1])
			}
		} else if p.Name == "" && len(t) > 3 && !strings.Contains(t, ":") {
			p.Name = t
		}
	}
	return p
}

func extractByPattern(n *html.Node, limit int) []certifiedProduct {
	var current *certifiedProduct
	var products []certifiedProduct

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if len(products) >= limit {
			return
		}
		if node.Type == html.TextNode {
			t := strings.TrimSpace(node.Data)
			if t == "" {
				return
			}
			if strings.Contains(t, "ENL認証登録番号") || strings.Contains(t, "認証登録番号 :") {
				if current != nil && current.CertNumber != "" {
					products = append(products, *current)
				}
				current = &certifiedProduct{}
				parts := strings.SplitN(t, ":", 2)
				if len(parts) == 2 {
					current.CertNumber = strings.TrimSpace(parts[1])
				}
			} else if current != nil {
				if strings.Contains(t, "Appendixバージョン") {
					parts := strings.SplitN(t, ":", 2)
					if len(parts) == 2 {
						current.AppendixVer = strings.TrimSpace(parts[1])
					}
				} else if strings.Contains(t, "認証登録日") {
					parts := strings.SplitN(t, ":", 2)
					if len(parts) == 2 {
						current.CertDate = strings.TrimSpace(parts[1])
					}
				} else if current.Name == "" && len(t) > 3 {
					current.Name = t
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)

	if current != nil && current.CertNumber != "" {
		products = append(products, *current)
	}
	return products
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
