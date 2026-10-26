package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"golang-gin/internal/domain"
	"golang-gin/internal/testutil"
)

// signedIn registers a customer and returns an app that carries their session.
func signedIn(t *testing.T, app *testutil.App, email string) (*testutil.App, uint64) {
	t.Helper()

	signup := app.POST("/auth/signup", fmt.Sprintf(
		`{"email":%q,"password":"correct horse","name":"remi","mediaName":"email"}`, email))
	require.Equal(t, http.StatusCreated, signup.Code, signup.Body.String())

	var customer domain.CustomerResponse
	require.NoError(t, json.Unmarshal(signup.Body.Bytes(), &customer))

	login := app.POST("/auth/login", fmt.Sprintf(
		`{"email":%q,"password":"correct horse","mediaName":"email"}`, email))
	require.Equal(t, http.StatusOK, login.Code)

	var session domain.LoginResponse
	require.NoError(t, json.Unmarshal(login.Body.Bytes(), &session))

	return app.As(session.Token), customer.ID
}

func TestDownloadCoupon_Success_CopiesDiscountAndSetsExpiry(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)
	me, customerID := signedIn(t, app, "dl@b.com")

	coupon := createCoupon(t, app, `{
		"name":"a","description":"d","type":1,
		"discountType":2,"discountValue":15,"validPeriod":60
	}`)

	res := me.POST(fmt.Sprintf("/coupon/%d/download", coupon.ID), "")
	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())

	var issued domain.CustomerCouponResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &issued))
	require.Equal(t, customerID, issued.CustomerID, "the owner comes from the session")
	require.Equal(t, coupon.ID, issued.CouponID)
	require.EqualValues(t, 2, issued.DiscountType)
	require.EqualValues(t, 15, issued.DiscountValue)
	require.Nil(t, issued.UsedAt)

	// expired_at is now + valid_period minutes, computed by the database.
	require.NotNil(t, issued.ExpiredAt)
	require.WithinDuration(t, time.Now().Add(60*time.Minute), *issued.ExpiredAt, 2*time.Minute)
}

// The point of copying the discount: editing the coupon afterwards must not
// change what an already issued one is worth.
func TestDownloadCoupon_LaterCouponEditDoesNotChangeIssued(t *testing.T) {
	tx := testutil.Tx(t)
	app := testutil.NewApp(t, tx)
	me, _ := signedIn(t, app, "snapshot@b.com")

	coupon := createCoupon(t, app,
		`{"name":"a","description":"d","type":1,"discountType":2,"discountValue":30}`)

	res := me.POST(fmt.Sprintf("/coupon/%d/download", coupon.ID), "")
	require.Equal(t, http.StatusCreated, res.Code)

	var issued domain.CustomerCouponResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &issued))

	update := app.PUT(fmt.Sprintf("/coupon/%d", coupon.ID),
		`{"name":"a","description":"d","type":1,"discountType":2,"discountValue":5}`)
	require.Equal(t, http.StatusOK, update.Code)

	var stored uint32
	require.NoError(t, tx.QueryRowContext(context.Background(),
		"SELECT discount_value FROM customer_coupon WHERE id = ?", issued.ID).Scan(&stored))
	require.EqualValues(t, 30, stored, "the issued copy keeps the terms it was issued with")
}

func TestDownloadCoupon_NoValidPeriod_LeavesExpiryNull(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))
	me, _ := signedIn(t, app, "noexp@b.com")

	coupon := createCoupon(t, app,
		`{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000}`)

	res := me.POST(fmt.Sprintf("/coupon/%d/download", coupon.ID), "")
	require.Equal(t, http.StatusCreated, res.Code)

	var issued domain.CustomerCouponResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &issued))
	require.Nil(t, issued.ExpiredAt)
}

func TestDownloadCoupon_PerPersonLimit_Returns409(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))
	me, _ := signedIn(t, app, "perperson@b.com")

	coupon := createCoupon(t, app, `{
		"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000,
		"issueLimitPerPerson":1
	}`)
	path := fmt.Sprintf("/coupon/%d/download", coupon.ID)

	require.Equal(t, http.StatusCreated, me.POST(path, "").Code)

	second := me.POST(path, "")
	require.Equal(t, http.StatusConflict, second.Code)
	require.Contains(t, second.Body.String(), "per-person")
}

func TestDownloadCoupon_TotalLimit_Returns409(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))
	first, _ := signedIn(t, app, "total1@b.com")
	second, _ := signedIn(t, app, "total2@b.com")

	coupon := createCoupon(t, app, `{
		"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000,
		"issueLimit":1
	}`)
	path := fmt.Sprintf("/coupon/%d/download", coupon.ID)

	require.Equal(t, http.StatusCreated, first.POST(path, "").Code)

	refused := second.POST(path, "")
	require.Equal(t, http.StatusConflict, refused.Code)
	require.Contains(t, refused.Body.String(), "issue limit")
}

func TestDownloadCoupon_Inactive_Returns409(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))
	me, _ := signedIn(t, app, "inactive@b.com")

	coupon := createCoupon(t, app,
		`{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000,"status":0}`)

	res := me.POST(fmt.Sprintf("/coupon/%d/download", coupon.ID), "")
	require.Equal(t, http.StatusConflict, res.Code)
	require.Contains(t, res.Body.String(), "not active")
}

func TestDownloadCoupon_OutsidePeriod_Returns409(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))
	me, _ := signedIn(t, app, "period@b.com")

	ended := createCoupon(t, app, `{
		"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000,
		"startedAt":"2020-01-01T00:00:00Z","endedAt":"2020-12-31T00:00:00Z"
	}`)
	notYet := createCoupon(t, app, `{
		"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000,
		"startedAt":"2099-01-01T00:00:00Z"
	}`)

	for _, id := range []uint64{ended.ID, notYet.ID} {
		res := me.POST(fmt.Sprintf("/coupon/%d/download", id), "")
		require.Equal(t, http.StatusConflict, res.Code)
		require.Contains(t, res.Body.String(), "period")
	}
}

func TestDownloadCoupon_UnknownCoupon_Returns404(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))
	me, _ := signedIn(t, app, "unknown@b.com")

	res := me.POST("/coupon/999999/download", "")
	require.Equal(t, http.StatusNotFound, res.Code)
}

// The session is what identifies the downloader, so no session means no
// download — there is no field to supply instead.
func TestDownloadCoupon_WithoutSession_Returns401(t *testing.T) {
	app := testutil.NewApp(t, testutil.Tx(t))

	coupon := createCoupon(t, app,
		`{"name":"a","description":"d","type":1,"discountType":1,"discountValue":1000}`)
	path := fmt.Sprintf("/coupon/%d/download", coupon.ID)

	require.Equal(t, http.StatusUnauthorized, app.POST(path, "").Code)
	require.Equal(t, http.StatusUnauthorized, app.As("garbage").POST(path, "").Code)
}
