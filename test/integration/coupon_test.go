package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"golang-gin/internal/domain"
	"golang-gin/internal/testutil"
)

func TestCreateCoupon_Minimal_Returns201AndPersists(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)

	res := app.POST("/coupon", `{
		"name": "신규 가입 쿠폰",
		"description": "가입하면 받는 쿠폰",
		"type": 1,
		"discountType": 1,
		"discountValue": 3000
	}`)

	require.Equal(t, http.StatusCreated, res.Code)

	var body domain.CouponResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.NotZero(t, body.ID)
	require.EqualValues(t, 1, body.Status, "status defaults to active")
	require.Nil(t, body.Code)
	require.Nil(t, body.ThumbnailURL)
	require.Nil(t, body.IssueLimit, "an omitted limit is NULL, not zero")
	require.False(t, body.CreatedAt.IsZero())

	// Check the row, not just the response.
	var (
		name       string
		issueLimit sql.NullInt32
		code       sql.NullString
	)
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT name, issue_limit, code FROM coupon WHERE id = ?", body.ID).
		Scan(&name, &issueLimit, &code))
	require.Equal(t, "신규 가입 쿠폰", name)
	require.False(t, issueLimit.Valid)
	require.False(t, code.Valid)
}

func TestCreateCoupon_AllFields_RoundTrips(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)

	res := app.POST("/coupon", `{
		"name": "가을 세일",
		"description": "10% 할인",
		"thumbnailUrl": "https://cdn.example.com/coupon/autumn.png",
		"type": 2,
		"discountType": 2,
		"discountValue": 10,
		"issueLimitPerPerson": 1,
		"issueLimit": 1000,
		"code": "AUTUMN10",
		"minPrice": 20000,
		"validPeriod": 43200,
		"status": 0,
		"startedAt": "2026-10-01T00:00:00Z",
		"endedAt": "2026-10-31T23:59:59Z"
	}`)

	require.Equal(t, http.StatusCreated, res.Code)

	var body domain.CouponResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.EqualValues(t, 2, body.Type)
	require.EqualValues(t, 2, body.DiscountType)
	require.EqualValues(t, 10, body.DiscountValue)
	require.EqualValues(t, 0, body.Status)
	require.NotNil(t, body.Code)
	require.Equal(t, "AUTUMN10", *body.Code)
	require.NotNil(t, body.IssueLimit)
	require.EqualValues(t, 1000, *body.IssueLimit)
	require.NotNil(t, body.ThumbnailURL)
	require.Equal(t, "https://cdn.example.com/coupon/autumn.png", *body.ThumbnailURL)
	require.NotNil(t, body.StartedAt)
	require.NotNil(t, body.EndedAt)
}

// The same code may be reused as long as the coupons differ, so two creates
// with one code must both succeed.
func TestCreateCoupon_DuplicateCode_IsAllowed(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)

	body := `{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000,"code":"REPEAT"}`
	first := app.POST("/coupon", body)
	second := app.POST("/coupon", body)

	require.Equal(t, http.StatusCreated, first.Code)
	require.Equal(t, http.StatusCreated, second.Code)

	var n int
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM coupon WHERE code = ?", "REPEAT").Scan(&n))
	require.Equal(t, 2, n)
}

func TestCreateCoupon_InvalidBody_Returns400(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	valid := `"description":"d","type":1,"discountType":1,"discountValue":1000`

	cases := []struct {
		name string
		body string
	}{
		{"no name", `{` + valid + `}`},
		{"name too long", `{"name":"` + strings.Repeat("a", 513) + `",` + valid + `}`},
		{"no description", `{"name":"a","type":1,"discountType":1,"discountValue":1000}`},
		{"unknown type", `{"name":"a","description":"d","type":3,"discountType":1,"discountValue":1000}`},
		{"unknown discount type", `{"name":"a","description":"d","type":1,"discountType":9,"discountValue":1000}`},
		{"zero discount value", `{"name":"a","description":"d","type":1,"discountType":1,"discountValue":0}`},
		{"zero issue limit", `{"name":"a",` + valid + `,"issueLimit":0}`},
		{"code too long", `{"name":"a",` + valid + `,"code":"` + strings.Repeat("A", 257) + `"}`},
		{"non-ascii code", `{"name":"a",` + valid + `,"code":"쿠폰코드"}`},
		{"unknown status", `{"name":"a",` + valid + `,"status":7}`},
		{"thumbnail not a url", `{"name":"a",` + valid + `,"thumbnailUrl":"not a url"}`},
		{"thumbnail too long", `{"name":"a",` + valid + `,"thumbnailUrl":"https://e.com/` + strings.Repeat("a", 2048) + `"}`},
		{"not json at all", `nope`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := app.POST("/coupon", tc.body)
			require.Equal(t, http.StatusBadRequest, res.Code)
		})
	}
}

// chk_coupon_discount_value: a percentage above 100 is refused. The service
// catches it first, but the constraint is what actually guarantees it.
func TestCreateCoupon_RateAbove100_Returns400(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	res := app.POST("/coupon",
		`{"name":"a","description":"d","type":1,"discountType":2,"discountValue":110}`)

	require.Equal(t, http.StatusBadRequest, res.Code)
	require.Contains(t, res.Body.String(), "1 and 100")
}

// An amount coupon has no such ceiling: 110 means 110 won, not 110 percent.
func TestCreateCoupon_AmountAbove100_IsAllowed(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	res := app.POST("/coupon",
		`{"name":"a","description":"d","type":1,"discountType":1,"discountValue":110000}`)

	require.Equal(t, http.StatusCreated, res.Code)
}

// chk_coupon_period: the end must not precede the start.
func TestCreateCoupon_EndBeforeStart_Returns400(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	res := app.POST("/coupon", `{
		"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000,
		"startedAt":"2026-10-31T00:00:00Z","endedAt":"2026-10-01T00:00:00Z"
	}`)

	require.Equal(t, http.StatusBadRequest, res.Code)
	require.Contains(t, res.Body.String(), "started_at")
}
