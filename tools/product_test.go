package tools

import (
	"strings"
	"testing"
)

// categorySelectHTML mirrors the con_product_type <select> of echonet.jp's
// product search form (excerpt).
const categorySelectHTML = `
<form method="get" id="searchform" action="https://echonet.jp/product/echonet-lite">
<select name="con_product_type">
	<option value="" selected style="display:none;">▼ 機種を選択してください</option>
	<option value="">すべて</option>
	<optgroup label="空調機器">
		<option value="air_prdct_sprtr_air_other" >その他</option>
		<option value="air_prdct_sprtr_%e5%ae%b6%e5%ba%ad%e7%94%a8%e3%82%a8%e3%82%a2%e3%82%b3%e3%83%b3" >家庭用エアコン</option>
		<option value="air_prdct_sprtr_%e6%a5%ad%e5%8b%99%e7%94%a8%e3%82%a8%e3%82%a2%e3%82%b3%e3%83%b3" >業務用エアコン</option>
	</optgroup>
	<optgroup label="住宅設備機器">
		<option value="house_prdct_sprtr_%e3%81%9d%e3%81%ae%e4%bb%96" >その他</option>
		<option value="house_prdct_sprtr_%e8%93%84%e9%9b%bb%e6%b1%a0" >蓄電池</option>
	</optgroup>
	<optgroup label="計測装置">
		<option value="measuring_prdct_sprtr_%e4%bd%8e%e5%9c%a7%e3%82%b9%e3%83%9e%e3%83%bc%e3%83%88%e9%9b%bb%e5%8a%9b%e9%87%8f%e3%83%a1%e3%83%bc%e3%82%bf" >低圧スマート電力量メータ</option>
	</optgroup>
</select>
<select name="other"><option value="x">関係ない</option></select>
</form>`

const (
	homeAirconValue = "air_prdct_sprtr_%e5%ae%b6%e5%ba%ad%e7%94%a8%e3%82%a8%e3%82%a2%e3%82%b3%e3%83%b3"
	houseOtherValue = "house_prdct_sprtr_%e3%81%9d%e3%81%ae%e4%bb%96"
)

func TestParseCategoryOptions(t *testing.T) {
	opts, err := parseCategoryOptions(categorySelectHTML)
	if err != nil {
		t.Fatalf("parseCategoryOptions() error = %v", err)
	}
	if len(opts) != 6 {
		t.Fatalf("len(opts) = %d, want 6 (empty values and other selects excluded): %+v", len(opts), opts)
	}
	want := categoryOption{Group: "空調機器", Label: "家庭用エアコン", Value: homeAirconValue}
	if opts[1] != want {
		t.Errorf("opts[1] = %+v, want %+v", opts[1], want)
	}
	if opts[3].qualifiedName() != "住宅設備機器/その他" {
		t.Errorf("opts[3].qualifiedName() = %q, want %q", opts[3].qualifiedName(), "住宅設備機器/その他")
	}
}

func TestMatchCategory(t *testing.T) {
	opts, err := parseCategoryOptions(categorySelectHTML)
	if err != nil {
		t.Fatalf("parseCategoryOptions() error = %v", err)
	}

	cases := []struct {
		name      string
		query     string
		want      string // expected Value; "" means an error is expected
		errSubstr string
	}{
		// The schema examples must resolve to an actual option.
		{"schema example: 家庭用エアコン", "家庭用エアコン", homeAirconValue, ""},
		{"schema example: 低圧スマート電力量メータ", "低圧スマート電力量メータ", "measuring_prdct_sprtr_%e4%bd%8e%e5%9c%a7%e3%82%b9%e3%83%9e%e3%83%bc%e3%83%88%e9%9b%bb%e5%8a%9b%e9%87%8f%e3%83%a1%e3%83%bc%e3%82%bf", ""},
		{"exact name with surrounding spaces", " 蓄電池 ", "house_prdct_sprtr_%e8%93%84%e9%9b%bb%e6%b1%a0", ""},
		{"option value as-is", homeAirconValue, homeAirconValue, ""},
		{"group-qualified name disambiguates", "住宅設備機器/その他", houseOtherValue, ""},
		{"unique partial match", "蓄電", "house_prdct_sprtr_%e8%93%84%e9%9b%bb%e6%b1%a0", ""},
		{"ambiguous exact name", "その他", "", "空調機器/その他, 住宅設備機器/その他"},
		{"ambiguous partial match", "エアコン", "", "空調機器/家庭用エアコン, 空調機器/業務用エアコン"},
		{"unknown category", "ルームエアコン", "", "指定できるカテゴリ"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := matchCategory(opts, c.query)
			if c.want == "" {
				if err == nil {
					t.Fatalf("matchCategory(%q) = %+v, want error", c.query, got)
				}
				if !strings.Contains(err.Error(), c.errSubstr) {
					t.Errorf("matchCategory(%q) error = %q, want it to contain %q", c.query, err, c.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("matchCategory(%q) error = %v", c.query, err)
			}
			if got.Value != c.want {
				t.Errorf("matchCategory(%q).Value = %q, want %q", c.query, got.Value, c.want)
			}
		})
	}
}

func TestMaxScanPagesCoversMaxLimit(t *testing.T) {
	// Every allowed limit must be reachable within maxScanPages pages.
	if maxScanPages*productsPerPage < maxProductLimit {
		t.Errorf("maxScanPages(%d) * productsPerPage(%d) < maxProductLimit(%d)", maxScanPages, productsPerPage, maxProductLimit)
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
	// When any filter is set, the whole form (empty fields included) must be
	// sent, because echonet.jp ignores con_manufacturer / con_product_type
	// otherwise.
	const emptyDateFields = "&con_num=&con_year_from=&con_month_from=&con_day_from=&con_year_to=&con_month_to=&con_day_to="
	cases := []struct {
		name     string
		page     int
		maker    string
		category string
		keyword  string
		want     string
	}{
		{"page 1, no filters", 1, "", "", "", productSearchURL},
		{"page 3, no filters", 3, "", "", "", productSearchURL + "page/3/"},
		{"page 1, keyword only", 1, "", "", "蓄電", productSearchURL + "?con_manufacturer=&con_product_type=" + emptyDateFields + "&pro_num=%E8%93%84%E9%9B%BB"},
		{"page 1, maker only", 1, "パナソニック", "", "", productSearchURL + "?con_manufacturer=%E3%83%91%E3%83%8A%E3%82%BD%E3%83%8B%E3%83%83%E3%82%AF&con_product_type=" + emptyDateFields + "&pro_num="},
		// The category option value contains literal %xx sequences, which
		// must be escaped as %25xx just like a browser submitting the form.
		{"page 2, maker + category", 2, "パナソニック", homeAirconValue, "", productSearchURL + "page/2/?con_manufacturer=%E3%83%91%E3%83%8A%E3%82%BD%E3%83%8B%E3%83%83%E3%82%AF&con_product_type=air_prdct_sprtr_%25e5%25ae%25b6%25e5%25ba%25ad%25e7%2594%25a8%25e3%2582%25a8%25e3%2582%25a2%25e3%2582%25b3%25e3%2583%25b3" + emptyDateFields + "&pro_num="},
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
