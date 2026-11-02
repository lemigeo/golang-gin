package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"golang-gin/internal/domain"
	"golang-gin/internal/testutil"
)

// createCoupon posts a coupon and returns the created row.
func createCoupon(t *testing.T, app *testutil.App, body string) domain.CouponResponse {
	t.Helper()

	res := app.POST("/coupon", body)
	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())

	var created domain.CouponResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &created))
	return created
}

func TestUpdateCoupon_ReplacesEveryWritableField(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)

	created := createCoupon(t, app, `{
		"name": "이전 이름",
		"description": "이전 설명",
		"thumbnailUrl": "https://cdn.example.com/old.png",
		"type": 1,
		"discountType": 1,
		"discountValue": 1000,
		"issueLimit": 10,
		"code": "OLD"
	}`)

	res := app.PUT(fmt.Sprintf("/coupon/%d", created.ID), `{
		"name": "새 이름",
		"description": "새 설명",
		"thumbnailUrl": "https://cdn.example.com/new.png",
		"type": 2,
		"discountType": 2,
		"discountValue": 25,
		"issueLimit": 50,
		"code": "NEW",
		"status": 0
	}`)

	require.Equal(t, http.StatusOK, res.Code)

	var body domain.CouponResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Equal(t, created.ID, body.ID, "the id must not change")
	require.Equal(t, "새 이름", body.Name)
	require.EqualValues(t, 2, body.Type)
	require.EqualValues(t, 25, body.DiscountValue)
	require.EqualValues(t, 0, body.Status)
	require.Equal(t, "NEW", *body.Code)
	require.Equal(t, created.CreatedAt, body.CreatedAt, "created_at must survive a replacement")

	var name, code string
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT name, code FROM coupon WHERE id = ?", created.ID).Scan(&name, &code))
	require.Equal(t, "새 이름", name)
	require.Equal(t, "NEW", code)
}

// PUT replaces rather than merges: a field the body omits becomes NULL.
func TestUpdateCoupon_OmittedFieldsBecomeNull(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)

	created := createCoupon(t, app, `{
		"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000,
		"thumbnailUrl":"https://cdn.example.com/old.png",
		"issueLimit":10,"issueLimitPerPerson":1,"minPrice":5000,"validPeriod":60,
		"code":"HAD_ONE","startedAt":"2026-10-01T00:00:00Z"
	}`)
	require.NotNil(t, created.Code)

	res := app.PUT(fmt.Sprintf("/coupon/%d", created.ID),
		`{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000}`)
	require.Equal(t, http.StatusOK, res.Code)

	var body domain.CouponResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Nil(t, body.Code)
	require.Nil(t, body.ThumbnailURL)
	require.Nil(t, body.IssueLimit)
	require.Nil(t, body.IssueLimitPerPerson)
	require.Nil(t, body.MinPrice)
	require.Nil(t, body.ValidPeriod)
	require.Nil(t, body.StartedAt)

	var code sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT code FROM coupon WHERE id = ?", created.ID).Scan(&code))
	require.False(t, code.Valid, "the column must actually be NULL, not an empty string")
}

func TestUpdateCoupon_UnknownID_Returns404(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	res := app.PUT("/coupon/999999",
		`{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000}`)

	require.Equal(t, http.StatusNotFound, res.Code)
}

func TestUpdateCoupon_InvalidID_Returns400(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	body := `{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000}`
	for _, id := range []string{"abc", "0", "-1"} {
		t.Run(id, func(t *testing.T) {
			res := app.PUT("/coupon/"+id, body)
			require.Equal(t, http.StatusBadRequest, res.Code)
		})
	}
}

// The same rules that guard creation guard replacement — they live on the
// shared CouponInput, so they cannot drift apart.
func TestUpdateCoupon_ViolatesColumnRules_Returns400(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	created := createCoupon(t, app,
		`{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000}`)
	path := fmt.Sprintf("/coupon/%d", created.ID)

	cases := []struct {
		name string
		body string
	}{
		{"rate above 100", `{"name":"a","description":"d","type":1,"discountType":2,"discountValue":110}`},
		{"end before start", `{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1,
			"startedAt":"2026-10-31T00:00:00Z","endedAt":"2026-10-01T00:00:00Z"}`},
		{"no name", `{"description":"d","type":1,"discountType":1,"discountValue":1000}`},
		{"unknown type", `{"name":"a","description":"d","type":9,"discountType":1,"discountValue":1000}`},
		{"thumbnail not a url", `{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1,"thumbnailUrl":"nope"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := app.PUT(path, tc.body)
			require.Equal(t, http.StatusBadRequest, res.Code)
		})
	}
}
