package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"golang-gin/internal/domain"
	"golang-gin/internal/testutil"
)

func TestUpdateCouponStatus_Deactivate_ThenActivate(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)

	created := createCoupon(t, app,
		`{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000}`)
	require.EqualValues(t, 1, created.Status, "a new coupon is active")

	path := fmt.Sprintf("/coupon/%d/status", created.ID)

	off := app.PATCH(path, `{"status":0}`)
	require.Equal(t, http.StatusOK, off.Code)

	var body domain.CouponResponse
	require.NoError(t, json.Unmarshal(off.Body.Bytes(), &body))
	require.EqualValues(t, 0, body.Status)

	var stored uint8
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT status FROM coupon WHERE id = ?", created.ID).Scan(&stored))
	require.EqualValues(t, 0, stored)

	on := app.PATCH(path, `{"status":1}`)
	require.Equal(t, http.StatusOK, on.Code)
	require.NoError(t, json.Unmarshal(on.Body.Bytes(), &body))
	require.EqualValues(t, 1, body.Status)
}

// This is the reason the endpoint exists: unlike PUT, it cannot lose a field
// that the body does not mention.
func TestUpdateCouponStatus_LeavesEveryOtherFieldAlone(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	created := createCoupon(t, app, `{
		"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000,
		"thumbnailUrl":"https://cdn.example.com/a.png",
		"issueLimit":10,"issueLimitPerPerson":1,"minPrice":5000,"validPeriod":60,
		"code":"KEEP","startedAt":"2026-10-01T00:00:00Z"
	}`)

	res := app.PATCH(fmt.Sprintf("/coupon/%d/status", created.ID), `{"status":0}`)
	require.Equal(t, http.StatusOK, res.Code)

	var body domain.CouponResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))

	require.EqualValues(t, 0, body.Status)
	require.Equal(t, created.Name, body.Name)
	require.Equal(t, *created.Code, *body.Code)
	require.Equal(t, *created.ThumbnailURL, *body.ThumbnailURL)
	require.Equal(t, *created.IssueLimit, *body.IssueLimit)
	require.Equal(t, *created.IssueLimitPerPerson, *body.IssueLimitPerPerson)
	require.Equal(t, *created.MinPrice, *body.MinPrice)
	require.Equal(t, *created.ValidPeriod, *body.ValidPeriod)
	require.Equal(t, created.StartedAt.UTC(), body.StartedAt.UTC())
	require.Equal(t, created.CreatedAt, body.CreatedAt)
}

func TestUpdateCouponStatus_InvalidBody_Returns400(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	created := createCoupon(t, app,
		`{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000}`)
	path := fmt.Sprintf("/coupon/%d/status", created.ID)

	cases := []struct {
		name string
		body string
	}{
		{"missing status", `{}`},
		{"unknown status", `{"status":7}`},
		{"status as string", `{"status":"active"}`},
		{"not json at all", `nope`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := app.PATCH(path, tc.body)
			require.Equal(t, http.StatusBadRequest, res.Code)
		})
	}
}

func TestUpdateCouponStatus_UnknownID_Returns404(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	res := app.PATCH("/coupon/999999/status", `{"status":0}`)
	require.Equal(t, http.StatusNotFound, res.Code)
}

func TestUpdateCouponStatus_InvalidID_Returns400(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	for _, id := range []string{"abc", "0"} {
		t.Run(id, func(t *testing.T) {
			res := app.PATCH("/coupon/"+id+"/status", `{"status":0}`)
			require.Equal(t, http.StatusBadRequest, res.Code)
		})
	}
}
