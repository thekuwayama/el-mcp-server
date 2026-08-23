package tools

import "testing"

func TestMatchesFilter(t *testing.T) {
	p := certifiedProduct{Name: "蓄電システム", Maker: "ニチコン株式会社"}

	cases := []struct {
		name     string
		maker    string
		category string
		want     bool
	}{
		{"no filter", "", "", true},
		{"maker match (partial)", "ニチコン", "", true},
		{"maker mismatch", "パナソニック", "", false},
		{"category match (substring of name)", "", "蓄電", true},
		{"category mismatch", "", "エアコン", false},
		{"maker and category both match", "ニチコン", "蓄電", true},
		{"maker matches, category doesn't", "ニチコン", "エアコン", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := matchesFilter(p, c.maker, c.category); got != c.want {
				t.Errorf("matchesFilter(%+v, %q, %q) = %v, want %v", p, c.maker, c.category, got, c.want)
			}
		})
	}
}

func TestParseProductHTML(t *testing.T) {
	const html = `
<div class="col-sm-4"><a href="https://echonet.jp/introduce/gz-001078/">
<h3 class="name">蓄電システム					<small>
					ニチコン株式会社					</small>
</h3>
<p><small>ENL認証登録番号 : GZ-001078</small></p>
<p><small>Appendixバージョン : Release Q.1</small></p>
<p>ENL認証登録日・更新日 : 2026/08/18</p>
</a></div>
<div class="col-sm-4"><a href="https://echonet.jp/introduce/gz-000994/">
<h3 class="name">ルームエアコン					<small>
					パナソニック株式会社					</small>
</h3>
<p><small>ENL認証登録番号 : GZ-000994</small></p>
</a></div>
`
	products, err := parseProductHTML(html, 100)
	if err != nil {
		t.Fatalf("parseProductHTML() error = %v", err)
	}
	if len(products) != 2 {
		t.Fatalf("len(products) = %d, want 2", len(products))
	}
	if products[0].Name != "蓄電システム" || products[0].Maker != "ニチコン株式会社" || products[0].CertNumber != "GZ-001078" {
		t.Errorf("products[0] = %+v", products[0])
	}
	if products[1].Name != "ルームエアコン" || products[1].Maker != "パナソニック株式会社" {
		t.Errorf("products[1] = %+v", products[1])
	}
}

func TestFetchProductPageURL(t *testing.T) {
	// fetchProductPage must issue GET requests using echonet.jp's actual
	// search form field names (con_manufacturer / con_product_type / pro_num)
	// and its /page/N/ pagination path, not the form's own POST semantics
	// nor made-up field names like "maker"/"category"/"keyword"/"per_page".
	cases := []struct {
		name     string
		page     int
		maker    string
		category string
		keyword  string
		want     string
	}{
		{"page 1, no filters", 1, "", "", "", productSearchURL},
		{"page 1, keyword only", 1, "", "", "蓄電", productSearchURL + "?pro_num=%E8%93%84%E9%9B%BB"},
		{"page 2, maker + category", 2, "パナソニック", "エアコン", "", productSearchURL + "page/2/?con_manufacturer=%E3%83%91%E3%83%8A%E3%82%BD%E3%83%8B%E3%83%83%E3%82%AF&con_product_type=%E3%82%A8%E3%82%A2%E3%82%B3%E3%83%B3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildProductPageURL(c.page, c.maker, c.category, c.keyword)
			if got != c.want {
				t.Errorf("buildProductPageURL(%d, %q, %q, %q) = %q, want %q", c.page, c.maker, c.category, c.keyword, got, c.want)
			}
		})
	}
}
