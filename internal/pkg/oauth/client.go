// Package oauth verifies a social access token with the provider and returns
// the profile behind it.
//
// There is no provider interface here on purpose: the base URLs are injected
// instead, so tests point the client at an httptest.Server rather than at a
// mock.
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang-gin/internal/domain"
)

const (
	DefaultKakaoBaseURL = "https://kapi.kakao.com"
	DefaultNaverBaseURL = "https://openapi.naver.com"
)

// Client talks to the social providers.
type Client struct {
	http         *http.Client
	kakaoBaseURL string
	naverBaseURL string
}

type Option func(*Client)

func WithKakaoBaseURL(u string) Option {
	return func(c *Client) { c.kakaoBaseURL = strings.TrimSuffix(u, "/") }
}

func WithNaverBaseURL(u string) Option {
	return func(c *Client) { c.naverBaseURL = strings.TrimSuffix(u, "/") }
}

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		// A signup request waits on this call, so it must not hang.
		http:         &http.Client{Timeout: 5 * time.Second},
		kakaoBaseURL: DefaultKakaoBaseURL,
		naverBaseURL: DefaultNaverBaseURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Profile exchanges an access token for the profile the provider holds.
//
// A token that the provider rejects, or a response without an id, is
// domain.ErrSocialVerification: from the caller's side both mean "this token
// does not prove anything".
func (c *Client) Profile(ctx context.Context, media domain.MediaName, accessToken string) (*domain.SocialProfile, error) {
	switch media {
	case domain.MediaKakao:
		return c.kakaoProfile(ctx, accessToken)
	case domain.MediaNaver:
		return c.naverProfile(ctx, accessToken)
	case domain.MediaEmail:
		return nil, fmt.Errorf("%w: %s", domain.ErrUnsupportedMedia, media)
	default:
		return nil, fmt.Errorf("%w: %s", domain.ErrUnsupportedMedia, media)
	}
}

func (c *Client) get(ctx context.Context, url, accessToken string) (body []byte, status int, err error) {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if reqErr != nil {
		return nil, 0, fmt.Errorf("build request: %w", reqErr)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, doErr := c.http.Do(req)
	if doErr != nil {
		return nil, 0, fmt.Errorf("call provider: %w", doErr)
	}
	defer func() { _ = res.Body.Close() }()

	// Bounded read: a provider that misbehaves must not exhaust memory.
	body, readErr := readAtMost(res.Body, 1<<20)
	if readErr != nil {
		return nil, res.StatusCode, fmt.Errorf("read provider response: %w", readErr)
	}
	return body, res.StatusCode, nil
}

// https://developers.kakao.com/docs/latest/ko/user-mgmt/rest-api
func (c *Client) kakaoProfile(ctx context.Context, accessToken string) (*domain.SocialProfile, error) {
	body, status, err := c.get(ctx, c.kakaoBaseURL+"/v2/user/me", accessToken)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%w: kakao returned %d", domain.ErrSocialVerification, status)
	}

	var res struct {
		ID         int64 `json:"id"`
		Properties struct {
			Nickname string `json:"nickname"`
		} `json:"properties"`
		KakaoAccount struct {
			HasEmail bool   `json:"has_email"`
			Email    string `json:"email"`
		} `json:"kakao_account"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("%w: kakao response is not valid json", domain.ErrSocialVerification)
	}
	if res.ID == 0 {
		return nil, fmt.Errorf("%w: kakao response has no id", domain.ErrSocialVerification)
	}

	profile := &domain.SocialProfile{
		ID:   strconv.FormatInt(res.ID, 10),
		Name: res.Properties.Nickname,
	}
	if res.KakaoAccount.HasEmail {
		profile.Email = res.KakaoAccount.Email
	}
	return profile, nil
}

// https://developers.naver.com/docs/login/profile/
func (c *Client) naverProfile(ctx context.Context, accessToken string) (*domain.SocialProfile, error) {
	body, status, err := c.get(ctx, c.naverBaseURL+"/v1/nid/me", accessToken)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%w: naver returned %d", domain.ErrSocialVerification, status)
	}

	var res struct {
		ResultCode string `json:"resultcode"`
		Message    string `json:"message"`
		Response   struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("%w: naver response is not valid json", domain.ErrSocialVerification)
	}
	// Naver answers 200 with a result code even when the token is bad:
	// 024 authentication failed, 028 missing header.
	if res.ResultCode != "00" {
		return nil, fmt.Errorf("%w: naver resultcode %s", domain.ErrSocialVerification, res.ResultCode)
	}
	if res.Response.ID == "" {
		return nil, fmt.Errorf("%w: naver response has no id", domain.ErrSocialVerification)
	}

	return &domain.SocialProfile{
		ID:    res.Response.ID,
		Email: res.Response.Email,
		Name:  res.Response.Name,
	}, nil
}
